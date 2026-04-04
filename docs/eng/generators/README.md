# Vitis Data Generators

Code generation system that converts Minecraft data files into Go source code for protocol compatibility.

## Overview

- **Type safety** — compile-time verification of IDs and constants
- **Performance** — zero-allocation lookups via generated maps
- **Maintainability** — single source of truth for each data type

All generators live in `internal/data/generator/<name>/main.go` and output to `internal/data/generated/<name>/`.

## Quick Start

```bash
# Update everything for a specific version (recommended)
./scripts/update_version.sh 1.21.4

# Or run a specific generator
go run internal/data/generator/block/main.go -version 1.21.4
```

## Generators (59 total)

### From PrismarineJS JSON files

| Generator | Input | Description |
|-----------|-------|-------------|
| `block` | `blocks.json` | Block types, states, properties (1095 blocks, 27866 states) |
| `item` | `items.json` | Item types, stack sizes (1385 items) |
| `entity` | `entities.json` | Entity types, dimensions (149 entities) |
| `sound` | `sounds.json` | Sound event IDs (1651 sounds) |
| `particle` | `particles.json` | Particle type IDs (112 particles) |
| `effect` | `effects.json` | Potion effect definitions (39 effects) |
| `biome` | `biomes.json` | Biome definitions (65 biomes) |
| `spawn_egg` | `items.json` | Spawn egg items (81 eggs) |
| `packet` | `protocol.json` | Packet IDs for all protocol states |
| `recipes` | `recipes.json` | Crafting recipes (1557 recipes) |
| `translation` | `language.json` | Translation keys (7000 translations) |
| `collision_shapes` | `blockCollisionShapes.json` + `blocks.json` | Block collision AABBs (4989 shapes) |

### From `registries.json` (server JAR data generator)

| Generator | Registry key | Description |
|-----------|-------------|-------------|
| `attribute` | `minecraft:attribute` | Attribute types (31) |
| `game_event` | `minecraft:game_event` | Game events (60) |
| `fluid` | `minecraft:fluid` | Fluid types (5) |
| `potion` | `minecraft:potion` | Potion types (46) |
| `screen` | `minecraft:menu` | Screen/menu types |
| `data_component` | `minecraft:data_component_type` | Data component types |
| `block_entity` | `minecraft:block_entity_type` | Block entity types |
| `villager_profession` | `minecraft:villager_profession` | Villager professions |
| `villager_type` | `minecraft:villager_type` | Villager types |
| `cat_variant` | `minecraft:cat_variant` | Cat variants |
| `frog_variant` | `minecraft:frog_variant` | Frog variants |
| `message_type` | `minecraft:message_type` | Message types |
| `registry` | all registries | Full registry system (80 registries + config NBT + tags) |
| `tag` | all tags | Tag→ID mappings for 13 registries |

### From misode/mcmeta datapacks

| Generator | Datapack dir | Description |
|-----------|-------------|-------------|
| `damage_type` | `datapacks/damage_type` | Damage type definitions (49) |
| `dimension` | `datapacks/dimension_type` | Dimension type definitions (4) |
| `enchantment` | `datapacks/enchantment` | Enchantment definitions (42) |
| `jukebox_song` | `datapacks/jukebox_song` | Jukebox songs (19) |
| `painting_variant` | `datapacks/painting_variant` | Painting variants (50) |
| `wolf_variant` | `datapacks/wolf_variant` | Wolf variants (9) |
| `instrument` | `datapacks/instrument` | Instruments (8) |
| `trim_pattern` | `datapacks/trim_pattern` | Armor trim patterns (18) |
| `trim_material` | `datapacks/trim_material` | Armor trim materials (11) |
| `banner_pattern` | `datapacks/banner_pattern` | Banner patterns (43) |
| `structures` | `datapacks/worldgen/structure` | Structure definitions |
| `noise_parameter` | `datapacks/worldgen/noise` | Noise parameters |
| `configured_features` | `datapacks/worldgen/configured_feature` | Configured features |
| `placed_features` | `datapacks/worldgen/placed_feature` | Placed features |
| `chunk_gen_settings` | `datapacks/worldgen/noise_settings` | Chunk generation settings |

### From decompiled server JAR

| Generator | Source | Description |
|-----------|--------|-------------|
| `chunk_status` | `ChunkStatus.java` | Chunk statuses (12) |
| `entity_pose` | `Pose.java` | Entity poses (18) |
| `entity_status` | `EntityEvent.java` | Entity status events (59) |
| `meta_data_type` | `EntityDataSerializers.java` | Metadata serializer types (31) |
| `sound_category` | `SoundSource.java` | Sound categories (10) |
| `world_event` | `LevelEvent.java` | World events (82) |
| `scoreboard_slot` | `DisplaySlot.java` | Scoreboard display slots (19) |
| `game_rules` | `GameRules.java` | Game rules (53) |
| `tracked_data` | `Entity*.java` | Entity tracked data fields (212) |
| `composter` | `ComposterBlock.java` | Compostable items (108) |
| `flower_pot` | `FlowerPotBlock.java` | Flower pot blocks (2) |
| `fuels` | `AbstractFurnaceBlockEntity.java` | Furnace fuels (41) |
| `smelting` | `*Recipe.java` | Cooking recipes (113) |
| `status_effect` | `MobEffect.java` | Status effects (23) |
| `potion_brewing` | `PotionBrewing.java` | Brewing recipes (41) |

### Placeholder/Remap generators

| Generator | Description |
|-----------|-------------|
| `block_state_remap` | Block state ID remap (version migration) |
| `entity_id_remap` | Entity ID remap (version migration) |
| `item_id_remap` | Item ID remap (version migration) |
| `recipe_remainder` | Recipe remainder items |

## Documentation

- [Version Upgrade Guide](version-upgrade.md) — Updating to a new Minecraft version
- [Data Sources](data-sources.md) — Where data files come from

## Directory Structure

```
.mcdata/
└── 1.21.4/
    ├── registries.json          # Generated from server JAR (all 80 registry IDs)
    ├── blocks.json              # PrismarineJS
    ├── items.json               # PrismarineJS
    ├── entities.json            # PrismarineJS
    ├── protocol.json            # PrismarineJS
    ├── sounds.json              # PrismarineJS
    ├── particles.json           # PrismarineJS
    ├── effects.json             # PrismarineJS
    ├── biomes.json              # PrismarineJS
    ├── language.json            # PrismarineJS
    ├── blockCollisionShapes.json # PrismarineJS
    ├── datapacks/               # misode/mcmeta (config registry data)
    │   ├── damage_type/
    │   ├── dimension_type/
    │   ├── enchantment/
    │   ├── worldgen/biome/
    │   ├── worldgen/structure/
    │   ├── worldgen/noise/
    │   └── ...
    └── tags/                    # misode/mcmeta (tag definitions)
        ├── block/
        ├── item/
        ├── entity_type/
        └── ...

internal/data/
├── generator/                   # Code generators (59)
│   ├── block/main.go
│   ├── item/main.go
│   └── ...
└── generated/                   # Generated Go code (DO NOT EDIT)
    ├── block/blocks.go
    ├── item/items.go
    └── ...
```
