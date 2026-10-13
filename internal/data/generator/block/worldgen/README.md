# Generation traits and worldgen reference cases

From the repository root:

Linux (Bash) and Windows (PowerShell 7.2+) use the same command:

```text
go run internal/data/generator/block/worldgen/main.go -version 1.21.4
```

Run `make generate` on Linux or `pwsh -NoProfile -File .\scripts\windows\update_version.ps1 -Version 1.21.4` on Windows from a fresh checkout to prepare inputs automatically. Development requires JDK 21 on PATH and the prepared local
`.mc-decompiled/1.21.4-remapped.jar`, its bundled
`.mc-decompiled/downloads/1.21.4/server.jar`, and the shipped
`.mcdata/1.21.4/vanilla` resources. The server and Go tests use only the checked-in
outputs; they do not execute Java, use CGO, or download dependencies.

The exporter boots Minecraft's block registry, queries each registered state,
and maps canonical names/properties to Vitis IDs. It rejects mismatched state
counts, unknown properties, or duplicate mappings before publication. The
1.21.4 inventory contains 27,866 states.

`internal/data/generated/block/generation_traits.go` stores a 32-bit word for
each state:

| Bits | Meaning |
| --- | --- |
| 0–5 | Full support faces: down, up, north, south, west, east |
| 6–11 | Full collision faces in the same order |
| 12 | State replacement flag (`canBeReplaced()` without placement context) |
| 13 | Water fluid, including flowing water |
| 14 | Source water fluid, including waterlogged states |
| 15 | Nonempty fluid |
| 16 | Legacy state solidity used by generation predicates |
| 17–20 | State-dependent light emission |
| 21 | Block-level `liquid()` flag, including bubble columns but excluding waterlogged solid blocks |

Shapes are queried with an empty block/collision context. This covers ordinary
generated block states, including connections encoded in state properties, but
does not model animated block entities or arbitrary context-dependent shapes.
Support and collision masks are intentionally distinct. Cached replacement and
legacy solidity are also distinct from item placement checks and full cubes.

`internal/world/worldgen/testdata/multiface_1_21_4.json` contains 362 cases executed
by Minecraft's actual multiface placement/spreading implementation against
sparse terrain. The exporter binds the shipped fire tag before sculk tests.
Cases record input and output states, placement result, and the next RNG value:

- Both glow lichen and sculk vein, 16 seeds, enclosed/coplanar/corner/search
  layouts, air, source water, flowing water, and replaceable vegetation.
- Search-range bounds, leaves, mud, soul sand, snow, slabs, and stairs on all
  six sides, including zero spreading probability.
- Directed spreading: same position, same plane, and corner wrapping; fluids,
  existing faces, replaceable plants, fire, forbidden substrates, alternate
  sculk source blocks, and blocked corners.

The Go comparisons run shared compiled features concurrently with independent
owned chunks/RNG streams and check all local mutations and neighboring writes.
Additional tests cover reload tag isolation, malformed configurations,
negative/border coordinates, block traits, waterlogging, saved/reloaded state,
and recomputed lighting. Failed pure search retries revisit the same adjacent
cell in 1.21.4; Go collapses those retries after confirming that no writes or RNG
draws occur. Reference cases pin the resulting random stream.

`internal/world/worldgen/structure_beard_generated.go` stores the raw bits of all
13,824 structure adaptation kernel coefficients. The exporter also runs 176
cases through the versioned terrain adapter, covering all five modes, negative
heights, kernel boundaries, overlapping pieces and jigsaw junctions. The Go
test checks each resulting double bit for bit.

`internal/world/worldgen/testdata/nether_fossil_sites_1_21_4.json` contains 576
cases run through the versioned Nether fossil site selector. Controlled raw
columns cover solid, empty, full-support, soul-sand, slab, lava and sea-level
boundaries. The Go test checks chosen coordinates, rotation and template, then
separately confirms that raw-column sampling matches Nether terrain generation.

`internal/world/worldgen/testdata/nether_fossil_templates_1_21_4.json` contains
all 14 fossil templates under all four rotations. The exporter loads each NBT
through Minecraft's template implementation; Go compares exact bounding boxes,
bone positions, and axes across all 56 cases.

`internal/world/worldgen/testdata/lake_1_21_4.json` contains 32 lava lake
placements from the versioned feature. Go compares every final nonstone block,
success and the remaining RNG stream across ordinary, negative, border and
minimum-height origins. The lake feature's deferred cave-air ticks and
postprocessing marks are outside this block-state comparison.

Linux (Bash) and Windows (PowerShell 7.2+):

```text
go test ./internal/world/worldgen ./internal/world/lighting -run 'Test(Multiface|Generation|FeaturePredicates|BlockEmission|StructureBeard|NetherFossil|LakeLava)' -count=1
```

Regenerating twice produces identical artifacts. The remapped input identity and checksums are recorded in `.mc-decompiled/1.21.4.sources.lock` and `.sources.sha256`; the updater checks them before reuse. The official bundled server is verified against Mojang metadata.
The generated structure kernel and three JSON reference artifacts have SHA-256
`b9d9dcdc4105d18c1e734cd40def7e0609fc0f83c389e200566ff552c0a59882`,
`88fd001c7cb4a99d05f204b379a56745f7a2ba6e3799c332f9dbe7783f542162`,
`f49511d51fd139a87c071b432a8ff7926b2014e91fb55ed3f6195c9aefd0a044`,
and `fe5be694b046cc0076ef9f656b645c11afdb8193bde0779adb63ccf0cb66085c`,
respectively.
The lake reference JSON has SHA-256
`80ed72208e48cfde7bc6ddb0cd45b668c6aee7ec9b39460e69f154fc285c71ad`.
