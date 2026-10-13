# Features and limitations

[Русский](../rus/status.md) · [Documentation](README.md)

Vitis supports Minecraft Java Edition 1.21.4, protocol 769 and DataVersion 4189. The project is experimental; full Survival support is still in development.

| Feature | Status | Support and limitations | Source |
| --- | --- | --- | --- |
| Automatic first-run setup | Implemented | Creates missing configuration and access files, installs 1.21.4 resources. No Go or Java needed to run the server | [Code](../../internal/data/vanilla/prepare.go) |
| Connection, status queries and login | Implemented | Mojang authentication, encryption, compression and transition to play | [Code](../../internal/session/protocol_flow.go) |
| Packet encoding and decoding | Implemented | Input size and structure checks | [Code](../../internal/protocol/states/) |
| Automatic configuration reload | Partial | MOTD, player and connection limits update without restarting. Other settings are covered in the reference | [Code](../../cmd/vitis/main.go) |
| Overworld, Nether and End | Partial | Gameplay and lifecycle use the session dimension. Login restores the saved world; prepared portal travel and cross-world respawn are connected. Full portal parity and End progression remain incomplete | [Code](../../internal/session/dimension_transfer.go) |
| Chunk loading, streaming, saving and pregeneration | Implemented | Bounded work queues, data snapshots and saved pregeneration progress | [Code](../../internal/world/autosave.go) |
| Simulation distance and chunk activity | Implemented | Separate visibility, simulation, spawn, portal and forced tickets; ticking waits for loaded dependency rings. Pregeneration does not activate simulation | [Code](../../internal/world/activity.go) |
| Scheduled block and fluid ticks | Implemented | Inactive chunks retain work; snapshots preserve remaining delays and priorities in Anvil `block_ticks` and `fluid_ticks` | [Code](../../internal/world/chunk/scheduled_ticks.go) |
| Random ticks and neighbor updates | Partial | Crop growth, support checks, delayed farmland conversion and portal collapse are connected. Queues and border scans are bounded; full chained-update order, other plants and farmland hydration/drying remain incomplete | [Code](../../internal/world/neighbor_updates.go) |
| Runtime lighting | Partial | Updates cross loaded chunk seams and reach existing observers; Nether and End exclude skylight. Full block-shape parity is unverified | [Code](../../internal/world/light_updates.go) |
| Runtime entity persistence | Partial | Implemented items, XP with Value/Count, TNT, falling blocks, basic mobs and projectiles use entity regions. Mob persistence and death-reward context are saved. Unknown types and passenger graphs remain opaque NBT; passenger simulation is unsupported | [Code](../../internal/world/entity_persistence.go) |
| Opening Anvil worlds | Partial | DataVersion 4189 only; no automatic migration between versions | [Code](../../cmd/vitis/world_bootstrap.go) |
| Datapacks and world generation | Partial | Resources are validated and applied atomically. Generation still differs from vanilla | [Code](../../internal/datapack/manager.go) |
| Movement, blocks, inventory and crafting | Partial | Server-side checks, complete stack data, grid recipes and ingredient placement | [Code](../../internal/session/play_handler.go) |
| Survival movement validation | Partial | Checks movement per tick, block shapes, ground state and teleport acknowledgements. Flight, swimming/crawling poses and glider wear are supported; vehicle control and complete movement physics are not | [Code](../../internal/session/movement_modes.go) |
| Survival, combat, death and bed respawn | Partial | Saves player state, spawn points and selected slots. Bed sleep, death and respawn use the player’s dimension; travel and disconnect cancel temporary actions | [Code](../../internal/session/player_lifecycle.go) |
| Mobs, projectiles, fishing, maps and portals | Partial | Handlers are listed in [gameplay systems](gameplay.md); player vehicle control is unavailable | [Code](../../internal/world/world.go) |
| Natural spawning | Partial | Uses biome tables, dimension/light rules, distance limits, global/local caps and biome spawn costs. Supports drowned and slime placement. Phantom spawning and some species are missing | [Code](../../internal/world/spawning/) |
| Mob combat, loot and experience | Partial | Melee, arrows, creeper explosions, equipment pickup, typed damage, loot tables, Looting, XP orbs and Mending. Species behavior, enchantments and loot functions remain incomplete | [Code](../../internal/world/world.go) |
| Dragon, Wither and complete mob behavior | Unsupported | Boss fights, raids, POI/professions and full species behavior remain unfinished | [Code](../../internal/entity/ai/) |
| Enchantments and special interfaces | Partial | Enchant command works. Enchanting-table, anvil, beacon and trading actions are not handled | [Code](../../internal/session/play_policy.go) |
| Stonecutter menu | Partial | Server checks recipe selection, costs and inventory capacity; results retain components. Inputs return on close, death, disconnect and travel. Crash recovery and progression events are missing | [Code](../../internal/session/menu_action.go) |
| Advancements | Partial | Definitions load; full trigger evaluation, rewards and client progress are not connected | [Code](../../internal/advancement/runtime.go) |
| Recipe book | Partial | Typed displays, resource-based persistence, vanilla recipe triggers and ingredient placement | [Code](../../internal/session/play_policy.go) |
| Player vehicle control | Unsupported | VehicleMove and SteerBoat packets are accepted without performing actions | [Code](../../internal/session/play_policy.go) |
| Signed chat | Unsupported | Messages are sent unsigned; signed-chat session updates are not handled | [Code](../../internal/session/play_policy.go) |
| Scoreboard, team, worldborder and bossbar commands | Unsupported | Registered but not connected to the game systems | [Code](../../cmd/vitis/main.go) |
| Operators, bans and whitelist | Partial | Lists persist; the whitelist enabled flag does not. Adding by name uses an offline UUID | [Code](../../internal/command/whitelist_cmds.go) |
| RCON and GS4 Query | Implemented | Enabled in configuration before startup | [Code](../../cmd/vitis/main.go) |
| Process and chunk metrics | Partial | Available in the tab list; no configured Prometheus/pprof HTTP endpoint | [Code](../../cmd/vitis/live_tab.go) |
| Bukkit/Paper plugins and Forge/Fabric mods | Unsupported | No compatible runtime | [Code](../../internal/session/play_policy.go) |
| Other Minecraft versions and Bedrock Edition | Unsupported | Only the Java Edition 1.21.4 protocol is supported | [Code](../../internal/protocol/state.go) |

## Known issues

- Some YAML settings are not used by the server. They are marked in the [configuration reference](configuration.md).
- Cross-region entity moves do not have a crash-atomic transaction. Complete passenger and species state support remains pending.
- Most commands that change blocks or world state still target the default world. Item grants and personal spawn points resolve the target player's dimension.

See the [roadmap](roadmap.md) for development plans.
