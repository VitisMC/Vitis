# World runtime

[Русский](../rus/world.md) · [Documentation](README.md)

## Dimensions and ticks

[Server startup](../../cmd/vitis/main.go) creates the Overworld, Nether and End, and additional dimensions from loaded generation resources. Data is stored in the world root, `DIM-1`, `DIM1` and `dimensions/<namespace>/<path>`.

The main [tick loop](../../internal/tick/loop.go) advances registered worlds. The [world tick](../../internal/world/world.go) runs queued tasks, applies a bounded number of background results, updates blocks, entities and players, streams chunks, and coordinates autosave and pregeneration. `tick.target_tps` sets the target frequency.

The two schedulers serve different purposes: the world queue accepts work from other goroutines, while the block and fluid scheduler tracks delayed game ticks. Disk IO and long calculations run separately.

## Chunk lifecycle

[Chunk manager](../../internal/world/chunk/manager.go) tracks pending and ready chunks and unload requests. [Loader](../../internal/world/chunk/loader.go) uses bounded workers and result queues. Requests are dispatched in batches; results are applied during ticks.

Check that a requested chunk is ready before reading or changing it. Handle load errors and full queues. Chunks visible to players stay in memory; unused chunks are scanned and unloaded in bounded batches.

In-memory chunks differ from their NBT/Anvil representation. Section palettes, block entities, heightmaps, lighting and dirty versions must remain consistent after block changes.

## Streaming

[Streaming](../../internal/world/streaming/) tracks player views, prioritizes nearby chunks and serializes ready data. The server setting caps client view distance. Send work is bounded per player and tick, and adjusted using client acknowledgements.

Packet sends, client acknowledgements and disk writes are measured separately. See [resources](resources.md) for tab-list and pregeneration metrics.

## Saving

[Autosave](../../internal/world/autosave.go) takes snapshots during ticks and sends them to bounded background queues. Save acknowledgements account for dirty versions: a chunk changed after its snapshot remains marked for saving. Chunk unload and server shutdown also finish pending writes.

[ChunkStore](../../internal/world/persistence/store.go) saves chunks in Anvil regions. Background writes use snapshots so changes to in-memory chunks do not change the data being saved.

[Entity persistence](../../internal/world/entity_persistence.go) loads entity-region data on chunk workers and restores implemented entities on the world tick. Runtime snapshots are written to `entities/*.mca` by the bounded chunk writer. A chunk stays resident until both its block data and entity payload are acknowledged at the current revision. Entity-only changes retain the cached block packet. Items, XP orbs, primed TNT, falling blocks, registered basic mobs, arrows, thrown projectiles and tridents have connected snapshot/restore. UUIDs, full item stacks, supported type state and unknown NBT fields are preserved. Unknown entity types, passenger graphs and XP orbs with `Count > 1` remain opaque; their runtime mechanics are unsupported. Cross-region moves are not crash-atomic. [Lifecycle tests](../../internal/world/entity_persistence_test.go) cover reload, restart, deduplication and save failures.

## Generation and compatibility

The main generator uses compiled datapack resources in [worldgen](../../internal/world/worldgen/). Older flat-world and noise helpers remain in the code but are not selected during normal startup. Generation still differs from vanilla.

The [comparison tool](../../cmd/vitis-worldgen-compare/main.go) compares output with a saved 1.21.4 world. Use the same seed, dimension, resource version and coordinates. Inspect block, air and biome differences, including generated feature boundaries.

Read [storage](storage.md) before opening an existing world and [pregeneration](pregeneration.md) before large jobs. Tests cover chunk management, loading, sections, autosave, storage, scheduling, world generation and pregeneration.

## Simulation foundation

Player simulation tickets use `simulation_distance`; visibility tickets use the effective client view distance. Spawn, portal and forced tickets retain chunks independently. Block ticking requires loaded one-ring dependencies and entity ticking requires two rings. Items, XP, TNT, falling blocks, projectiles, mobs, AI and natural spawning respect activity. Pregeneration alone acquires neither simulation activity nor runtime light observers. Unavailable blocks are not treated as air by actions or physics.

Scheduled ticks are attached on publication, written in `block_ticks` / `fluid_ticks` from immutable snapshots, and removed from runtime queues only after durable eviction. Inactive queues retain due work without blocking active chunks. Production unload uses the bounded background writer; failed writes retain state for retry. `randomTickSpeed` samples eligible sections for wheat, carrots, potatoes and beetroots using light, moisture and crowding conditions. Light source changes rebuild bounded neighborhoods and send standalone updates to current observers; Nether and End exclude skylight. Complete random-tick and block-shape lighting parity remains pending.

## Neighbor updates

[Neighbor updates](../../internal/world/neighbor_updates.go) queue resident neighbors in west/east/down/up/north/south order on the common world tick. The ring holds 32,768 events with a 100,000-callback budget per world tick. Border reconciliation scans at most 4,096 cells per tick and rotates between loaded chunks. Saturation defers current support checks into persistent block ticks; the original directional event is not retained. Saving and shutdown process queued notifications before snapshots.

Crops lose invalid support or insufficient raw light; obstructed farmland converts after a scheduled recheck. Broken rectangular portal frames collapse only when the required cells are available. Frame validation follows the portal's axis and allows missing corners. [World tests](../../internal/world/neighbor_updates_test.go) and [connected packet tests](../../internal/session/neighbor_updates_test.go) cover ordering, overflow, restart, shutdown, unknown borders and dimension isolation. FIFO processing is not full vanilla chained-update ordering. Redstone, general shape updates, waterlogging, farmland hydration/drying and entity displacement during conversion remain incomplete; crop drops do not have full loot-table parity.

## Player dimensions and transfers

[Dimension dispatch](../../internal/session/dimension_handlers.go) selects immutable adapters from the session binding. Stateful packets enter a bounded world queue; nested actions retain TCP order on the common tick. Player views scope gameplay broadcasts and sleep to their dimension while admission, tab information and chat remain global. [Transfers](../../internal/session/dimension_transfer.go) retain origin membership until destination preparation finishes, then close transient windows, detach tracking, send Respawn and a confirmed teleport, and start destination chunk streaming. Login and final/autosave snapshots use the current player dimension and portal cooldown.

The manager publishes transfers before any dimension advances. Destination preparation retains collision dependencies; disconnect cancels unfinished travel and gameplay waits for teleport confirmation. Mining, eating and shield use are cancelled on transfer, death and disconnect. Respawn resends attributes, abilities, health, experience, effects, selected slot and destination difficulty.

Nether preparation searches within 16 blocks in the Nether and 128 in the Overworld, with a shared work budget, timeout and at most sixteen concurrent transfers. End entry prepares a 5×5 platform; an existing exit portal returns a living player to their personal or world spawn. Complete portal placement and exit collision parity, mutable worldborder integration, End credits, respawn anchors, dragon exit-portal creation and non-player/passenger travel remain unfinished. Most world-changing commands still use the default world; item grants and personal spawn points use the recipient's dimension.
