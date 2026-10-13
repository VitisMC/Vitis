# Configuration reference

[Русский](../rus/configuration.md) · [Documentation](README.md)

## Loading and defaults

`-config` selects the configuration file. The default is `configs/vitis.yaml` relative to the working directory. Startup creates a missing file and preserves an existing one. Unknown YAML keys are rejected. Omitted settings use [Default](../../internal/config/defaults.go) and then pass [validation](../../internal/config/validate.go).

The table lists **built-in defaults**. Example YAML overrides are listed below. `P = max(2, GOMAXPROCS(0))`, `W = max(2, floor(2 × P / 3))`. Tick durations depend on the world's actual tick rate.

## Runtime changes

The server checks the file every 500 ms and waits 150 ms after a change before reloading it. Invalid values or disallowed changes leave the previous settings active.

| Code | Actual behavior |
| --- | --- |
| L | Applied without restarting. |
| R | Changes during operation are rejected; restart required. |
| S | Read at startup. Reload may accept the file, but the value takes effect after restarting. |
| U | The server does not use this setting. |
| U/R | Not used; changes during operation are also rejected. |

`/reload` updates datapacks. YAML is tracked separately. Tab-list settings use a separate file; see [resources](resources.md).

## All settings

| YAML key | Default | Units / constraints | Change | Purpose |
| --- | --- | --- | --- | --- |
| `server.host` | `0.0.0.0` | Nonempty bind address | R | TCP bind address |
| `server.port` | `25565` | TCP port; 1–65535 | R | Minecraft listener |
| `server.max_players` | `200` | Players; >0 | L | Online capacity; status and live tab |
| `server.online_mode` | `true` | Boolean | R | Mojang authentication; false uses offline UUIDs |
| `server.motd` | `Vitis Server` | Nonblank text | L | Server-list description; Query retains startup value |
| `server.default_game_mode` | `survival` | survival, creative, adventure, spectator | S | Initial player game mode |
| `network.max_connections` | `512` | Connections; >0 | L | Includes status/login; independent of player limit |
| `network.admission_rate_per_second` | `10` | Admissions/s/IP; >0 | R | Per-IP token refill |
| `network.admission_burst` | `20` | Admissions/IP; >0 | R | Per-IP burst tokens |
| `network.read_timeout_seconds` | `30` | Seconds; >0 | R | Read deadline |
| `network.write_timeout_seconds` | `30` | Seconds; >0 | R | Write deadline |
| `network.idle_timeout_seconds` | `120` | Seconds; >0 | R | Idle and pre-play phase timeout |
| `network.compression_threshold` | `256` | Bytes; ≥−1 | S | −1 disables compression; startup login policy |
| `network.max_packet_size` | `2097152` | Bytes; ≥512 | R | Bootstrap frame limit floors at 2097152 |
| `network.max_inbound_queue_capacity` | `4096` | Tasks; ≥1 | R | Bootstrap worker queue floors at 1024 |
| `tick.target_tps` | `20` | Ticks/s; 1–1000 | R | Target tick frequency |
| `tick.max_catch_up_ticks` | `5` | Ticks; 0–100 | R | Maximum catch-up work |
| `world.default_world_name` | `world` | Nonblank name/path | R | World root relative to working directory |
| `world.seed` | `42` | int64 | R | New-world seed; existing level.dat takes precedence |
| `world.difficulty` | `normal` | peaceful, easy, normal, hard | R | New-world difficulty; existing metadata is loaded |
| `world.backup_world` | `true` | Boolean | R | One-time backup before opening existing data |
| `world.view_distance` | `10` | Chunks; 2–96 | R | Server streaming distance cap |
| `world.simulation_distance` | `6` | Chunks; 2–96, ≤view_distance | R | Ticket radius for block/entity simulation; ticking waits for loaded dependencies |
| `world.scheduler_queue_capacity` | `8192` | Tasks; ≥1 | R | World mutation queue capacity |
| `world.scheduler_drain_per_tick` | `1024` | Tasks/tick; ≥1 | R | World mutation drain budget |
| `world.chunk_load_workers` | `W` | Workers; ≥1 | R | Async load/generation concurrency |
| `world.chunk_worker_queue_size` | `W` | Tasks; ≥1 | R | Loader worker queue |
| `world.chunk_request_queue_size` | `8192` | Requests; ≥1 | R | Pending load request capacity |
| `world.chunk_result_queue_size` | `8192` | Results; ≥1 | R | Worker completion queue |
| `world.chunk_request_pump_batch` | `256` | Requests/tick; ≥1 | R | Request dispatch budget |
| `world.chunk_completion_batch` | `256` | Results/tick; ≥1 | R | Completion application budget |
| `world.chunk_unload_batch` | `64` | Chunks/tick; ≥1 | R | Chunk unload budget |
| `world.chunk_unload_scan_batch` | `512` | Entries/tick; ≥1 | R | Unload candidate scan budget |
| `world.chunk_unload_ttl_ticks` | `600` | Ticks; ≥1 | R | Idle TTL; 30s at 20 TPS |
| `world.chunk_storage_initial_capacity` | `2048` | Entries; ≥1 | R | Initial map capacity, not a hard chunk limit |
| `world.chunk_unload_queue_size` | `8192` | Requests; ≥1 | R | Unload queue capacity |
| `world.chunk_stream_chunks_per_tick` | `32` | Chunks/player/tick; ≥1 | R | Send budget adjusted by client batch feedback |
| `world.autosave_interval_ticks` | `1200` | Ticks; ≥1 | R | Dirty scan interval; 60s at 20 TPS |
| `world.autosave_batch_per_tick` | `2` | Chunks/tick; ≥1 | R | Dirty chunk snapshot budget |
| `world.autosave_queue_capacity` | `64` | Snapshots; ≥1 | R | Bounded async save queue |
| `logging.level` | `info` | trace, debug, info, warn, error | S | Logger initialized once |
| `logging.format` | `console` | console, json | S | Logger initialized once |
| `logging.file_path` | `""` | String; no path validation | U | Not connected to logger output |
| `logging.enable_color` | `true` | Boolean | U | Not passed to logger initialization |
| `performance.io_worker_pool_size` | `P` | Workers; ≥1 | R | Inbound packet dispatch pool |
| `performance.read_buffer_size` | `65536` | Bytes; ≥1024 | R | Bootstrap read buffer floors at 4096 |
| `performance.write_buffer_size` | `65536` | Bytes; ≥1024 | U/R | Validated and restart-guarded; not passed to ConnConfig |
| `performance.outbound_queue_size` | `2048` | Packets; ≥1 | R | Bootstrap write queue floors at 1024 |
| `rcon.enabled` | `false` | Boolean | S | Start remote console |
| `rcon.address` | `127.0.0.1` | Nonblank when enabled | S | RCON TCP bind address |
| `rcon.port` | `25575` | TCP port; 1–65535 when enabled | S | RCON listener port |
| `rcon.password` | `""` | Secret; validated when enabled | S | ≥16 bytes, ≥8 distinct bytes, ≥2 character categories; no edge spaces/common fragments |
| `query.enabled` | `false` | Boolean | S | Start GS4 Query |
| `query.port` | `25565` | UDP port; 1–65535 when enabled | S | Binds all interfaces, independently of server.host |
| `security.chat_messages_per_10_seconds` | `5` | Messages/10s/player; >0 | S | Bootstrap chat rate limit |
| `security.commands_per_10_seconds` | `10` | Commands/10s/player; >0 | S | Bootstrap command rate limit |

## Example YAML differences

[configs/vitis.example.yaml](../../configs/vitis.example.yaml) is a production example using online mode, Survival and default view/simulation distances `10/6`. Local `configs/vitis.yaml` is runtime configuration and is excluded from Git.

Offline mode does not authenticate a Mojang account: clients can claim another player's name. Enable online mode for public access. Changing mode changes UUIDs and playerdata/access-list identity; it does not migrate them automatically.

A customized runtime configuration may increase distances and chunk send budgets. Start with built-in defaults and increase them after testing your workload; see [installation](getting-started.md).

## Behavior sources

[Loader](../../internal/config/loader.go), [watcher](../../internal/config/watcher.go), [bootstrap and reload hook](../../cmd/vitis/main.go). Minimum effective network values are listed in the table. See [operations](operations.md) for RCON/Query and logging.
