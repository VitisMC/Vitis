# Roadmap

This document outlines the current state and planned development direction for Vitis.

---

## Completed

### 🌐 Server & Networking
- [x] Full Minecraft 1.21.4 protocol (handshake, status, login, configuration, play)
- [x] Online-mode authentication (Mojang session servers)
- [x] RSA encryption and CFB8 stream cipher
- [x] Packet compression (zlib with configurable threshold)
- [x] Structured packet codec with VarInt, NBT, buffer pooling
- [x] Hot-reloadable YAML configuration
- [x] Structured logging with colored console output (log/slog)
- [x] Tick loop with configurable TPS and overload handling
- [x] Event bus with priority-sorted dispatch
- [x] Graceful shutdown with signal handling
- [x] 59 code generators for protocol data
- [x] 78+ registry entries from vanilla data

### 🌍 World
- [x] Anvil region file read/write (.mca)
- [x] Noise terrain generation (simplex hills, water, caves, stone layers)
- [x] Flat world generation
- [x] Async chunk loading with bounded worker pool
- [x] Per-player chunk streaming with Manhattan-distance priority queue
- [x] Block placement and breaking with world persistence
- [x] World border with lerp transitions
- [x] Day/night cycle (world time ticking)
- [x] Furnace smelting, blast furnace, smoker (full tick with fuel, recipes, XP, NBT persistence)
- [x] Container block entities (chest, barrel, hopper, dropper, dispenser, shulker box)
- [x] Block entity GUIs (furnace progress sync, container open/interact)
- [x] TNT explosion with ray-cast block destruction

### 👤 Player
- [x] Player movement, rotation, and position sync
- [x] Creative and survival game modes with switching
- [x] Fall damage, hunger, natural regen, starvation
- [x] Death, respawn, and combat (PvP with knockback)
- [x] Inventory management (click, shift-click, drag, number keys, drop)
- [x] Player data persistence (position, game mode, health, inventory)
- [x] Operator permissions system
- [x] Hunger/saturation/exhaustion tick system (natural regen, starvation)
- [x] Player skin data sync (online-mode texture properties via Mojang API)

### 🐾 Entities
- [x] AABB collision detection and physics simulation
- [x] Gravity, drag, step-up, terminal velocity per entity type
- [x] Item entities with pickup, merging, and despawn timer
- [x] Experience orbs with player-seeking flight
- [x] TNT with fuse countdown and explosion ray-casting
- [x] Projectiles (arrows with damage/piercing, snowballs, eggs, ender pearls)
- [x] Vehicles (boats with wood types, minecarts with type variants)
- [x] Falling blocks
- [x] Entity metadata sync and visibility

### 🎮 Survival Mechanics
- [x] Food registry (40+ items) with eating tracker, item consumption, and hunger/saturation restoration
- [x] Mining speed calculator with tool tiers, harvest rules, and progress tracking
- [x] Block drop system (item entity spawning on break)
- [x] Entity attributes system (modifiers: add/multiply_base/multiply_total, dirty tracking, bootstrap sync)
- [x] Armor and equipment system (armor/weapon registries, vanilla damage reduction formula)
- [x] Improved combat (attack cooldown, weapon-based damage, sweeping edge, critical hit gate)
- [x] Crafting system (1284 recipes from mcdata, 2×2/3×3 grid matching with mirror support)
- [x] Block placement improvements (solid collision check, player overlap prevention, directional states, placement sounds)
- [x] Client-server inventory synchronization (authoritative state ID, full resync on every click)
- [x] Chunk keepalive (player-visible chunks stay loaded in the chunk manager)

### 💬 Multiplayer
- [x] Entity animation, sneaking, sprinting, equipment broadcast
- [x] Scoreboard and teams runtime
- [x] Boss bars with per-viewer tracking
- [x] Tab list with header/footer
- [x] Chat (disguised chat messages)
- [x] Command system with tab completion (20 built-in commands)
- [x] Sound and particle broadcast

---

## In Progress

- [ ] **Loot tables** — evaluation engine exists, needs wiring to block/entity drops
- [ ] **Metrics & profiling** — profiler and stats structs exist, need server loop integration
- [ ] **Storage abstraction** — KV interface exists, needs persistence backend

