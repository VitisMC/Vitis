package worldgen

import (
	"math"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/world/chunk/section"
)

// referenceOreSurfaceCheck preserves the original heightmap scan and outside-region acceptance.
func referenceOreSurfaceCheck(chunk *section.Chunk, origin blockPosition, size int) bool {
	radius := float32(size) / 8
	margin := int(math.Ceil(float64((float32(size)/16*2 + 1) / 2)))
	startX, startZ := origin.x-int(math.Ceil(float64(radius)))-margin, origin.z-int(math.Ceil(float64(radius)))-margin
	startY := origin.y - 2 - margin
	width := 2 * (int(math.Ceil(float64(radius))) + margin)
	baseX, baseZ := int(chunk.X)*16, int(chunk.Z)*16
	outside := false
	for x := startX; x <= startX+width; x++ {
		for z := startZ; z <= startZ+width; z++ {
			if x < baseX || x >= baseX+16 || z < baseZ || z >= baseZ+16 {
				outside = true
				continue
			}
			if height, _ := chunk.Heightmap(x, z, section.HeightmapOceanFloor); startY <= height {
				return true
			}
		}
	}
	return outside
}

// TestOreSurfaceCheckMatchesScan verifies bounded and crossing footprints at negative coordinates.
func TestOreSurfaceCheckMatchesScan(t *testing.T) {
	chunk := section.NewChunkWithBounds(-2, 3, -64, 384, 0)
	stone := block.DefaultStateID("minecraft:stone")
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			chunk.SetBlock(x, (x*7+z*3)%128, z, stone)
		}
	}
	chunk.EnableHeightmapCache()
	for _, size := range []int{1, 16, 32, 64} {
		for x := -35; x <= -13; x += 2 {
			for z := 45; z <= 67; z += 2 {
				for y := -80; y <= 340; y += 71 {
					origin := blockPosition{x, y, z}
					if got, want := oreMayIntersectSurface(chunk, origin, size), referenceOreSurfaceCheck(chunk, origin, size); got != want {
						t.Fatalf("size=%d origin=%v: %v != %v", size, origin, got, want)
					}
				}
			}
		}
	}
}
