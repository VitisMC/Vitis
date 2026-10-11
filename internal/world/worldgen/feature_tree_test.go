package worldgen

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// TestTreeFeaturesGenerateLogsAndLeaves checks common canopy types and seeded output.
func TestTreeFeaturesGenerateLogsAndLeaves(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	grass := block.DefaultStateID("minecraft:grass_block")
	for _, name := range []string{"oak", "birch", "spruce", "pine", "mega_pine", "mega_spruce", "fancy_oak", "jungle_bush", "jungle_tree", "jungle_tree_no_vine", "mega_jungle_tree", "swamp_oak", "acacia", "dark_oak", "pale_oak", "cherry", "cherry_bees_005", "azalea_tree"} {
		t.Run(name, func(t *testing.T) {
			raw, ok := manager.Current().Get("data/minecraft/worldgen/configured_feature/" + name + ".json")
			if !ok {
				t.Fatal("missing tree definition")
			}
			feature, supported, err := decodeConfiguredFeature(manager.Current(), raw)
			if err != nil || !supported {
				t.Fatalf("tree decode: supported=%t, error=%v", supported, err)
			}
			makeChunk := func() *section.Chunk {
				chunk := section.NewChunkWithBounds(0, 0, -64, 384, 0)
				for x := 0; x < 16; x++ {
					for z := 0; z < 16; z++ {
						chunk.SetBlock(x, 64, z, grass)
					}
				}
				return chunk
			}
			first, second := makeChunk(), makeChunk()
			origin := blockPosition{x: 8, y: 65, z: 8}
			for _, chunk := range []*section.Chunk{first, second} {
				if !feature.placeFeature(nil, chunk, worldrandom.NewLegacy(42), origin) {
					t.Fatal("tree did not generate")
				}
			}
			logs, leaves, connectedLeaves := 0, 0, 0
			for x := 0; x < 16; x++ {
				for z := 0; z < 16; z++ {
					for y := 65; y < 105; y++ {
						state := first.GetBlock(x, y, z)
						if state != second.GetBlock(x, y, z) {
							t.Fatalf("different tree block at %d,%d,%d", x, y, z)
						}
						name := block.NameFromState(state)
						if name == "minecraft:oak_log" || name == "minecraft:birch_log" || name == "minecraft:spruce_log" || name == "minecraft:jungle_log" || name == "minecraft:acacia_log" || name == "minecraft:dark_oak_log" || name == "minecraft:pale_oak_log" || name == "minecraft:cherry_log" {
							logs++
						}
						if name == "minecraft:oak_leaves" || name == "minecraft:birch_leaves" || name == "minecraft:spruce_leaves" || name == "minecraft:jungle_leaves" || name == "minecraft:acacia_leaves" || name == "minecraft:dark_oak_leaves" || name == "minecraft:pale_oak_leaves" || name == "minecraft:cherry_leaves" || name == "minecraft:azalea_leaves" || name == "minecraft:flowering_azalea_leaves" {
							leaves++
							leaf, err := block.BlockStateFromID(state)
							if err != nil {
								t.Fatal(err)
							}
							if distance, ok := leaf.Property("distance"); ok && distance != "7" {
								connectedLeaves++
							}
						}
					}
				}
			}
			if logs < 1 || leaves < 5 || connectedLeaves < 1 {
				t.Fatalf("logs=%d leaves=%d connected=%d", logs, leaves, connectedLeaves)
			}
			soil := block.NameFromState(first.GetBlock(8, 64, 8))
			if soil != "minecraft:dirt" && soil != "minecraft:rooted_dirt" && soil != "minecraft:podzol" && soil != "minecraft:pale_moss_block" {
				t.Fatal("tree did not prepare soil")
			}
		})
	}
}

