package worldgen

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/world/chunk/section"
)

// TestLeafDistanceScratchReuse checks frontier propagation and isolation between trees.
func TestLeafDistanceScratchReuse(t *testing.T) {
	leaf := block.DefaultStateID("minecraft:oak_leaves")
	feature := &treeFeature{}
	for _, trunkX := range []int{4, 8, 4} {
		chunk := section.NewChunkWithBounds(0, 0, 0, 128, 0)
		logs := []blockPosition{{x: trunkX, y: 64, z: 8}}
		leaves := make([]blockPosition, 6)
		for distance := range leaves {
			leaves[distance] = blockPosition{x: trunkX + distance + 1, y: 64, z: 8}
			chunk.SetBlock(leaves[distance].x, 64, 8, leaf)
		}
		feature.updateLeafDistances(chunk, logs, leaves)
		for index, position := range leaves {
			want := leafDistanceStates[leaf][index+1]
			if got := chunk.GetBlock(position.x, position.y, position.z); got != want {
				t.Fatalf("trunk x=%d, leaf distance=%d: state=%d, want=%d", trunkX, index+1, got, want)
			}
		}
	}
}
