package worldgen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/vitismc/vitis/internal/block"
)

// templateRotation follows the stable vanilla quarter-turn enumeration order.
type templateRotation uint8

const (
	templateRotationNone templateRotation = iota
	templateClockwise90
	templateClockwise180
	templateCounterclockwise90
)

// templateMirror names the two horizontal reflection planes.
type templateMirror uint8

const (
	templateMirrorNone templateMirror = iota
	templateMirrorLeftRight
	templateMirrorFrontBack
)

// templateTransform mirrors around the origin before rotating around the configured pivot.
type templateTransform struct {
	rotation templateRotation
	mirror   templateMirror
	pivot    blockPosition
}

// position transforms integer block coordinates without shifting the mirror plane to the pivot.
func (t templateTransform) position(p blockPosition) blockPosition {
	if t.mirror == templateMirrorLeftRight {
		p.z = -p.z
	}
	if t.mirror == templateMirrorFrontBack {
		p.x = -p.x
	}
	x, z := p.x, p.z
	switch t.rotation {
	case templateClockwise90:
		p.x, p.z = t.pivot.x+t.pivot.z-z, t.pivot.z-t.pivot.x+x
	case templateClockwise180:
		p.x, p.z = 2*t.pivot.x-x, 2*t.pivot.z-z
	case templateCounterclockwise90:
		p.x, p.z = t.pivot.x-t.pivot.z+z, t.pivot.x+t.pivot.z-x
	}
	return p
}

// entityPosition reflects continuous coordinates around the centers of template blocks.
func (t templateTransform) entityPosition(p [3]float64) [3]float64 {
	if t.mirror == templateMirrorLeftRight {
		p[2] = 1 - p[2]
	}
	if t.mirror == templateMirrorFrontBack {
		p[0] = 1 - p[0]
	}
	x, z, px, pz := p[0], p[2], float64(t.pivot.x), float64(t.pivot.z)
	switch t.rotation {
	case templateClockwise90:
		p[0], p[2] = px+pz+1-z, pz-px+x
	case templateClockwise180:
		p[0], p[2] = 2*px+1-x, 2*pz+1-z
	case templateCounterclockwise90:
		p[0], p[2] = px-pz+z, px+pz+1-x
	}
	return p
}

// rotatedSize returns the rotated positive footprint of a template.
func (t *structureTemplate) rotatedSize(rotation templateRotation) blockPosition {
	size := t.size
	if rotation == templateClockwise90 || rotation == templateCounterclockwise90 {
		size.x, size.z = size.z, size.x
	}
	return size
}

// anchoredOrigin keeps the transformed footprint at the requested minimum X/Z position.
func (t *structureTemplate) anchoredOrigin(minimum blockPosition, transform templateTransform) blockPosition {
	a, b := transform.position(blockPosition{}), transform.position(blockPosition{t.size.x - 1, t.size.y - 1, t.size.z - 1})
	return blockPosition{minimum.x - min(a.x, b.x), minimum.y - min(a.y, b.y), minimum.z - min(a.z, b.z)}
}

// bounds computes the inclusive world-space bounds of a transformed template.
func (t *structureTemplate) bounds(origin blockPosition, transform templateTransform) (blockPosition, blockPosition) {
	a, b := transform.position(blockPosition{}), transform.position(blockPosition{t.size.x - 1, t.size.y - 1, t.size.z - 1})
	return blockPosition{origin.x + min(a.x, b.x), origin.y + min(a.y, b.y), origin.z + min(a.z, b.z)}, blockPosition{origin.x + max(a.x, b.x), origin.y + max(a.y, b.y), origin.z + max(a.z, b.z)}
}

// direction applies horizontal reflection and quarter turns while preserving vertical directions.
func (t templateTransform) direction(direction string) string {
	names := [...]string{"north", "east", "south", "west"}
	for index, name := range names {
		if name != direction {
			continue
		}
		if t.mirror == templateMirrorLeftRight && index%2 == 0 || t.mirror == templateMirrorFrontBack && index%2 == 1 {
			index = (index + 2) & 3
		}
		return names[(index+int(t.rotation))&3]
	}
	return direction
}