// TestMangroveTreesPlaceRootsAndCanopies checks both mangrove configurations.
func TestMangroveTreesPlaceRootsAndCanopies(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"mangrove", "tall_mangrove"} {
		t.Run(name, func(t *testing.T) {
			raw, _ := manager.Current().Get("data/minecraft/worldgen/configured_feature/" + name + ".json")
			feature, ok, err := decodeConfiguredFeature(manager.Current(), raw)
			if err != nil || !ok {
				t.Fatalf("decode: supported=%v error=%v", ok, err)
			}
			chunk := section.NewChunkWithBounds(0, 0, -64, 384, 0)
			chunk.EnableBorderBlocks()
			for x := 0; x < 16; x++ {
				for z := 0; z < 16; z++ {
					for y := 48; y < 62; y++ {
						chunk.SetBlock(x, y, z, block.DefaultStateID("minecraft:stone"))
					}
					for y := 62; y <= 64; y++ {
						chunk.SetBlock(x, y, z, block.DefaultStateID("minecraft:mud"))
					}
				}
			}
			placed := false
			for seed := int64(0); seed < 32 && !placed; seed++ {
				placed = feature.placeFeature(nil, chunk, worldrandom.NewLegacy(seed), blockPosition{x: 8, y: 65, z: 8})
			}
			if !placed {
				t.Fatal("tree never generated")
			}
			logs, leaves, roots := 0, 0, 0
			for y := 65; y < 110; y++ {
				for x := 0; x < 16; x++ {
					for z := 0; z < 16; z++ {
						switch block.NameFromState(chunk.GetBlock(x, y, z)) {
						case "minecraft:mangrove_log":
							logs++
						case "minecraft:mangrove_leaves":
							leaves++
						case "minecraft:mangrove_roots", "minecraft:muddy_mangrove_roots":
							roots++
						}
					}
				}
			}
			if logs < 2 || leaves < 5 || roots < 1 {
				t.Fatalf("logs=%d leaves=%d roots=%d", logs, leaves, roots)
			}
		})
	}
}

// TestGiantTreesHaveCompleteTrunksAndCrowns checks jungle and old-growth taiga shapes.
func TestGiantTreesHaveCompleteTrunksAndCrowns(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"mega_jungle_tree", "mega_pine", "mega_spruce"} {
		t.Run(name, func(t *testing.T) {
			raw, _ := manager.Current().Get("data/minecraft/worldgen/configured_feature/" + name + ".json")
			configured, supported, err := decodeConfiguredFeature(manager.Current(), raw)
			if err != nil || !supported {
				t.Fatalf("tree decode: supported=%t, error=%v", supported, err)
			}
			chunk := section.NewChunkWithBounds(0, 0, -64, 384, 0)
			for x := 7; x <= 8; x++ {
				for z := 7; z <= 8; z++ {
					chunk.SetBlock(x, 64, z, block.DefaultStateID("minecraft:grass_block"))
				}
			}
			if !configured.placeFeature(nil, chunk, worldrandom.NewLegacy(42), blockPosition{x: 7, y: 65, z: 7}) {
				t.Fatal("giant tree did not generate")
			}
			logName := "minecraft:spruce_log"
			leafName := "minecraft:spruce_leaves"
			if name == "mega_jungle_tree" {
				logName, leafName = "minecraft:jungle_log", "minecraft:jungle_leaves"
			}
			for _, offset := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
				if got := block.NameFromState(chunk.GetBlock(7+offset[0], 65, 7+offset[1])); got != logName {
					t.Fatalf("incomplete trunk at %v: %s", offset, got)
				}
			}
			leaves := 0
			for x := 0; x < 16; x++ {
				for z := 0; z < 16; z++ {
					for y := 65; y < 112; y++ {
						if block.NameFromState(chunk.GetBlock(x, y, z)) == leafName {
							leaves++
						}
					}
				}
			}
			if leaves < 30 {
				t.Fatalf("crown too small: %d leaves", leaves)
			}
		})
	}
}

// TestGiantTreeBiomeSelectorsCompile checks that missing variants do not disable whole biomes.
func TestGiantTreeBiomeSelectorsCompile(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"trees_jungle", "bamboo_vegetation", "trees_old_growth_pine_taiga", "trees_old_growth_spruce_taiga"} {
		t.Run(name, func(t *testing.T) {
			raw, ok := manager.Current().Get("data/minecraft/worldgen/configured_feature/" + name + ".json")
			if !ok {
				t.Fatal("missing biome tree selector")
			}
			_, supported, err := decodeConfiguredFeature(manager.Current(), raw)
			if err != nil || !supported {
				t.Fatalf("biome tree selector: supported=%t, error=%v", supported, err)
			}
		})
	}
	compiled, err := compileResources(manager.Current())
	if err != nil {
		t.Fatal(err)
	}
	placed := compiled.(*ResourceSet).features.placed
	for _, name := range []string{
		"trees_jungle", "bamboo_vegetation", "mega_jungle_tree_checked",
		"trees_old_growth_pine_taiga", "trees_old_growth_spruce_taiga",
		"mega_pine_checked", "mega_spruce_checked", "trees_sparse_jungle",
		"trees_birch", "trees_taiga", "trees_savanna", "trees_cherry",
		"trees_swamp", "trees_grove", "pale_garden_vegetation",
	} {
		if placed["minecraft:"+name] == nil {
			t.Errorf("biome tree feature %s was not compiled", name)
		}
	}
}

