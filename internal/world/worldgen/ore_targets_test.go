package worldgen

import (
	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
	"testing"
)

// TestOreResolvedTargetsPreserveOrderAndRNG checks overlapping targets at chunk boundaries.
func TestOreResolvedTargetsPreserveOrderAndRNG(t *testing.T) {
	stone, ore := block.DefaultStateID("minecraft:stone"), block.DefaultStateID("minecraft:diamond_ore")
	reference := configuredOre{size: 32, discard: 0.5, targets: []oreTarget{{states: map[int32]struct{}{stone: {}}, result: ore}, {states: map[int32]struct{}{stone: {}}, result: block.DefaultStateID("minecraft:gold_ore")}}}
	resolved := reference
	resolved.targetBase = stone
	resolved.resolvedTargets = []int32{ore}
	for _, size := range []int{1, 16, 32, 64} {
		reference.size, resolved.size = size, size
		for _, origin := range []blockPosition{{-16, -63, -16}, {-1, 0, -1}, {-8, 8, -8}} {
			for _, seed := range []int64{0, 42, -17} {
				first := section.NewChunkWithBounds(-1, -1, -64, 128, 0)
				for _, current := range first.Sections {
					current.Blocks = section.NewSinglePalette(section.BlocksPerSection, stone)
				}
				first.SetBlock(-8, 9, -8, 0)
				second := first.CloneRaw()
				a, b := worldrandom.NewXoroshiro(seed), worldrandom.NewXoroshiro(seed)
				if resolved.place(first, a, origin) != reference.placeGeometryReference(second, b, origin) {
					t.Fatal("placement result differs")
				}
				for y := first.MinY(); y <= first.MaxY(); y++ {
					for x := -16; x < 0; x++ {
						for z := -16; z < 0; z++ {
							if first.GetBlock(x, y, z) != second.GetBlock(x, y, z) {
								t.Fatalf("different ore block at %d,%d,%d", x, y, z)
							}
						}
					}
				}
				if a.NextLong() != b.NextLong() {
					t.Fatal("resolved target lookup changed RNG sequence")
				}
			}
		}
	}
}
