package portal

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
)

// TestFindFrame detects both valid frame geometry and axis.
func TestFindFrame(t *testing.T) {
	states := map[BlockPos]int32{}
	obsidian := block.DefaultStateID("minecraft:obsidian")
	for x := 0; x <= 3; x++ {
		states[BlockPos{X: x, Y: 0}] = obsidian
		states[BlockPos{X: x, Y: 4}] = obsidian
	}
	for y := 0; y <= 4; y++ {
		states[BlockPos{X: 0, Y: y}] = obsidian
		states[BlockPos{X: 3, Y: y}] = obsidian
	}
	frame, ok := FindFrame(1, 1, 0, func(x, y, z int) int32 { return states[BlockPos{X: x, Y: y, Z: z}] })
	if !ok || frame.Axis != "x" || len(frame.Interior) != 6 {
		t.Fatalf("frame = %+v, found=%v", frame, ok)
	}
}

// TestFindFrameAllowsMissingCorners checks minimal frames in both orientations.
func TestFindFrameAllowsMissingCorners(t *testing.T) {
	for _, axis := range []string{"x", "z"} {
		t.Run(axis, func(t *testing.T) {
			states := map[BlockPos]int32{}
			dx, dz := 1, 0
			if axis == "z" {
				dx, dz = 0, 1
			}
			obsidian := block.DefaultStateID("minecraft:obsidian")
			for column := 1; column <= 2; column++ {
				states[BlockPos{X: column * dx, Y: 0, Z: column * dz}] = obsidian
				states[BlockPos{X: column * dx, Y: 4, Z: column * dz}] = obsidian
			}
			for y := 1; y <= 3; y++ {
				states[BlockPos{Y: y}] = obsidian
				states[BlockPos{X: 3 * dx, Y: y, Z: 3 * dz}] = obsidian
			}
			frame, found := FindFrame(dx, 1, dz, func(x, y, z int) int32 {
				return states[BlockPos{X: x, Y: y, Z: z}]
			})
			if !found || frame.Axis != axis || len(frame.Interior) != 6 {
				t.Fatalf("minimal frame=%+v, found=%v", frame, found)
			}
		})
	}
}

// TestFindFrameRejectsBrokenTop verifies complete boundary validation.
func TestFindFrameRejectsBrokenTop(t *testing.T) {
	states := map[BlockPos]int32{}
	obsidian := block.DefaultStateID("minecraft:obsidian")
	for x := 0; x <= 3; x++ {
		states[BlockPos{X: x, Y: 0}] = obsidian
	}
	for y := 0; y <= 4; y++ {
		states[BlockPos{X: 0, Y: y}] = obsidian
		states[BlockPos{X: 3, Y: y}] = obsidian
	}
	if _, ok := FindFrame(1, 1, 0, func(x, y, z int) int32 { return states[BlockPos{X: x, Y: y, Z: z}] }); ok {
		t.Fatal("accepted broken frame")
	}
}

// TestCoordinateScale verifies portal coordinate conversion and clamping.
func TestCoordinateScale(t *testing.T) {
	if OverworldToNether(800) != 100 || NetherToOverworld(-100) != -800 {
		t.Fatal("incorrect coordinate scale")
	}
	if NetherToOverworld(1e9) != 29999984 {
		t.Fatal("coordinate was not clamped")
	}
}