// TestGiantTreeRecordsAdjacentTrunk checks two by two trunks at chunk borders.
func TestGiantTreeRecordsAdjacentTrunk(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := manager.Current().Get("data/minecraft/worldgen/configured_feature/mega_jungle_tree.json")
	feature, supported, err := decodeConfiguredFeature(manager.Current(), raw)
	if err != nil || !supported {
		t.Fatalf("decode giant tree: supported=%t, error=%v", supported, err)
	}
	source := section.NewChunkWithBounds(0, 0, -64, 384, 0)
	source.EnableBorderBlocks()
	source.SetBlock(15, 64, 8, block.DefaultStateID("minecraft:grass_block"))
	if !feature.placeFeature(nil, source, worldrandom.NewLegacy(42), blockPosition{x: 15, y: 65, z: 8}) {
		t.Fatal("giant tree did not grow at the border")
	}
	neighbor := section.NewChunkWithBounds(1, 0, -64, 384, 0)
	mergeTreeBorders(neighbor, source.BorderBlocks())
	for z := 8; z <= 9; z++ {
		if got := block.NameFromState(neighbor.GetBlock(16, 65, z)); got != "minecraft:jungle_log" {
			t.Fatalf("missing adjacent trunk at z=%d: %s", z, got)
		}
	}
}

// TestJungleTreeDecoratorsPlaceVines checks the foliage and trunk details.
func TestJungleTreeDecoratorsPlaceVines(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"jungle_tree", "mega_jungle_tree"} {
		t.Run(name, func(t *testing.T) {
			raw, _ := manager.Current().Get("data/minecraft/worldgen/configured_feature/" + name + ".json")
			feature, supported, err := decodeConfiguredFeature(manager.Current(), raw)
			if err != nil || !supported {
				t.Fatalf("decode jungle tree: supported=%t, error=%v", supported, err)
			}
			chunk := section.NewChunkWithBounds(0, 0, -64, 384, 0)
			for x := 7; x <= 8; x++ {
				for z := 7; z <= 8; z++ {
					chunk.SetBlock(x, 64, z, block.DefaultStateID("minecraft:grass_block"))
				}
			}
			if !feature.placeFeature(nil, chunk, worldrandom.NewLegacy(42), blockPosition{x: 7, y: 65, z: 7}) {
				t.Fatal("jungle tree did not generate")
			}
			vines := 0
			for x := 0; x < 16; x++ {
				for z := 0; z < 16; z++ {
					for y := 65; y < 110; y++ {
						if block.NameFromState(chunk.GetBlock(x, y, z)) == "minecraft:vine" {
							vines++
						}
					}
				}
			}
			if vines == 0 {
				t.Fatal("jungle tree has no vines")
			}
		})
	}
}

// TestTreeFeatureRejectsObstructedTrunk checks collision before soil mutation.
func TestTreeFeatureRejectsObstructedTrunk(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := manager.Current().Get("data/minecraft/worldgen/configured_feature/oak.json")
	feature, supported, err := decodeConfiguredFeature(manager.Current(), raw)
	if err != nil || !supported {
		t.Fatalf("decode tree: %v", err)
	}
	chunk := section.NewChunkWithBounds(0, 0, -64, 384, 0)
	grass := block.DefaultStateID("minecraft:grass_block")
	stone := block.DefaultStateID("minecraft:stone")
	chunk.SetBlock(8, 64, 8, grass)
	chunk.SetBlock(8, 67, 8, stone)
	if feature.placeFeature(nil, chunk, worldrandom.NewLegacy(42), blockPosition{x: 8, y: 65, z: 8}) {
		t.Fatal("tree grew through stone")
	}
	if chunk.GetBlock(8, 64, 8) != grass {
		t.Fatal("failed tree changed soil")
	}
}

