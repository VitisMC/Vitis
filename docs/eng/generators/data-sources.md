# Minecraft data sources

[Русский](../../rus/generators/data-sources.md) · [Documentation](../README.md)

## Files and sources

| Artifact | Source / role |
| --- | --- |
| `mojang-version.json` | Version metadata selected from Mojang's official manifest |
| `.mc-decompiled/downloads/<version>/server.jar` | Official bundled server; verified against metadata SHA-1 |
| `server_mappings.txt` | Official Mojang mappings with verified SHA-1 |
| `<world>/vanilla/<version>/` | Runtime resources, pack metadata and version identity |
| `.mcdata/<version>/vanilla` | Developer data-generator output plus embedded data and reports; preserved migration source |
| `registries.json`, `datapacks/`, `tags/`, `recipes/`, `loot_tables/` | Copies derived from the official vanilla pack for code generators |
| Versioned PrismarineJS JSON | Blocks, items, entities, protocol, biomes, shapes and related codec/data inputs |
| `source.lock` | Pinned minecraft-data commit and official JAR/mapping hashes |
| `checksums.sha256` | SHA-256 inventory of staged source artifacts |
| `.mc-decompiled/<version>-decompiled` | Automatically prepared named Java source for source-based generators |
| `.mc-decompiled/<version>-remapped.jar` | Automatically prepared remapped runtime JAR for Java exporters |

The current full updater uses official JAR-derived datapack data and a pinned [PrismarineJS minecraft-data repository](https://github.com/PrismarineJS/minecraft-data). The current workflow does not use misode snapshots.

## Reproducibility

Select one Minecraft version throughout the JAR, mappings, resources, named source and protocol JSON. Check Mojang metadata hashes, resource metadata, pinned source commit and generated diffs.

`vitis-data` verifies the bundled version metadata and pack content but does not download the JAR or verify it against Mojang's published hash; automatic server preparation and the full updater perform that checksum step.

The updater prepares named/remapped artifacts automatically using pinned MinecraftDecompiler and Vineflower. Their input/tool fingerprint and full output checksums are recorded in `.mc-decompiled/<version>.sources.lock` and `.sources.sha256`. Java exporters document their additional requirements in the [generation-traits guide](../../../internal/data/generator/block/worldgen/README.md).

## Source code

[Linux preparation](../../../scripts/linux/update_version.sh), [Windows preparation](../../../scripts/windows/update_version.ps1), [runtime extraction](../../../internal/data/vanilla/prepare.go), [developer generation](../../../internal/data/vanilla/generate.go), [CLI](../../../cmd/vitis-data/main.go), [generators](../../../internal/data/generator/).

See [version upgrades](version-upgrade.md) for porting the core.
