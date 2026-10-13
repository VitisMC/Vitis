package worldgen

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// newClayPoolSite supplies enough stone below the entire lush-cave patch.
func newClayPoolSite() *section.Chunk {
	target := section.NewChunkWithBounds(0, 0, -64, 384, 0)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			for y := 59; y <= 64; y++ {
				target.SetBlock(x, y, z, block.DefaultStateID("minecraft:stone"))
			}
		}
	}
	return target
}

// TestWaterloggedPatchCreatesContainedClayPool checks the ground pass and sealed water selection.
func TestWaterloggedPatchCreatesContainedClayPool(t *testing.T) {
	f := loadSmallFeature(t, "clay_pool_with_dripleaves").(*vegetationPatchFeature)
	f.vegetationChance = 0
	target := newClayPoolSite()
	if !f.placeFeature(nil, target, worldrandom.NewLegacy(42), blockPosition{8, 65, 8}) {
		t.Fatal("clay pool did not generate")
	}
	water, clay := 0, 0
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			if block.IsWater(target.GetBlock(x, 64, z)) {
				water++
				if x == 0 || x == 15 || z == 0 || z == 15 || block.NameFromState(target.GetBlock(x, 63, z)) != "minecraft:clay" {
					t.Fatal("pool leaks or lost clay floor")
				}
			}
			for y := 61; y <= 64; y++ {
				if block.NameFromState(target.GetBlock(x, y, z)) == "minecraft:clay" {
					clay++
				}
			}
		}
	}
	if water < 8 || clay < water*2 {
		t.Fatalf("water=%d clay=%d", water, clay)
	}
}

// TestPatchGroundExposureRejectsOpenSidesAndFloors exercises all five pool support directions.
func TestPatchGroundExposureRejectsOpenSidesAndFloors(t *testing.T) {
	center := blockPosition{8, 64, 8}
	for _, offset := range [...][3]int{{0, 0, -1}, {1, 0, 0}, {0, 0, 1}, {-1, 0, 0}, {0, -1, 0}} {
		target := newClayPoolSite()
		if patchGroundExposed(target, center) {
			t.Fatal("fully enclosed site is exposed")
		}
		target.SetBlock(center.x+offset[0], center.y+offset[1], center.z+offset[2], block.DefaultStateID("minecraft:cave_air"))
		if !patchGroundExposed(target, center) {
			t.Fatalf("exposure ignored for %v", offset)
		}
	}
}

// recordingPoolVegetation records the nested origin and returns a waterloggable block.
type recordingPoolVegetation struct{ positions []blockPosition }

// placeFeature substitutes a waterloggable state to verify pool postprocessing.
func (f *recordingPoolVegetation) placeFeature(_ *featureSystem, target *section.Chunk, _ worldrandom.Source, p blockPosition) bool {
	f.positions = append(f.positions, p)
	setChunkBlock(target, p, block.StateID("minecraft:oak_slab", map[string]string{"waterlogged": "false"}))
	return true
}

// TestWaterloggedPatchVegetationOriginAndWaterlogging checks submerged origins and state updates.
func TestWaterloggedPatchVegetationOriginAndWaterlogging(t *testing.T) {
	f := loadSmallFeature(t, "clay_pool_with_dripleaves").(*vegetationPatchFeature)
	vegetation := &recordingPoolVegetation{}
	f.vegetationChance = 1
	f.vegetation = placedFeatureReference{inline: &placedFeature{feature: vegetation}}
	target := newClayPoolSite()
	if !f.placeFeature(nil, target, worldrandom.NewLegacy(42), blockPosition{8, 65, 8}) || len(vegetation.positions) == 0 {
		t.Fatal("nested pool vegetation did not run")
	}
	for _, p := range vegetation.positions {
		if p.y != 64 {
			t.Fatalf("pool vegetation at %v is above the water", p)
		}
		state, err := block.BlockStateFromID(getChunkBlock(target, p))
		if err != nil {
			t.Fatal(err)
		}
		if wet, _ := state.Property("waterlogged"); wet != "true" {
			t.Fatal("pool vegetation not waterlogged")
		}
	}
}
