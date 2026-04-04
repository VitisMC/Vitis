# Architecture Overview

Vitis is structured as a layered system where each layer has clear responsibilities and minimal coupling to other layers.

## Layers

### 1. Network Layer (`internal/network/`)

The lowest layer. Handles raw TCP connections, byte framing, and I/O worker pools.

- **Listener** — accepts incoming TCP connections with configurable max connection limit
- **Conn** — per-connection read/write loops with deadline management
- **Pipeline** — extensible inbound handler chain (frame decode → session dispatch)
- **BufferPool** — `sync.Pool`-based buffer recycling to minimize GC pressure
- **WorkerPool** — bounded goroutine pool for packet processing

### 2. Protocol Layer (`internal/protocol/`)

Implements the Minecraft 1.21.4 wire protocol.

- **Buffer** — zero-copy read/write buffer for protocol types (VarInt, VarLong, String, UUID, NBT, etc.)
- **Registry** — version-aware and state-aware packet ID → constructor mapping
- **Decoder/Encoder** — frame → packet decoding and packet → frame encoding
- **States** — Handshake, Status, Login, Configuration, Play packet registrations
- **Crypto** — RSA key generation, CFB8 stream cipher for online-mode encryption

### 3. Session Layer (`internal/session/`)

Manages the lifecycle of a connected player from handshake through play.

- **Session** — state machine (handshake → login → configuration → play)
- **PacketRouter** — copy-on-write handler dispatch by (state, packet ID)
- **Manager** — thread-safe session tracking with ID and connection lookup
- **Handlers** — status, login (with online-mode auth), configuration (registry sync), play (gameplay)

### 4. World Layer (`internal/world/`)

Owns all world state. Mutations happen exclusively on the world tick goroutine.

- **World** — central world state: chunks, players, entities, time, border
- **Manager** — multi-world container (currently single world)
- **Chunk system** (`chunk/`) — async load/generate with bounded worker pool, results applied on tick
- **Streaming** (`streaming/`) — per-player chunk streaming with Manhattan-distance priority queue
- **Terrain** (`terrain/`) — simplex noise generator and flat generator
- **Region** (`region/`) — Anvil .mca file read/write
- **Level** (`level/`) — level.dat and player data NBT persistence

### 5. Entity Layer (`internal/entity/`)

Entity types with physics simulation.

- **Player** — extends LivingEntity with session binding, view distance
- **LivingEntity** — health, food, XP, damage, respawn
- **Physics** (`physics/`) — AABB collision, gravity, step-up, block collision scanning
- **Item/XPOrb/TNT** — entity types with specific tick behavior
- **Projectiles** (`projectile/`) — arrows, thrown items with ray-cast hit detection
- **Vehicles** (`vehicle/`) — boats, minecarts with passenger management

### 6. Registry Layer (`internal/registry/`)

Pre-built Minecraft registry data for client synchronization.

- 78+ builtin registries from `registries.json`
- 12 config registries from datapacks (NBT-encoded)
- Tag system for biome/enchantment grouping
- Zero-allocation ID ↔ name lookups

### 7. Data Generation (`internal/data/`)

59 code generators that convert Minecraft data files into type-safe Go code.

- Input: PrismarineJS JSON, `registries.json`, misode/mcmeta datapacks, decompiled server JAR
- Output: `internal/data/generated/` — compile-time verified IDs and constants

## Threading Model

```
Main goroutine       → bootstrap, signal handling, shutdown
Tick loop goroutine  → world.Tick() at 20 TPS
  └─ chunk workers   → async chunk load/generate (bounded pool)
Network I/O workers  → read loops per connection
Packet worker pool   → decode + dispatch (bounded pool)
Per-session write    → async outbound queue per connection
Config watcher       → file change detection for hot-reload
Console reader       → stdin command dispatch
```

## Data Flow

```
Client TCP → Conn read loop → frame decode → Pipeline
  → Session.processInboundFrame → Decoder → PacketRouter → Handler
  → Handler mutates state / schedules world action
  → World.Tick() applies mutations
  → Send packet → Session outbound queue → Encoder → Conn write loop → Client TCP
```
