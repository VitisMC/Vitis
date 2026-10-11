package worldgen

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// TestMossPatchReplacesCaveGround checks floor and ceiling patch placement.
func TestMossPatchReplacesCaveGround(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"moss_patch", "moss_patch_ceiling"} {
		t.Run(name, func(t *testing.T) {
			raw, _ := manager.Current().Get("data/minecraft/worldgen/configured_feature/" + name + ".json")
			feature, ok, err := decodeConfiguredFeature(manager.Current(), raw)
			if err != nil || !ok {
				t.Fatalf("decode: supported=%v error=%v", ok, err)
			}
			chunk := section.NewChunkWithBounds(0, 0, -64, 384, 0)
			groundY := 64
			originY := 65
			if name == "moss_patch_ceiling" {
				groundY = 68
				originY = 67
			}
			for x := 0; x < 16; x++ {
				for z := 0; z < 16; z++ {
					chunk.SetBlock(x, groundY, z, block.DefaultStateID("minecraft:stone"))
				}
			}
			if !feature.placeFeature(&featureSystem{}, chunk, worldrandom.NewLegacy(4), blockPosition{x: 8, y: originY, z: 8}) {
				t.Fatal("moss patch did not generate")
			}
			moss := 0
			for x := 0; x < 16; x++ {
				for z := 0; z < 16; z++ {
					if block.NameFromState(chunk.GetBlock(x, groundY, z)) == "minecraft:moss_block" {
						moss++
					}
				}
			}
			if moss < 4 {
				t.Fatalf("moss blocks=%d", moss)
			}
		})
	}
}

// TestCoralShapesGrowUnderwater checks all coral variants on a water floor.
func TestCoralShapesGrowUnderwater(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"minecraft:coral_tree", "minecraft:coral_claw", "minecraft:coral_mushroom"} {
		t.Run(kind, func(t *testing.T) {
			feature, ok, err := decodeCoralFeature(manager.Current(), kind)
			if err != nil || !ok {
				t.Fatalf("decode: supported=%v error=%v", ok, err)
			}
			chunk := section.NewChunkWithBounds(0, 0, -64, 384, 0)
			for x := 0; x < 16; x++ {
				for z := 0; z < 16; z++ {
					for y := 64; y <= 75; y++ {
						chunk.SetBlock(x, y, z, block.DefaultStateID("minecraft:water"))
					}
				}
			}
			if !feature.placeFeature(nil, chunk, worldrandom.NewLegacy(4), blockPosition{x: 7, y: 64, z: 7}) {
				t.Fatal("coral did not generate")
			}
			coral := 0
			for x := 4; x <= 12; x++ {
				for z := 4; z <= 12; z++ {
					for y := 64; y <= 72; y++ {
						if isCoralState(chunk.GetBlock(x, y, z)) {
							coral++
						}
					}
				}
			}
			if coral < 1 {
				t.Fatal("missing coral blocks")
			}
		})
	}
}
