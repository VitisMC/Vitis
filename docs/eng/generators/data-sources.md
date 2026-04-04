# Data Sources

External data sources used by Vitis generators. All data is stored under `.mcdata/{version}/`.

## Primary Sources

### 1. Minecraft Server JAR — `registries.json`

**Source:** Mojang's official server JAR, built-in data generator.

**Produces:** `registries.json` — all 80 builtin registry name↔ID mappings with protocol IDs.

**Format:**
```json
{
  "minecraft:block": {
    "protocol_id": 3,
    "entries": {
      "minecraft:air": { "protocol_id": 0 },
      "minecraft:stone": { "protocol_id": 1 }
    }
  }
}
```

**How it works in `update_version.sh`:**
1. Server JAR is downloaded by `extract_loot_tables.sh` (cached at `.mc-decompiled/downloads/{version}/server.jar`)
2. Data generator runs: `java -DbundlerMainClass="net.minecraft.data.Main" -jar server.jar --reports`
3. Output `generated/reports/registries.json` is copied to `.mcdata/{version}/registries.json`

**Used by:** 14 generators (attribute, fluid, potion, game_event, screen, data_component, block_entity, villager_profession, villager_type, cat_variant, frog_variant, message_type, registry, tag).

### 2. PrismarineJS/minecraft-data

**Repository:** https://github.com/PrismarineJS/minecraft-data

**Files downloaded:**
- `blocks.json` — Block definitions, states, properties
- `items.json` — Item definitions, stack sizes
- `entities.json` — Entity types, dimensions
- `protocol.json` — Packet definitions for all states
- `sounds.json` — Sound event names and IDs
- `particles.json` — Particle type IDs
- `effects.json` — Potion effect definitions
- `biomes.json` — Biome definitions
- `foods.json` — Food properties
- `recipes.json` — Crafting recipes
- `language.json` — Translation keys
- `blockCollisionShapes.json` — Block collision shapes
- `materials.json`, `tints.json`, `enchantments.json`

**URL pattern:** `https://raw.githubusercontent.com/PrismarineJS/minecraft-data/master/data/pc/{version}/{file}.json`

**Versions:** 1.8 through latest. Updated within days of release.

### 3. misode/mcmeta — datapacks & tags

**Repository:** https://github.com/misode/mcmeta

**Used for:**
- **Configuration registry datapacks** — JSON definitions for registries sent to clients during Configuration phase
- **Tags** — Named groups of registry entries (block tags, item tags, etc.)

**Tag:** `{version}-data-json` (e.g. `1.21.4-data-json`)

**Datapacks downloaded** (under `.mcdata/{version}/datapacks/`):
- `banner_pattern/`, `chat_type/`, `damage_type/`, `dimension_type/`
- `enchantment/`, `instrument/`, `jukebox_song/`, `painting_variant/`
- `trim_material/`, `trim_pattern/`, `wolf_variant/`
- `worldgen/biome/`, `worldgen/structure/`, `worldgen/noise/`
- `worldgen/configured_feature/`, `worldgen/placed_feature/`, `worldgen/noise_settings/`

**Tags downloaded** (under `.mcdata/{version}/tags/`):
- `block/`, `entity_type/`, `fluid/`, `item/`

**Versions:** 1.14 through latest. Updated within hours of release.

### 4. Minecraft Server JAR — decompiled source

**Tool:** MaxPixelStudios/MinecraftDecompiler (https://github.com/MaxPixelStudios/MinecraftDecompiler)

**Used for:** 15 generators that parse decompiled Java source code to extract constants and mappings not available in public JSON data (entity poses, game rules, world events, tracked data, etc.).

**Process:** `update_version.sh` Step 5 decompiles the server JAR with Vineflower. Generators in the `GENERATORS` array read from `.mc-decompiled/{version}-decompiled/`.

## Fallback Sources

- **Official Minecraft Wiki** — https://minecraft.wiki
- **Articdive/ArticData** — https://github.com/Articdive/ArticData

## Version Availability

| Source | Min Version | Update Frequency |
|--------|-------------|------------------|
| PrismarineJS | 1.7.10 | Days after release |
| misode/mcmeta | 1.14 | Hours after release |
| Server JAR data generator | Any | Immediate (from Mojang) |
| Decompiled JAR | Any | Immediate (requires Java) |
