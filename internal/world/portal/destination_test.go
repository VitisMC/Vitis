package portal

import (
	"github.com/vitismc/vitis/internal/block"
	"testing"
)

// TestPortalSiteSearchBuildsFrameAndRetainsRelativePosition checks both axes and bounded work.
func TestPortalSiteSearchBuildsFrameAndRetainsRelativePosition(t *testing.T) {
	for _, axis := range []string{"x", "z"} {
		t.Run(axis, func(t *testing.T) {
			states := map[BlockPos]int32{}
			stone := block.DefaultStateID("minecraft:stone")
			read := func(x, y, z int) int32 {
				if state, ok := states[BlockPos{X: x, Y: y, Z: z}]; ok {
					return state
				}
				if y < 64 {
					return stone
				}
				return 0
			}
			search := SiteSearch{Target: BlockPos{X: 8, Y: 64, Z: 8}, Axis: axis, MinY: 0, MaxY: 127}
			if search.Step(1, read) {
				t.Fatal("bounded search completed all columns in one step")
			}
			for !search.Step(8, read) {
			}
			rect, err := search.Build(read, func(x, y, z int, state int32) { states[BlockPos{X: x, Y: y, Z: z}] = state })
			if err != nil {
				t.Fatal(err)
			}
			found, ok := RectangleAt(rect.Min, read)
			if !ok || found != rect {
				t.Fatal("created portal is not rectangular")
			}
			x, y, z := ExitPosition(rect, .5, 0, 0)
			h, v, d := RelativePosition(rect, x, y, z)
			if h != .5 || v != 0 || d != 0 {
				t.Fatal("relative position was not preserved")
			}
		})
	}
}

// TestPortalDestinationRejectsUnavailableWrites requires the full construction volume before mutation.
func TestPortalDestinationRejectsUnavailableWrites(t *testing.T) {
	writes := 0
	read := func(x, y, z int) int32 { return -1 }
	write := func(int, int, int, int32) { writes++ }
	if err := CreateEndPlatform(read, write); err == nil || writes != 0 {
		t.Fatal("unavailable End platform was modified")
	}
	search := SiteSearch{Target: BlockPos{Y: 64}, Axis: "x", MinY: 0, MaxY: 127}
	for !search.Step(64, read) {
	}
	if _, err := search.Build(read, write); err == nil || writes != 0 {
		t.Fatal("unavailable artificial portal was modified")
	}
}
