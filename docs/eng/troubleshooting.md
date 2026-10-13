# Troubleshooting

[Русский](../rus/troubleshooting.md) · [Documentation](README.md)

Start by recording the release tag or source revision, client version (and Go/Java versions only for developer tools), working directory and exact startup command. Include the first relevant log error and reproduction steps; redact passwords, tokens, public IPs and player data.

| Symptom | Check / action |
| --- | --- |
| `initialize server files failed` | Check writable parent directories and filesystem support for hard links used to publish complete defaults without overwriting existing files. |
| `load config failed` | A missing file is created at `-config`; check directory permissions, unknown fields and invalid values. Compare with [configuration](configuration.md). |
| `prepare server resources failed` | Check connectivity, HTTPS proxy/trust settings, free space and the reported checksum/archive error. Retry the same command; an invalid existing vanilla pack is preserved. See [installation](getting-started.md). |
| `load server resources failed` | Inspect datapack/compiler errors and the working directory. A valid local vanilla pack is reused; no full code generation is needed. |
| `java` missing / unsupported class version in developer tools | Full generation requires both `java` and `javac` from JDK 21 on PATH. The standalone report generator needs a compatible JVM. Normal startup does not use Java. |
| Source preparation fails during `make generate` | Named sources are prepared automatically; check the pinned tool download/checksum, JDK 21, free space and the first remap/decompiler error. See [generation](generators/README.md). |
| `unsupported DataVersion` | Use a copy of a 1.21.4 world; do not alter the version tag to bypass the check. |
| Backup failure | Check free space, permissions and unsupported file types. Existing `.vitis-backup` must be a directory. |
| Access policy unreadable | Validate the reported JSON file; restore a known-good file. Do not silently discard access policy data. |
| Connection refused | Wait for `Vitis listening`; verify host, port, firewall and listener errors. |
| Login fails | Use client 1.21.4; online mode needs Mojang session authentication. Check bans, duplicate sessions and max players. |
| Whitelist rejects online player | Name-based add uses offline UUIDs; see [operations](operations.md). |
| YAML accepted but behavior unchanged | Some settings are used only at startup; others are unused. See the [configuration reference](configuration.md). |
| Hot reload rejected | A restart-required setting changed, or validation failed. Previous active configuration remains. |
| `reload` fails | Inspect the first resource compiler error; fix pack metadata/JSON/resource types. Previous snapshot remains active. |
| Scoreboard/team/worldborder/bossbar unavailable | These commands are not connected to the game systems. |
| Slow exploration / high RAM | Reduce view distance and streaming budget; inspect chunk workers, pregeneration and disk latency. Compare equal workloads. |
| Shutdown/save errors | Preserve logs and world files; verify backups before restarting or restoring. |

If a packet is accepted but no action occurs, check [feature status](status.md).

For a useful issue, include expected/actual behavior, minimal configuration with secrets removed, reproduction on a disposable world, and whether it also occurs with no custom datapacks. Performance reports should include workload, players, distances, seed, dimensions, CPU/RAM and storage.

Developers can use [tests and performance tools](testing.md). No HTTP metrics or profiling endpoint is currently configured.
