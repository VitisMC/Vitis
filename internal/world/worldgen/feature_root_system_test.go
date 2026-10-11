package worldgen

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// TestRootSystemGrowsAzalea checks nested tree placement and deterministic roots.
func TestRootSystemGrowsAzalea(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	raw, ok := manager.Current().Get("data/minecraft/worldgen/configured_feature/rooted_azalea_tree.json")
	if !ok {
		t.Fatal("missing root system")
	}
	feature, supported, err := decodeConfiguredFeature(manager.Current(), raw)
	if err != nil || !supported {
		t.Fatalf("root system decode: supported=%t err=%v", supported, err)
	}
	makeChunk := func() *section.Chunk {
		chunk := section.NewChunkWithBounds(0, 0, -64, 384, 0)
		stone := block.DefaultStateID("minecraft:stone")
		for x := 0; x < 16; x++ {
			for z := 0; z < 16; z++ {
				for y := 61; y <= 64; y++ {
					chunk.SetBlock(x, y, z, stone)
				}
				chunk.SetBlock(x, 65, z, block.DefaultStateID("minecraft:dirt"))
			}
		}
		return chunk
	}
	first, second := makeChunk(), makeChunk()
	origin := blockPosition{x: 8, y: 60, z: 8}
	system := &featureSystem{}
	for _, chunk := range []*section.Chunk{first, second} {
		if !feature.placeFeature(system, chunk, worldrandom.NewLegacy(42), origin) {
			t.Fatal("root system rejected cave start")
		}
	}
	logs, roots, columnRoots := 0, 0, 0
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			for y := 55; y < 90; y++ {
				state := first.GetBlock(x, y, z)
				if state != second.GetBlock(x, y, z) {
					t.Fatalf("nondeterministic root block at %d,%d,%d", x, y, z)
				}
				switch block.NameFromState(state) {
				case "minecraft:oak_log":
					logs++
				case "minecraft:rooted_dirt", "minecraft:hanging_roots":
					roots++
					if y < 65 && block.NameFromState(state) == "minecraft:rooted_dirt" {
						columnRoots++
					}
				}
			}
		}
	}
	if logs == 0 || roots == 0 || columnRoots == 0 {
		t.Fatalf("logs=%d roots=%d columnRoots=%d", logs, roots, columnRoots)
	}
	blocked := makeChunk()
	blocked.SetBlock(8, 60, 8, block.DefaultStateID("minecraft:stone"))
	if feature.placeFeature(system, blocked, worldrandom.NewLegacy(42), origin) {
		t.Fatal("root system accepted solid origin")
	}
}
