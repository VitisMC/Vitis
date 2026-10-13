# Datapacks and tab list

[Русский](../rus/resources.md) · [Documentation](README.md)

## Resource loading

At startup Vitis prepares a missing `<world>/vanilla/1.21.4/` pack without Java, then loads it and user datapacks from `<world>/datapacks/`. Folders and ZIP archives are supported. User packs are sorted by path; definitions in later packs replace earlier ones.

The world root comes from `world.default_world_name`. Vanilla is loaded exactly once, before user packs. An existing `.mcdata/<version>/vanilla` is validated before use as a migration source. A pack with `version.json` must match all binary version constants. Older Vitis packs without that file are accepted only with the exact supported-version description and pack format, then receive version identity in the world copy.

A pack needs `pack.mcmeta` with data pack format **61**. Resource keys use `data/<namespace>/<type>/<path>`, such as `data/minecraft/recipe/example.json`. The 1.21.4 directories `recipe`, `loot_table` and `advancement` are singular.

The loader rejects symlinks, invalid paths, malformed JSON, duplicate ZIP entries and oversized input. Limits: 32 MiB per file, 512 MiB per pack and 65,536 ZIP entries. A syntactically valid JSON file may still fail a subsystem compiler.

Recipes, loot, natural spawning, advancements and worldgen register compilers before the first snapshot. `/reload` loads and compiles a complete candidate, then atomically publishes it. If any compiler fails, the old snapshot remains active.

To test an override, copy a vanilla recipe JSON into the same relative path under a new world pack, include metadata and change a supported result/count. Run `reload`, check the console, then verify crafting with a client. Installing a datapack does not require changing generated Go code.

Natural spawning reads biome `spawners`, `spawn_costs`, dimension light/bounds and the `more_frequent_drowned_spawns` biome tag from the compiled snapshot. Charge and energy budget must be finite. Drowned depth uses the active dimension's existing sea-level setting; this adds no world generation behavior.

Loot expands composite entries and item tags before weighted selection, then applies singleton, nested-table, pool and table functions. `set_components` uses the complete stack codec and rejects malformed or unknown components before publication. Death-cause predicates resolve required/excluded `damage_type` tags from the same snapshot; malformed or missing required tag references reject reload. Constant, uniform and binomial number providers support nesting up to 16 levels; binomial trial counts are limited to 65,536. Other number providers fail compilation. Complete loot-function and predicate coverage remains pending.

## Tab list

`configs/tab.yaml` contains `header` and `footer` strings. Both support Minecraft `§` color codes and newline escapes. The path is fixed relative to the working directory; the file is excluded from Git. Startup creates a missing file using the built-in templates. Invalid input or two empty templates falls back to [DefaultTabConfig](../../internal/config/tab.go). Restart to reload templates.

| Placeholder | Value |
| --- | --- |
| `{server}` | Literal Vitis |
| `{online}`, `{max}` | Current player count and configured capacity |
| `{tps}` | Smoothed server TPS |
| `{cpu}` | Process CPU percentage; one busy logical core is 100% |
| `{ram_mib}` | Resident process memory in MiB |
| `{generated_per_sec}`, `{sent_per_sec}` | New chunk generation and chunk packet send rates |
| `{chunk_avg_ms}`, `{chunk_last_ms}` | Generation duration, interval mean and last sample |
| `{full_path_per_sec}` | New generated chunks first acknowledged by clients per second |
| `{full_path_avg_ms}`, `{full_path_last_ms}` | Generation start to first client acknowledgement, mean/last |

Updates are sent once per second. [Pregeneration metrics](pregeneration.md) separately track chunk writes to disk.

Sources: [pack loader](../../internal/datapack/manager.go), [world pack discovery](../../internal/datapack/discovery.go), [live tab](../../cmd/vitis/live_tab.go).