// TestTreeFeatureRejectsExistingTreeBase prevents trunks from growing on or inside trees.
func TestTreeFeatureRejectsExistingTreeBase(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := manager.Current().Get("data/minecraft/worldgen/configured_feature/oak.json")
	configured, supported, err := decodeConfiguredFeature(manager.Current(), raw)
	if err != nil || !supported {
		t.Fatalf("decode tree: %v", err)
	}
	feature := configured.(*treeFeature)
	for _, test := range []struct {
		name string
		base string
		atY  int
	}{
		{"on leaves", "minecraft:oak_leaves", 64},
		{"on log", "minecraft:oak_log", 64},
		{"inside leaves", "minecraft:oak_leaves", 65},
		{"inside log", "minecraft:oak_log", 65},
		{"through leaves", "minecraft:oak_leaves", 68},
		{"through log", "minecraft:oak_log", 68},
	} {
		t.Run(test.name, func(t *testing.T) {
			chunk := section.NewChunkWithBounds(0, 0, -64, 384, 0)
			state := block.DefaultStateID(test.base)
			chunk.SetBlock(8, test.atY, 8, state)
			if feature.placeFeature(nil, chunk, worldrandom.NewLegacy(42), blockPosition{x: 8, y: 65, z: 8}) {
				t.Fatal("tree grew on an existing tree")
			}
			if got := chunk.GetBlock(8, test.atY, 8); got != state {
				t.Fatalf("existing tree block changed: %d", got)
			}
		})
	}
}

// TestDarkOakRejectsTreeInBaseFootprint checks all four trunk columns.
func TestDarkOakRejectsTreeInBaseFootprint(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := manager.Current().Get("data/minecraft/worldgen/configured_feature/dark_oak.json")
	feature, supported, err := decodeConfiguredFeature(manager.Current(), raw)
	if err != nil || !supported {
		t.Fatalf("decode tree: %v", err)
	}
	chunk := section.NewChunkWithBounds(0, 0, -64, 384, 0)
	grass := block.DefaultStateID("minecraft:grass_block")
	for x := 8; x <= 9; x++ {
		for z := 8; z <= 9; z++ {
			chunk.SetBlock(x, 64, z, grass)
		}
	}
	leaves := block.DefaultStateID("minecraft:oak_leaves")
	chunk.SetBlock(9, 64, 9, leaves)
	if feature.placeFeature(nil, chunk, worldrandom.NewLegacy(42), blockPosition{x: 8, y: 65, z: 8}) {
		t.Fatal("dark oak grew on another tree")
	}
	if chunk.GetBlock(8, 64, 8) != grass || chunk.GetBlock(9, 64, 9) != leaves {
		t.Fatal("rejected dark oak changed its base")
	}
}

