# World storage, backups and recovery

[Русский](../rus/storage.md) · [Documentation](README.md)

## Layout and compatibility

`world.default_world_name` selects the world root. The normal layout is:

```text
world/
  level.dat
  playerdata/
  region/
  entities/
  poi/
  datapacks/
  vanilla/<version>/
  DIM-1/                 Nether region/entities/poi
  DIM1/                  End region/entities/poi
  dimensions/<namespace>/<path>/   Additional dimensions
```

Chunks use Anvil `.mca`; player records and metadata use NBT. Existing `level.dat` must have DataVersion **4189**. Vitis rejects a different version; it has no automatic DataFixer migration. Existing metadata supplies the world's seed and settings.

Region writes preserve supported data and unknown fields. Test the server on an independent copy before opening a valuable world.

## Saving

World ticks create bounded immutable snapshots; background saving writes region batches and acknowledges successful saves. A chunk changed after its snapshot must remain dirty. Autosave intervals and budgets are in the [configuration reference](configuration.md).

Player snapshots are saved periodically using `world.autosave_interval_ticks`, during lifecycle operations and at shutdown. A bounded writer applies ordered snapshots and retains pending offers when its queue is full. Playerdata preserves the current dimension, portal cooldown, personal spawn dimension, inventory and selected hotbar slot. Watch for player save, region write and shutdown failures. SIGKILL, a full disk or interrupted writes can cause data loss.

## Automatic first-use backup

With `backup_world: true`, bootstrap copies an existing world to sibling `<world>.vitis-backup` before normal world writes. Copying uses a temporary sibling directory and publishes the completed backup; unsupported file types, including symlinks, cause an error.

An existing completed backup is reused and **never refreshed**. It retains the state from the first opening. Metadata validation and the safety copy run before resource installation. A new world containing only its validated managed vanilla pack does not trigger a safety copy.

A safety copy made before the first resource migration retains the original layout. It does not gain resources installed later; keep the preserved `.mcdata` source or create a current full backup after stopping the migrated server.

## Create a current backup

1. Run `stop`; wait for successful exit and review errors.
2. Copy the entire world directory, preserving all dimensions, metadata, datapacks, `vanilla/<version>/`, playerdata and pregeneration checkpoint files.
3. Store the active YAML, access-list JSON and tab templates separately with restricted access.
4. Keep the backup outside the live world directory and verify that it can be restored.

For a stopped server, a Linux example is:

```bash
mkdir -p /tmp/vitis-backups
tar -czf "/tmp/vitis-backups/world-$(date -u +%Y%m%dT%H%M%SZ).tar.gz" world
```

Windows (PowerShell), after stopping the server:

```powershell
$backupRoot = Join-Path $env:TEMP "vitis-backups"
New-Item -ItemType Directory -Path $backupRoot -Force | Out-Null
$stamp = [DateTime]::UtcNow.ToString("yyyyMMddTHHmmssfffffffZ")
Copy-Item -LiteralPath world -Destination (Join-Path $backupRoot "world-$stamp") -Recurse -Force
```

Adjust `world` to the configured root. Choose durable storage for actual backups; `/tmp` and `$env:TEMP` are only example destinations. Copying live region files is not a consistent backup.

## Recover

Stop Vitis and preserve the failed world separately. Restore an entire known-good backup into an empty destination, retaining its original layout and configuration. Confirm DataVersion and resource version, then start and inspect logs before admitting players. Do not merge selected region files into a live world or change DataVersion by hand.

Changing online mode changes UUID identity; existing playerdata and operator/ban/whitelist entries do not migrate automatically.

Sources: [world preparation](../../cmd/vitis/world_bootstrap.go), [backup](../../internal/world/level/backup.go), [autosave](../../internal/world/autosave.go), [Anvil](../../internal/world/region/anvil.go), [player snapshots](../../cmd/vitis/player_snapshot.go).
