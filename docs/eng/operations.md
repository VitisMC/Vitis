# Server operations

[Русский](../rus/operations.md) · [Documentation](README.md)

## Working directory and files

Start from your writable server directory containing the unpacked binary. `-config` changes only the YAML location; resources, `configs/tab.yaml`, `ops.json`, `whitelist.json`, `banned-players.json` and `banned-ips.json` still use the process working directory. See [installation](getting-started.md).

Missing access-list files are accepted as empty. Unreadable or malformed files prevent startup. Do not share real operator lists, passwords or player data when reporting issues.

## Console and permissions

Use commands without a leading slash in stdin: `help`, `list`, `op PlayerName`, `stop`. The in-game form includes a slash. Console and RCON senders have administrative permissions; player-only commands still require a player context.

For the first operator, connect the player and use `op PlayerName` in the console. Operator entries persist in `ops.json`. See the [command catalogue](commands.md); permissions use numeric levels.

Whitelist entries persist, but its enabled flag is memory-only: reissue `whitelist on` after every restart. **Whitelist add currently derives an offline UUID from the name**, which does not identify an online-mode Mojang account. Ban-by-name uses the actual UUID for connected targets and an offline UUID otherwise; operator grants use the connected player's UUID. Operators bypass whitelist checks, not player/IP bans.

## RCON and Query

RCON is disabled by default and listens on loopback when enabled. Set `rcon.enabled`, address, port and a private strong password in YAML; restart. The validator requires at least 16 bytes, 8 distinct bytes and two character categories, and rejects common password fragments and surrounding whitespace. See [password validation](../../internal/network/rcon/password.go).

RCON carries remote administrative commands over TCP without TLS. Keep the listener on a trusted interface or behind a secure tunnel. Never commit a real secret.

GS4 Query uses UDP. When enabled, it binds all interfaces on `query.port`, independently of `server.host`. Restrict exposure with the host firewall as appropriate. Its MOTD/max-player metadata is captured at startup. RCON/Query startup errors are logged but do not necessarily terminate the Minecraft listener; verify each listener you enable.

## Reloading

- Editing YAML triggers the config watcher. MOTD, player and connection limits apply without restarting. Other settings are described in the [reference](configuration.md).
- `reload` recompiles server datapacks. Failure retains the last successful resource snapshot.
- Tab templates are read at startup; restart to apply their changes.
- No SIGHUP reload handler is installed.

## Shutdown and logging

Use `stop`, SIGINT or SIGTERM and wait for process exit. Shutdown cancels the tab/config tasks, stops auxiliary listeners, closes player sessions, drains player lifecycle work, stops the world tick and flushes world persistence. The shared graceful deadline is five seconds; inspect shutdown errors before trusting a backup.

Logs use console or JSON format. Level/format are initialized at startup. YAML `logging.file_path` and `logging.enable_color` are not wired into logger initialization. Capture stdout/stderr through your process supervisor or shell if you need files.

For supervised processes, preserve the working directory, keep shutdown timeouts long enough for Vitis, and avoid force-killing it during saving. No Prometheus/pprof HTTP endpoint is currently configured.

Sources: [server bootstrap](../../cmd/vitis/main.go), [console](../../internal/session/console.go), [access policies](../../internal/data/whitelist.go), [command bridge](../../internal/session/command_bridge.go).
