// Package portal implements Nether portal frame detection and coordinate rules.
package portal

import "github.com/vitismc/vitis/internal/block"

const (
	minWidth  = 2
	maxWidth  = 21
	minHeight = 3
	maxHeight = 21
)

// BlockPos is one integer block position.
type BlockPos struct{ X, Y, Z int }

// Frame is a valid rectangular Nether portal interior.
type Frame struct {
	Axis     string
	Interior []BlockPos
}

// FindFrame finds a valid frame containing the supplied interior position.
func FindFrame(x, y, z int, stateAt func(int, int, int) int32) (Frame, bool) {
	if stateAt == nil {
		return Frame{}, false
	}
	if frame, ok := findAxisFrame(x, y, z, 1, 0, "x", stateAt); ok {
		return frame, true
	}
	return findAxisFrame(x, y, z, 0, 1, "z", stateAt)
}

// FindFrameAxis validates one orientation without reading unrelated frame planes.
func FindFrameAxis(x, y, z int, axis string, stateAt func(int, int, int) int32) (Frame, bool) {
	if stateAt == nil {
		return Frame{}, false
	}
	switch axis {
	case "x":
		return findAxisFrame(x, y, z, 1, 0, axis, stateAt)
	case "z":
		return findAxisFrame(x, y, z, 0, 1, axis, stateAt)
	default:
		return Frame{}, false
	}
}

// OverworldToNether converts and clamps X/Z coordinates for Nether travel.
func OverworldToNether(value float64) float64 {
	return clamp(value/8, -3749998, 3749998)
}

// NetherToOverworld converts and clamps X/Z coordinates for Overworld travel.
func NetherToOverworld(value float64) float64 {
	return clamp(value*8, -29999984, 29999984)
}

func findAxisFrame(x, y, z, dx, dz int, axis string, stateAt func(int, int, int) int32) (Frame, bool) {
	bottomY := y
	for steps := 0; steps < maxHeight && replaceable(stateAt(x, bottomY-1, z)); steps++ {
		bottomY--
	}
	if block.NameFromState(stateAt(x, bottomY-1, z)) != "minecraft:obsidian" {
		return Frame{}, false
	}
	leftX, leftZ := x, z
	for steps := 0; steps < maxWidth && replaceable(stateAt(leftX-dx, bottomY, leftZ-dz)); steps++ {
		leftX -= dx
		leftZ -= dz
	}
	if block.NameFromState(stateAt(leftX-dx, bottomY, leftZ-dz)) != "minecraft:obsidian" {
		return Frame{}, false
	}
	width := 0
	for width <= maxWidth && replaceable(stateAt(leftX+width*dx, bottomY, leftZ+width*dz)) {
		if block.NameFromState(stateAt(leftX+width*dx, bottomY-1, leftZ+width*dz)) != "minecraft:obsidian" {
			return Frame{}, false
		}
		width++
	}
	if width < minWidth || width > maxWidth || block.NameFromState(stateAt(leftX+width*dx, bottomY, leftZ+width*dz)) != "minecraft:obsidian" {
		return Frame{}, false
	}
	height := 0
	for ; height <= maxHeight; height++ {
		if block.NameFromState(stateAt(leftX-dx, bottomY+height, leftZ-dz)) != "minecraft:obsidian" ||
			block.NameFromState(stateAt(leftX+width*dx, bottomY+height, leftZ+width*dz)) != "minecraft:obsidian" {
			break
		}
		validRow := true
		for column := 0; column < width; column++ {
			if !replaceable(stateAt(leftX+column*dx, bottomY+height, leftZ+column*dz)) {
				validRow = false
				break
			}
		}
		if !validRow {
			break
		}
	}
	if height < minHeight || height > maxHeight {
		return Frame{}, false
	}
	for column := 0; column < width; column++ {
		if block.NameFromState(stateAt(leftX+column*dx, bottomY+height, leftZ+column*dz)) != "minecraft:obsidian" {
			return Frame{}, false
		}
	}
	interior := make([]BlockPos, 0, width*height)
	for row := 0; row < height; row++ {
		for column := 0; column < width; column++ {
			interior = append(interior, BlockPos{X: leftX + column*dx, Y: bottomY + row, Z: leftZ + column*dz})
		}
	}
	return Frame{Axis: axis, Interior: interior}, true
}

func replaceable(stateID int32) bool {
	switch block.NameFromState(stateID) {
	case "minecraft:air", "minecraft:cave_air", "minecraft:void_air", "minecraft:fire", "minecraft:nether_portal":
		return true
	default:
		return false
	}
}

func clamp(value, minimum, maximum float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
