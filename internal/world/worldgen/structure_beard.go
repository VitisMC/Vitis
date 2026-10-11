package worldgen

import (
	"fmt"
	"math"
)

// terrainAdjustment identifies the versioned structure density transformation.
type terrainAdjustment uint8

const (
	terrainAdjustmentNone terrainAdjustment = iota
	terrainAdjustmentBury
	terrainAdjustmentThin
	terrainAdjustmentBox
	terrainAdjustmentEncapsulate
)

// decodeTerrainAdjustment validates the resource's terrain deformation mode.
func decodeTerrainAdjustment(name string) (terrainAdjustment, error) {
	if name == "" {
		return terrainAdjustmentNone, nil
	}
	for index, value := range [...]string{"none", "bury", "beard_thin", "beard_box", "encapsulate"} {
		if name == value {
			return terrainAdjustment(index), nil
		}
	}
	return 0, fmt.Errorf("unknown terrain adaptation %q", name)
}

// structureRigid retains immutable piece geometry and its ground-level offset.
type structureRigid struct {
	box         structureBox
	adjustment  terrainAdjustment
	groundDelta int
}

// structureJunction describes a rigid-to-terrain jigsaw connection's density contribution.
type structureJunction struct{ source blockPosition }

// structureDensity evaluates piece/junction contributions in their stable order after interpolation.
type structureDensity struct {
	pieces     []structureRigid
	junctions  []structureJunction
	minY, maxY int
	bounded    bool
}

// includeY extends the conservative vertical range of possible density changes.
func (s *structureDensity) includeY(low, high int) {
	if !s.bounded {
		s.minY, s.maxY, s.bounded = low, high, true
		return
	}
	s.minY = min(s.minY, low)
	s.maxY = max(s.maxY, high)
}

// densityForStructures collects terrain-adjusting pieces referenced by one chunk.
func densityForStructures(starts []*structureStartPlan) structureDensity {
	var result structureDensity
	for _, start := range starts {
		if start.adjustment == terrainAdjustmentNone {
			continue
		}
		for _, piece := range start.pieces {
			result.pieces = append(result.pieces, structureRigid{box: piece.box, adjustment: start.adjustment})
			switch start.adjustment {
			case terrainAdjustmentBury, terrainAdjustmentThin:
				result.includeY(piece.box.low.y-12, piece.box.low.y+11)
			case terrainAdjustmentBox:
				result.includeY(piece.box.low.y-11, piece.box.high.y+11)
			case terrainAdjustmentEncapsulate:
				result.includeY(piece.box.low.y-11, piece.box.high.y+11)
			}
		}
	}
	return result
}

// sample preserves the versioned arithmetic and the independent vertical-distance conventions.
func (s structureDensity) sample(x, y, z int) float64 {
	if s.bounded && (y < s.minY || y > s.maxY) {
		return 0
	}
	value := 0.0
	for _, piece := range s.pieces {
		box := piece.box
		dx, dz := max(0, max(box.low.x-x, x-box.high.x)), max(0, max(box.low.z-z, z-box.high.z))
		ground := box.low.y + piece.groundDelta
		relative := y - ground
		vertical := relative
		switch piece.adjustment {
		case terrainAdjustmentNone:
			continue
		case terrainAdjustmentBox:
			vertical = max(0, max(ground-y, y-box.high.y))
		case terrainAdjustmentEncapsulate:
			vertical = max(0, max(box.low.y-y, y-box.high.y))
		}
		switch piece.adjustment {
		case terrainAdjustmentBury:
			value += structureBurial(float64(dx), float64(vertical)/2, float64(dz))
		case terrainAdjustmentThin, terrainAdjustmentBox:
			value += structureBeard(dx, vertical, dz, relative) * 0.8
		case terrainAdjustmentEncapsulate:
			value += structureBurial(float64(dx)/2, float64(vertical)/2, float64(dz)/2) * 0.8
		}
	}
	for _, junction := range s.junctions {
		p := junction.source
		value += structureBeard(x-p.x, y-p.y, z-p.z, y-p.y) * 0.4
	}
	return value
}

// structureBurial maps Euclidean distance into the bounded six-block burial contribution.
func structureBurial(x, y, z float64) float64 {
	distance := math.Sqrt(x*x + y*y + z*z)
	if distance >= 6 {
		return 0
	}
	if distance <= 0 {
		return 1
	}
	return 1 + (distance/6)*(0-1)
}

// structureBeard combines the checked-in vanilla float kernel with its vertical weighting.
func structureBeard(x, y, z, relativeY int) float64 {
	kx, ky, kz := x+12, y+12, z+12
	if kx < 0 || kx >= 24 || ky < 0 || ky >= 24 || kz < 0 || kz >= 24 {
		return 0
	}
	dy := float64(relativeY) + 0.5
	square := float64(x)*float64(x) + dy*dy + float64(z)*float64(z)
	weight := -dy * structureInverseSqrt(square/2) / 2
	return weight * float64(math.Float32frombits(structureBeardKernel[kz*24*24+kx*24+ky]))
}

// structureInverseSqrt reproduces the generation algorithm's versioned double-bit Newton step.
func structureInverseSqrt(value float64) float64 {
	half := 0.5 * value
	estimate := math.Float64frombits(6910469410427058090 - (math.Float64bits(value) >> 1))
	return estimate * (1.5 - half*estimate*estimate)
}