// state preserves directional connections, rail geometry, and block-specific mirror semantics.
func (t templateTransform) state(id int32) (int32, error) {
	if t.rotation == templateRotationNone && t.mirror == templateMirrorNone {
		return id, nil
	}
	state, err := block.BlockStateFromID(id)
	if err != nil {
		return 0, err
	}
	properties := state.Properties()
	if facing, ok := properties["facing"]; ok {
		properties["facing"] = t.direction(facing)
	}
	if axis := properties["axis"]; axis != "y" && (t.rotation == templateClockwise90 || t.rotation == templateCounterclockwise90) {
		if axis == "x" {
			properties["axis"] = "z"
		} else if axis == "z" {
			properties["axis"] = "x"
		}
	}
	if rotation, ok := properties["rotation"]; ok {
		segment, err := strconv.Atoi(rotation)
		if err != nil {
			return 0, fmt.Errorf("invalid rotation property %s", rotation)
		}
		if t.mirror == templateMirrorLeftRight {
			segment = 8 - segment
		}
		if t.mirror == templateMirrorFrontBack {
			segment = -segment
		}
		properties["rotation"] = strconv.Itoa((segment + int(t.rotation)*4) & 15)
	}
	for _, direction := range [...]string{"north", "east", "south", "west"} {
		if value, ok := state.Property(direction); ok {
			properties[t.direction(direction)] = value
		}
	}
	if orientation, ok := properties["orientation"]; ok {
		parts := strings.Split(orientation, "_")
		if len(parts) != 2 {
			return 0, fmt.Errorf("invalid template orientation %s", orientation)
		}
		properties["orientation"] = t.direction(parts[0]) + "_" + t.direction(parts[1])
	}
	if t.mirror != templateMirrorNone && strings.HasSuffix(state.Name(), "_door") {
		if properties["hinge"] == "left" {
			properties["hinge"] = "right"
		} else {
			properties["hinge"] = "left"
		}
	}
	if shape, ok := properties["shape"]; ok {
		if strings.HasSuffix(state.Name(), "_stairs") {
			facing, _ := state.Property("facing")
			if t.mirror == templateMirrorLeftRight && (facing == "north" || facing == "south") || t.mirror == templateMirrorFrontBack && (facing == "east" || facing == "west") && strings.HasPrefix(shape, "outer_") {
				if strings.HasSuffix(shape, "_left") {
					properties["shape"] = strings.TrimSuffix(shape, "_left") + "_right"
				} else if strings.HasSuffix(shape, "_right") {
					properties["shape"] = strings.TrimSuffix(shape, "_right") + "_left"
				}
			}
		} else if state.Name() == "minecraft:rail" || strings.HasSuffix(state.Name(), "_rail") {
			properties["shape"] = t.railShape(shape)
		}
	}
	result, err := block.BlockStateFromProperties(state.Name(), properties)
	if err != nil {
		return 0, err
	}
	return result.ID(), nil
}

// railShape transforms slopes and unordered direction pairs into valid rail states.
func (t templateTransform) railShape(shape string) string {
	if strings.HasPrefix(shape, "ascending_") {
		return "ascending_" + t.direction(strings.TrimPrefix(shape, "ascending_"))
	}
	parts := strings.Split(shape, "_")
	if len(parts) != 2 {
		return shape
	}
	a, b := t.direction(parts[0]), t.direction(parts[1])
	if a == "north" && b == "south" || a == "south" && b == "north" {
		return "north_south"
	}
	if a == "east" && b == "west" || a == "west" && b == "east" {
		return "east_west"
	}
	if a == "east" || a == "west" {
		a, b = b, a
	}
	return a + "_" + b
}
