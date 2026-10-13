// Package density implements executable world generation density functions.
package density

import (
	"math"

	"github.com/vitismc/vitis/internal/world/worldgen/noise"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// EndIslands samples the base density that shapes outer End islands.
type EndIslands struct {
	noise *noise.Simplex
}

// NewEndIslands creates a deterministic End island density function.
func NewEndIslands(seed int64) *EndIslands {
	source := worldrandom.NewLegacy(seed)
	for range 17292 {
		source.NextInt()
	}
	return &EndIslands{noise: noise.NewSimplex(source)}
}

// Sample returns density at integer block coordinates.
func (e *EndIslands) Sample(blockX, blockZ int) float64 {
	height := endHeight(e.noise, int32(blockX/8), int32(blockZ/8))
	return (float64(height) - 8) / 128
}

func endHeight(source *noise.Simplex, x, z int32) float32 {
	cellX, cellZ := x/2, z/2
	remainderX, remainderZ := x%2, z%2
	squared := int32(uint32(x*x) + uint32(z*z))
	height := float32(100) - float32(math.Sqrt(float64(float32(squared))))*8
	height = clampFloat32(height, -100, 80)

	for offsetX := int32(-12); offsetX <= 12; offsetX++ {
		for offsetZ := int32(-12); offsetZ <= 12; offsetZ++ {
			islandX, islandZ := int64(cellX+offsetX), int64(cellZ+offsetZ)
			if islandX*islandX+islandZ*islandZ <= 4096 || source.Sample2D(float64(islandX), float64(islandZ)) >= float64(float32(-0.9)) {
				continue
			}
			sizeBase := float32(math.Abs(float64(float32(islandX))))*3439 + float32(math.Abs(float64(float32(islandZ))))*147
			size := float32(math.Mod(float64(sizeBase), 13)) + 9
			deltaX := float32(remainderX - offsetX*2)
			deltaZ := float32(remainderZ - offsetZ*2)
			candidate := float32(100) - float32(math.Sqrt(float64(deltaX*deltaX+deltaZ*deltaZ)))*size
			candidate = clampFloat32(candidate, -100, 80)
			if candidate > height {
				height = candidate
			}
		}
	}
	return height
}

func clampFloat32(value, minimum, maximum float32) float32 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
