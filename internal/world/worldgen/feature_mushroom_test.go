package worldgen

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// TestHugeMushroomFeatures checks that both variants grow a stem and cap.
func TestHugeMushroomFeatures(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"huge_brown_mushroom", "huge_red_mushroom"} {
		t.Run(name, func(t *testing.T) {
			raw, _ := manager.Current().Get("data/minecraft/worldgen/configured_feature/" + name + ".json")
			feature, ok, err := decodeConfiguredFeature(manager.Current(), raw)
			if err != nil || !ok {
				t.Fatalf("decode: supported=%v error=%v", ok, err)
			}
			chunk := section.NewChunkWithBounds(0, 0, -64, 384, 0)
			chunk.SetBlock(8, 64, 8, block.DefaultStateID("minecraft:mycelium"))
			if !feature.placeFeature(nil, chunk, worldrandom.NewLegacy(7), blockPosition{x: 8, y: 65, z: 8}) {
				t.Fatal("mushroom did not grow")
			}
			if block.NameFromState(chunk.GetBlock(8, 65, 8)) != "minecraft:mushroom_stem" {
				t.Fatal("missing stem")
			}
			cap := "minecraft:brown_mushroom_block"
			if name == "huge_red_mushroom" {
				cap = "minecraft:red_mushroom_block"
			}
			found := false
			for y := 69; y < 80; y++ {
				if block.NameFromState(chunk.GetBlock(9, y, 8)) == cap {
					found = true
				}
			}
			if !found {
				t.Fatal("missing cap")
			}
		})
	}
}
