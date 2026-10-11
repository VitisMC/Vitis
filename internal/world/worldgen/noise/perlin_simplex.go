package noise

import (
	"fmt"
	"math"
	"sort"

	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// PerlinSimplex combines octave-scaled simplex samplers used by biome temperature.
type PerlinSimplex struct {
	levels     []*Simplex
	inputScale float64
	valueScale float64
}

// NewPerlinSimplex creates a sampler from a set of integer octave indices.
func NewPerlinSimplex(source worldrandom.Source, octaves []int) (*PerlinSimplex, error) {
	if source == nil || len(octaves) == 0 {
		return nil, fmt.Errorf("perlin simplex requires a source and octaves")
	}
	ordered := append([]int(nil), octaves...)
	sort.Ints(ordered)
	for i := 1; i < len(ordered); i++ {
		if ordered[i] == ordered[i-1] {
			return nil, fmt.Errorf("duplicate simplex octave %d", ordered[i])
		}
	}
	minimum, maximum := ordered[0], ordered[len(ordered)-1]
	zeroIndex := -minimum
	count := zeroIndex + maximum + 1
	if count < 1 {
		return nil, fmt.Errorf("invalid simplex octave range")
	}
	present := make(map[int]bool, len(ordered))
	for _, octave := range ordered {
		present[octave] = true
	}
	base := NewSimplex(source)
	levels := make([]*Simplex, count)
	if maximum >= 0 && maximum < count && present[0] {
		levels[maximum] = base
	}
	for index := maximum + 1; index < count; index++ {
		if index >= 0 && present[maximum-index] {
			levels[index] = NewSimplex(source)
		} else {
			consume(source, 262)
		}
	}
	if maximum > 0 {
		seed := int64(base.Sample3D(base.xOffset, base.yOffset, base.zOffset) * float64(float32(9.223372e18)))
		derived := worldrandom.NewLegacy(seed)
		for index := maximum - 1; index >= 0; index-- {
			if index < count && present[maximum-index] {
				levels[index] = NewSimplex(derived)
			} else {
				consume(derived, 262)
			}
		}
	}
	return &PerlinSimplex{
		levels: levels, inputScale: math.Pow(2, float64(maximum)),
		valueScale: 1 / (math.Pow(2, float64(count)) - 1),
	}, nil
}

// Sample returns two-dimensional octave noise.
func (p *PerlinSimplex) Sample(x, y float64, useOffsets bool) float64 {
	inputScale, valueScale, result := p.inputScale, p.valueScale, 0.0
	for _, level := range p.levels {
		if level != nil {
			xOffset, yOffset := 0.0, 0.0
			if useOffsets {
				xOffset, yOffset = level.xOffset, level.yOffset
			}
			result += level.Sample2D(x*inputScale+xOffset, y*inputScale+yOffset) * valueScale
		}
		inputScale /= 2
		valueScale *= 2
	}
	return result
}