// TestTreeFeatureRecordsAdjacentCanopy checks that chunk borders do not trim trees.
func TestTreeFeatureRecordsAdjacentCanopy(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := manager.Current().Get("data/minecraft/worldgen/configured_feature/oak.json")
	feature, supported, err := decodeConfiguredFeature(manager.Current(), raw)
	if err != nil || !supported {
		t.Fatalf("decode tree: %v", err)
	}
	source := section.NewChunkWithBounds(0, 0, -64, 384, 0)
	source.EnableBorderBlocks()
	source.SetBlock(15, 64, 8, block.DefaultStateID("minecraft:grass_block"))
	if !feature.placeFeature(nil, source, worldrandom.NewLegacy(42), blockPosition{x: 15, y: 65, z: 8}) {
		t.Fatal("edge tree did not generate")
	}
	border := source.BorderBlocks()
	if len(border) == 0 {
		t.Fatal("tree canopy was clipped at the chunk border")
	}
	neighbor := section.NewChunkWithBounds(1, 0, -64, 384, 0)
	var blocked section.BorderPosition
	blockedFound := false
	for position := range border {
		if position.X >= 16 && position.X < 32 && position.Z >= 0 && position.Z < 16 {
			blocked = position
			blockedFound = true
			break
		}
	}
	if !blockedFound {
		t.Fatal("tree produced no blocks in the adjacent chunk")
	}
	stone := block.DefaultStateID("minecraft:stone")
	neighbor.SetBlock(blocked.X, blocked.Y, blocked.Z, stone)
	mergeTreeBorders(neighbor, border)
	if got := neighbor.GetBlock(blocked.X, blocked.Y, blocked.Z); got != stone {
		t.Fatalf("tree replaced neighboring terrain: %d", got)
	}
	found := false
	for position, state := range border {
		if position != blocked && position.X >= 16 && position.X < 32 && position.Z >= 0 && position.Z < 16 && neighbor.GetBlock(position.X, position.Y, position.Z) == state {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("neighbor did not receive tree canopy")
	}
}

// TestDarkOakRecordsAdjacentTrunk checks a two-wide trunk at the border.
func TestDarkOakRecordsAdjacentTrunk(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := manager.Current().Get("data/minecraft/worldgen/configured_feature/dark_oak.json")
	feature, supported, err := decodeConfiguredFeature(manager.Current(), raw)
	if err != nil || !supported {
		t.Fatalf("decode dark oak: %v", err)
	}
	source := section.NewChunkWithBounds(0, 0, -64, 384, 0)
	source.EnableBorderBlocks()
	source.SetBlock(15, 64, 8, block.DefaultStateID("minecraft:grass_block"))
	if !feature.placeFeature(nil, source, worldrandom.NewLegacy(42), blockPosition{x: 15, y: 65, z: 8}) {
		t.Fatal("dark oak did not generate")
	}
	neighbor := section.NewChunkWithBounds(1, 0, -64, 384, 0)
	mergeTreeBorders(neighbor, source.BorderBlocks())
	if name := block.NameFromState(neighbor.GetBlock(16, 65, 8)); name != "minecraft:dark_oak_log" {
		t.Fatalf("adjacent trunk is %s", name)
	}
}

// TestTreeFeatureRecordsDiagonalCanopy checks trees at a chunk corner.
func TestTreeFeatureRecordsDiagonalCanopy(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := manager.Current().Get("data/minecraft/worldgen/configured_feature/oak.json")
	feature, supported, err := decodeConfiguredFeature(manager.Current(), raw)
	if err != nil || !supported {
		t.Fatalf("decode tree: %v", err)
	}
	source := section.NewChunkWithBounds(0, 0, -64, 384, 0)
	source.EnableBorderBlocks()
	source.SetBlock(15, 64, 15, block.DefaultStateID("minecraft:grass_block"))
	if !feature.placeFeature(nil, source, worldrandom.NewLegacy(42), blockPosition{x: 15, y: 65, z: 15}) {
		t.Fatal("corner tree did not generate")
	}
	neighbor := section.NewChunkWithBounds(1, 1, -64, 384, 0)
	mergeTreeBorders(neighbor, source.BorderBlocks())
	for position, state := range source.BorderBlocks() {
		if position.X >= 16 && position.Z >= 16 && neighbor.GetBlock(position.X, position.Y, position.Z) == state {
			return
		}
	}
	t.Fatal("diagonal canopy was clipped")
}

// TestCherryTreeConfigRejectsInvalidRanges checks malformed datapack branch settings.
func TestCherryTreeConfigRejectsInvalidRanges(t *testing.T) {
	count := []byte(`1`)
	horizontal := []byte(`2`)
	end := []byte(`0`)
	for _, start := range []string{`{"min_inclusive":-3,"max_inclusive":-3}`, `{"min_inclusive":-17,"max_inclusive":-2}`, `{"min_inclusive":-4,"max_inclusive":1}`} {
		if _, err := decodeCherryTreeConfig(count, horizontal, []byte(start), end, 0, 0, 0, 0); err == nil {
			t.Fatalf("accepted invalid cherry start %s", start)
		}
	}
	if _, err := decodeCherryTreeConfig(count, horizontal, []byte(`{"min_inclusive":-4,"max_inclusive":-3}`), end, 0, 0, 1.1, 0); err == nil {
		t.Fatal("accepted invalid leaf chance")
	}
}

// TestOverworldGeneratesTrees checks that biome placements reach terrain generation.
func TestOverworldGeneratesTrees(t *testing.T) {
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	generator, err := runtime.NewTerrainGenerator("minecraft:overworld", 123456789, 0)
	if err != nil {
		t.Fatal(err)
	}
	for chunkX := int32(-3); chunkX <= 3; chunkX++ {
		for chunkZ := int32(-3); chunkZ <= 3; chunkZ++ {
			chunk, err := generator.Generate(chunkX, chunkZ)
			if err != nil {
				t.Fatal(err)
			}
			for x := 0; x < 16; x++ {
				for z := 0; z < 16; z++ {
					for y := 50; y < 160; y++ {
						name := block.NameFromState(chunk.GetBlock(x, y, z))
						if name == "minecraft:oak_log" || name == "minecraft:birch_log" || name == "minecraft:spruce_log" {
							return
						}
					}
				}
			}
		}
	}
	t.Fatal("no tree logs in sampled Overworld chunks")
}