---

## Planned

### Short Term

#### 🌍 World
- [ ] Stonecutter interaction
- [ ] Campfire cooking block entity
- [ ] Enchantment table and anvil
- [ ] Brewing stand and potion brewing
- [ ] Hopper item transfer tick
- [ ] Biome-aware world generation (temperature, humidity, terrain shape)
- [ ] Structure generation (villages, dungeons, mineshafts, strongholds)
- [ ] Light engine (block light propagation, sky light)
- [ ] Redstone basics (wire, torch, repeater, comparator, piston, lever, button, pressure plate)
- [ ] Falling sand/gravel physics
- [ ] Crop growth (wheat, carrots, potatoes, beetroot)
- [ ] Tree growth from saplings
- [ ] Fire spread and extinguishing
- [ ] Creeper and ghast explosion block damage

#### 👤 Player
- [ ] Status effects and potions (speed, strength, poison, regeneration, etc.)
- [ ] Experience system (enchanting cost, repair cost, anvil)
- [ ] Advancements and achievement tracking
- [ ] Recipe book unlocking
- [ ] Bed sleeping and spawn point setting
- [ ] Player-to-player trading (drop-based)

#### 🐾 Entities
- [ ] Basic mob AI (pathfinding, targeting, attack)
- [ ] Passive mobs (cow, pig, sheep, chicken) with drops
- [ ] Hostile mobs (zombie, skeleton, creeper, spider) with basic behavior
- [ ] Mob spawning rules (light level, biome, spawn limits)
- [ ] Mob loot drops (using loot table engine)
- [ ] Animal breeding
- [ ] Villager trading
- [ ] Armor stand entities

#### 🌐 Server
- [ ] Plugin API — extensibility for third-party plugins
- [ ] Persistent storage backend (LevelDB)
- [ ] Server-side resource pack delivery
- [ ] Ban list (player bans, IP bans)
- [ ] Whitelist
- [ ] RCON remote console
- [ ] Query protocol (server status for external tools)

### Medium Term

#### 🌍 World
- [ ] Multi-world support (Overworld, Nether, End with portals)
- [ ] Dimension transitions (portal linking, coordinate scaling)
- [ ] Weather system (rain, thunder, lightning strikes)
- [ ] World pregeneration tool
- [ ] WorldEdit-style region manipulation API
- [ ] Custom world generators (API for plugins)
- [ ] Advanced redstone (observer, sticky piston, slime blocks, honey blocks)

#### 👤 Player
- [ ] Creative inventory GUI (search, survival inventory tab)
- [ ] Spectator mode (noclip, entity camera, invisible)
- [ ] Adventure mode (block interaction restrictions)
- [ ] Book and quill editing
- [ ] Map item rendering
- [ ] Fishing
- [ ] Shield blocking and cooldown

#### 🐾 Entities
- [ ] Advanced mob AI (fleeing, herding, patrol, raids)
- [ ] Boss mobs (Ender Dragon, Wither)
- [ ] Neutral mobs (wolf, bee, iron golem, enderman)
- [ ] Water mobs (fish, dolphin, squid, guardian)
- [ ] Mount riding (horse, pig with saddle, strider)
- [ ] Mob equipment (armor, weapons, drops)
- [ ] Entity leashing and name tags

#### 🌐 Server
- [ ] Proxy protocol support (Velocity, BungeeCord)
- [ ] Metrics export (Prometheus-compatible)
- [ ] Web-based admin panel
- [ ] Hot-pluggable plugin loading/unloading
- [ ] Permission system (groups, per-node permissions)
- [ ] Scheduled tasks and cron-like system
- [ ] Chunk-level access control (claim system API)

### Long Term

#### 🌍 World
- [ ] Custom dimensions (datapack-driven)
- [ ] World format migration tools
- [ ] Schematic import/export

#### 🌐 Server
- [ ] Protocol version negotiation (multi-version support: 1.20.x–1.21.x)
- [ ] Cluster mode (distributed server across multiple nodes)
- [ ] Performance benchmarking suite with automated regression detection
- [ ] Replay recording and playback
- [ ] Anti-cheat framework (movement validation, reach checks, speed checks)
- [ ] REST API for external integrations (player data, world state, server control)
