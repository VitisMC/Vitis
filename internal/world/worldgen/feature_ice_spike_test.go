package worldgen

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// TestIceSpikeFeaturePlacement checks snow support, replacement, and seeded output.
func TestIceSpikeFeaturePlacement(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	raw, ok := manager.Current().Get("data/minecraft/worldgen/configured_feature/ice_spike.json")
	if !ok {
		t.Fatal("missing ice spike definition")
	}
	configured, supported, err := decodeConfiguredFeature(manager.Current(), raw)
	if err != nil || !supported {
		t.Fatalf("decode ice spike: supported=%t, error=%v", supported, err)
	}
	feature, ok := configured.(*iceSpikeFeature)
	if !ok {
		t.Fatalf("ice spike compiled as %T", configured)
	}
	snow := block.DefaultStateID("minecraft:snow_block")
	stone := block.DefaultStateID("minecraft:stone")
	origin := blockPosition{x: 8, y: 84, z: 8}
	makeChunk := func(support int32) *section.Chunk {
		chunk := section.NewChunkWithBounds(0, 0, -64, 384, 0)
		chunk.SetBlock(8, 80, 8, support)
		chunk.SetBlock(8, 79, 8, stone)
		return chunk
	}
	first, second := makeChunk(snow), makeChunk(snow)
	for _, chunk := range []*section.Chunk{first, second} {
		if !feature.placeFeature(nil, chunk, worldrandom.NewLegacy(42), origin) {
			t.Fatal("ice spike did not generate on snow")
		}
	}
	packed := 0
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			for y := 50; y < 110; y++ {
				state := first.GetBlock(x, y, z)
				if state != second.GetBlock(x, y, z) {
					t.Fatalf("non-deterministic ice spike at %d,%d,%d", x, y, z)
				}
				if state == feature.packedIce {
					packed++
				}
			}
		}
	}
	if packed < 20 || first.GetBlock(8, 79, 8) != stone {
		t.Fatalf("packed ice count = %d, support beneath = %d", packed, first.GetBlock(8, 79, 8))
	}
	if feature.placeFeature(nil, makeChunk(stone), worldrandom.NewLegacy(42), origin) {
		t.Fatal("ice spike generated without snow support")
	}
}
