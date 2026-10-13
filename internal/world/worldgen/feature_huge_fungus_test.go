package worldgen

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// TestHugeFungusVariants checks natural and planted fungi, complete stems, cap states, and deterministic RNG.
func TestHugeFungusVariants(t *testing.T) {
	for _, name := range []string{"crimson_fungus", "warped_fungus", "crimson_fungus_planted", "warped_fungus_planted"} {
		t.Run(name, func(t *testing.T) {
			f := loadSmallFeature(t, name).(*hugeFungusFeature)
			caps, lights, vines, wideCount := 0, 0, 0, 0
			for seed := int64(0); seed < 32; seed++ {
				first, second := section.NewChunkWithBounds(-1, -1, 0, 256, 0), section.NewChunkWithBounds(-1, -1, 0, 256, 0)
				origin := blockPosition{-8, 32, -8}
				for _, target := range []*section.Chunk{first, second} {
					target.SetBlock(origin.x, origin.y-1, origin.z, f.base)
				}
				expected := worldrandom.NewLegacy(seed)
				height := 4 + int(expected.NextIntBound(10))
				if expected.NextIntBound(12) == 0 {
					height *= 2
				}
				wide := !f.planted && expected.NextFloat() < .06
				if wide {
					wideCount++
				}
				randomA, randomB := worldrandom.NewLegacy(seed), worldrandom.NewLegacy(seed)
				if !f.placeFeature(&featureSystem{height: 128}, first, randomA, origin) || !f.placeFeature(&featureSystem{height: 128}, second, randomB, origin) {
					t.Fatalf("fungus seed %d rejected", seed)
				}
				if randomA.NextLong() != randomB.NextLong() {
					t.Fatal("nondeterministic RNG")
				}
				for dy := 0; dy < height; dy++ {
					if first.GetBlock(origin.x, origin.y+dy, origin.z) != f.stem {
						t.Fatalf("incomplete stem at height %d", dy)
					}
					if wide && first.GetBlock(origin.x+1, origin.y+dy, origin.z) != f.stem {
						t.Fatal("wide stem side missing")
					}
					if !wide && dy == 0 && first.GetBlock(origin.x+1, origin.y, origin.z) == f.stem {
						t.Fatal("unexpected wide stem")
					}
				}
				for x := -16; x < 0; x++ {
					for z := -16; z < 0; z++ {
						for y := 32; y <= 60; y++ {
							state := first.GetBlock(x, y, z)
							if state != second.GetBlock(x, y, z) {
								t.Fatalf("nondeterministic fungus at %d,%d,%d", x, y, z)
							}
							if state == f.hat {
								caps++
							}
							if state == f.decor {
								lights++
							}
							if block.NameFromState(state) == "minecraft:weeping_vines" {
								vines++
								vine, _ := block.BlockStateFromID(state)
								age, _ := vine.Property("age")
								if age != "23" && age != "24" && age != "25" {
									t.Fatal("incorrect fungus vine age")
								}
							}
						}
					}
				}
			}
			if caps == 0 || lights == 0 || f.crimson && vines == 0 || !f.crimson && vines != 0 || f.planted && wideCount != 0 {
				t.Fatalf("cap=%d lights=%d vines=%d wide=%d", caps, lights, vines, wideCount)
			}
		})
	}
}

// TestHugeFungusRejectsInvalidSites checks base validation and natural generation-depth limits.
func TestHugeFungusRejectsInvalidSites(t *testing.T) {
	f := loadSmallFeature(t, "crimson_fungus").(*hugeFungusFeature)
	for _, validBase := range []bool{false, true} {
		target := section.NewChunkWithBounds(0, 0, 0, 256, 0)
		origin := blockPosition{8, 126, 8}
		if validBase {
			target.SetBlock(8, 125, 8, f.base)
		}
		random, expected := worldrandom.NewLegacy(42), worldrandom.NewLegacy(42)
		if validBase {
			expected.NextIntBound(10)
			expected.NextIntBound(12)
		}
		if f.placeFeature(&featureSystem{height: 128}, target, random, origin) || random.NextLong() != expected.NextLong() || target.GetBlock(8, 126, 8) != 0 {
			t.Fatal("invalid fungus site modified world or RNG")
		}
	}
}

// TestHugeFungusReplacementAndBorder preserves stone and records complete caps at chunk edges.
func TestHugeFungusReplacementAndBorder(t *testing.T) {
	f := loadSmallFeature(t, "warped_fungus").(*hugeFungusFeature)
	target := section.NewChunkWithBounds(0, 0, 0, 256, 0)
	target.EnableBorderBlocks()
	origin := blockPosition{15, 32, 8}
	target.SetBlock(15, 31, 8, f.base)
	stone := block.DefaultStateID("minecraft:stone")
	target.SetBlock(15, 34, 8, stone)
	target.SetBlock(15, 33, 8, block.DefaultStateID("minecraft:warped_fungus"))
	if !f.placeFeature(&featureSystem{height: 128}, target, worldrandom.NewLegacy(42), origin) {
		t.Fatal("fungus did not generate")
	}
	if target.GetBlock(15, 34, 8) != stone || target.GetBlock(15, 33, 8) != f.stem {
		t.Fatal("stem replacement predicate ignored")
	}
	if len(target.BorderBlocks()) == 0 {
		t.Fatal("fungus cap was clipped at chunk boundary")
	}
	neighbor := section.NewChunkWithBounds(1, 0, 0, 256, 0)
	mergeTreeBorders(neighbor, target.BorderBlocks())
	capBlocks := 0
	for _, state := range target.BorderBlocks() {
		if state == f.hat {
			capBlocks++
		}
	}
	if capBlocks == 0 {
		t.Fatal("no neighboring cap blocks")
	}
}
