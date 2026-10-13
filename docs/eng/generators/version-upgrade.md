# Minecraft version upgrades

[Русский](../../rus/generators/version-upgrade.md) · [Documentation](../README.md)

This guide covers porting the core to another Minecraft version. Vitis currently supports 1.21.4. Generating data for a new version requires corresponding server code changes.

## Prepare an isolated change

Start from a clean development branch or a separate copy containing the intended current source. Keep existing generated artifacts, fixtures and worlds recoverable. Record baseline build/test results and inspect [known issues](../status.md).

Verify the target wire protocol, DataVersion, pack format, Java version, mappings, registries and packet layouts against that version's primary data/source. The [generation pipeline](README.md) prepares the official JAR, named source and remapped JAR automatically. Port and pin the Java/toolchain requirements if the target version needs a different JDK or source format.

## Regenerate and inspect

For the current version:

Linux (Bash):

```bash
make generate MC_VERSION=1.21.4
```

Windows (PowerShell 7.2+):

```powershell
pwsh -NoProfile -File .\scripts\windows\update_version.ps1 -Version 1.21.4
```

After adapting generators, select the new version with `MC_VERSION` on Linux or `-Version` on Windows. The default is 1.21.4. The direct Linux script `./scripts/linux/update_version.sh <version>` accepts the same argument.

Check exit status and rollback behavior. Inspect `source.lock`, checksums, all generated files and reference fixture diffs. Repeated generation with the same verified inputs should be deterministic; investigate unexpected ordering/ID/type changes.

## Integrate version changes

- Update [protocol constants](../../../internal/protocol/state.go), [pack-format constant](../../../internal/datapack/manager.go) and every runtime version-dependent contract. Automatic resource preparation uses these same constants; verify first-run extraction with the new official JAR.
- The packet generator updates the runtime [packetid package](../../../internal/protocol/packetid/) and compatibility constants together. Run its `--check` mode after regeneration.
- Update typed packet layouts, direction/state registrations, configuration/login flows and bounds for added/removed fields.
- Check registry IDs, configuration NBT types, tags and known-pack declarations. Update builtin adapters when new registry names require them.
- Validate block states, components, recipes, loot, entity metadata and generation traits against the target.
- Define world DataVersion compatibility explicitly. Changing a constant is not a world migration; preserve backup/rejection behavior.
- Update reference exporters/fixtures and both language documentation, including the status matrix.

Useful searches from the root:

Linux (Bash):

```bash
rg -n '1\.21\.4|769|4189|CurrentPackFormat|go:generate' cmd internal scripts configs docs README*.md
go build ./...
make verify
```

Windows (PowerShell 7.2+):

```powershell
Get-ChildItem cmd,internal,scripts,configs,docs -Recurse -File | Select-String -Pattern '1\.21\.4|769|4189|CurrentPackFormat|go:generate'
Get-ChildItem README*.md | Select-String -Pattern '1\.21\.4|769|4189|CurrentPackFormat|go:generate'
go build ./...
pwsh -NoProfile -File .\scripts\windows\verify.ps1
```

Review results; do not blindly replace matching numbers.

## Verify the result

Check phase ordering, encryption/compression, registry sync with/without known vanilla packs, malformed packets and packet ID uniqueness in each context. Connect the target client; test world join, chunks, inventory, representative mechanics, persistence and reconnect/shutdown.

Compare generated chunks with reference chunks and reopen a disposable existing world. Record remaining test failures.

Related references: [packet IDs](../packets/packet-ids.md), [registry overview](../registry/overview.md), [test guide](../testing.md).
