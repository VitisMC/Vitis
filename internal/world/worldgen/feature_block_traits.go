package worldgen

import genblock "github.com/vitismc/vitis/internal/data/generated/block"

// featureFace uses the direction order recorded by versioned generation metadata.
type featureFace uint8

const (
	featureDown featureFace = iota
	featureUp
	featureNorth
	featureSouth
	featureWest
	featureEast

	featureReplaceableTrait uint32 = 1 << 12
	featureWaterTrait       uint32 = 1 << 13
	featureWaterSourceTrait uint32 = 1 << 14
	featureFluidTrait       uint32 = 1 << 15
	featureSolidTrait       uint32 = 1 << 16
	featureLiquidTrait      uint32 = 1 << 21
)

var featureFaceNames = [...]string{"down", "up", "north", "south", "west", "east"}
var featureFaceOffsets = [...]blockPosition{{y: -1}, {y: 1}, {z: -1}, {z: 1}, {x: -1}, {x: 1}}

// parseFeatureFace validates a namespaced-independent direction property value.
func parseFeatureFace(name string) (featureFace, bool) {
	for face, value := range featureFaceNames {
		if name == value {
			return featureFace(face), true
		}
	}
	return 0, false
}

// opposite returns the face pointing away from this direction.
func (f featureFace) opposite() featureFace { return f ^ 1 }

// offset returns an adjacent world position in this direction.
func (f featureFace) offset(p blockPosition) blockPosition {
	d := featureFaceOffsets[f]
	return blockPosition{p.x + d.x, p.y + d.y, p.z + d.z}
}

// generationBlockTraits returns immutable traits or an empty mask for an invalid state.
func generationBlockTraits(state int32) uint32 {
	if state < 0 || int(state) >= len(genblock.GenerationTraits) {
		return 0
	}
	return genblock.GenerationTraits[state]
}

// featureFullFace checks support geometry and optionally empty-context collision geometry.
func featureFullFace(state int32, face featureFace, includeCollision bool) bool {
	traits := generationBlockTraits(state)
	if includeCollision {
		traits |= traits >> 6 & 63
	}
	return traits&(uint32(1)<<face) != 0
}
