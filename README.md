# Vitis

[![License](https://img.shields.io/badge/License-GPLv3-blue?style=for-the-badge)](LICENSE)
[![Status](https://img.shields.io/badge/Status-WIP-orange?style=for-the-badge)]()
[![Minecraft](https://img.shields.io/badge/Minecraft-1.21.4-green?style=for-the-badge)]()
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev)

[Русская версия](README_RU.md)

A high-performance Minecraft server software written in Go. Currently supports **Minecraft 1.21.4** (protocol 769).

> **Status: Experimental / Work in Progress**
> Vitis is under active development. It is not yet suitable for production use.

## Features

### Networking & Protocol
- Full Minecraft 1.21.4 protocol implementation (handshake, login, configuration, play)
- Online-mode authentication (Mojang session servers, RSA encryption)
- Packet compression (zlib)
- Hot-reloadable server configuration

### World
- Anvil region file read/write (.mca)
- Noise-based terrain generation (simplex hills, water, caves)
- Flat world generation
- Async chunk loading with bounded worker pool
- Chunk streaming with per-player view distance and priority queue
- World border with lerp transitions
- Day/night cycle (world time ticking)

### Gameplay
- Player movement, position sync, and teleportation
- Block placement and breaking with world persistence
- Creative and survival game modes
- Fall damage, hunger, natural regeneration, starvation
- Death, respawn, and combat (PvP with knockback, attack cooldown, weapon damage, sweeping edge, critical hits)
- Inventory management (click, shift-click, drag, number keys)
- Crafting system (1284 recipes, 2×2/3×3 grid matching with mirror support)
- Food eating with 40+ item registry and hunger/saturation restoration
- Mining speed with tool tiers, harvest rules, and block drops
- Entity attributes with modifiers and dirty tracking
- Armor and equipment system with vanilla damage reduction
- Player data persistence (position, game mode, health, inventory)

### Entities
- Entity physics with AABB collision detection
- Item entities with pickup, merging, and despawn
- Experience orbs with player-seeking behavior
- TNT with fuse countdown and explosion ray-casting
- Projectiles (arrows, snowballs, eggs, ender pearls)
- Vehicles (boats, minecarts)
- Falling blocks

### Multiplayer
- Entity sync and visibility (animation, sneaking, sprinting, equipment)
- Scoreboard and teams
- Boss bars
- Tab list with header/footer
- Chat (profiled disguised chat messages)
- Tab completion for commands
- Sound and particle broadcast
- Operator permissions system

### Infrastructure
- 59 code generators for protocol data (blocks, items, entities, sounds, etc.)
- Full registry system (78+ registries from vanilla data)
- Structured logging with configurable levels
- Tick loop with configurable TPS and overload handling
- Event bus with priority-sorted dispatch
- Command system with tab completion

## Roadmap

See the full roadmap with categorized short-term, medium-term, and long-term goals:

- **[Roadmap (English)](docs/eng/roadmap.md)**
- **[Roadmap (Russian)](docs/rus/roadmap.md)**

## Getting Started

### Requirements

- **Go 1.23** or later
- **Git**

### Build

```bash
git clone https://github.com/vitismc/vitis.git
cd vitis
go build -o vitis ./cmd/vitis
```

### Run

```bash
# Copy the example config
cp configs/vitis.yaml vitis.yaml

# Start the server
./vitis -config vitis.yaml
```

The server listens on `0.0.0.0:25565` by default. Connect with a Minecraft 1.21.4 client.

### Configuration

See [`configs/vitis.yaml`](configs/vitis.yaml) for all available options including:
- Server settings (host, port, max players, online mode, MOTD)
- Network tuning (timeouts, compression, buffer sizes)
- World settings (view distance, chunk workers, unload TTL)
- Tick settings (target TPS, catch-up behavior)
- Logging (level, format)

## Architecture

```
cmd/vitis/          Entry point
internal/
  network/          TCP listener, connection management, worker pools
  protocol/         Packet codec, VarInt, buffer, state machine
  session/          Player session lifecycle, packet routing, handlers
  registry/         Minecraft registry system (78+ registries)
  world/            World manager, chunk system, terrain generation
    chunk/          Chunk loading, storage, async workers
    streaming/      Per-player chunk streaming with priority queue
    terrain/        Noise and flat terrain generators
    region/         Anvil .mca file format
    level/          level.dat and player data persistence
    persistence/    Chunk store abstraction
  entity/           Entity types, physics, metadata
    physics/        AABB collision, gravity, movement
    projectile/     Arrows, thrown items
    vehicle/        Boats, minecarts
  block/            Block registry, behaviors, fluids
  item/             Item registry
  inventory/        Container and window management
  command/          Command registry and built-in commands
  config/           YAML configuration with hot-reload
  logger/           Structured logging (log/slog)
  event/            Event bus
  tick/             Tick loop and scheduler
  data/
    generator/      59 code generators
    generated/      Generated Go code (DO NOT EDIT)
  scoreboard/       Scoreboard and teams runtime
  bossbar/          Boss bar runtime
  chat/             Text component encoding
  nbt/              NBT encoder/decoder
  auth/             Mojang authentication
  operator/         Operator permissions
configs/            Example configuration files
docs/               Documentation (English & Russian)
scripts/            Build and data-fetching scripts
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on building, testing, and submitting changes.

## Documentation

Detailed documentation is available in [`docs/`](docs/):

- **[English](docs/eng/)** — Full documentation in English
- **[Russian](docs/rus/)** — Full documentation in Russian

## License

Vitis is licensed under the [GNU General Public License v3.0](LICENSE).
