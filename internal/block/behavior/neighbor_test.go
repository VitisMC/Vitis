package behavior

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
)

// TestCropNeighborSupport checks resident support, light, and conservative missing data.
func TestCropNeighborSupport(t *testing.T) {
	crop := block.DefaultStateID("minecraft:wheat")
	b := GetByState(crop).(NeighborBehavior)
	for _, test := range []struct {
		name      string
		support   int32
		available bool
		light     uint8
		removed   bool
	}{
		{"farmland", block.DefaultStateID("minecraft:farmland"), true, 8, false},
		{"dark", block.DefaultStateID("minecraft:farmland"), true, 7, true},
		{"air", 0, true, 15, true},
		{"unavailable", 0, false, 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := b.OnNeighborCheck(&NeighborContext{StateID: crop, Light: test.light,
				GetBlock: func(int, int, int) (int32, bool) { return test.support, test.available },
			})
			if test.removed {
				if result == nil || !result.Changed || !result.Drop || result.StateID != 0 {
					t.Fatalf("removal = %+v", result)
				}
			} else if result != nil {
				t.Fatalf("valid or unknown support changed the crop: %+v", result)
			}
		})
	}
}

// TestFarmlandNeighborDelay checks delayed conversion and missing upper cells.
func TestFarmlandNeighborDelay(t *testing.T) {
	state := block.DefaultStateID("minecraft:farmland")
	b := GetByState(state).(NeighborBehavior)
	ctx := &NeighborContext{StateID: state, GetBlock: func(int, int, int) (int32, bool) {
		return block.DefaultStateID("minecraft:stone"), true
	}}
	result := b.OnNeighborCheck(ctx)
	if result == nil || result.Delay != 1 || result.Changed {
		t.Fatalf("initial update = %+v", result)
	}
	ctx.Scheduled = true
	result = b.OnNeighborCheck(ctx)
	if result == nil || !result.Changed || result.Drop || result.StateID != block.DefaultStateID("minecraft:dirt") {
		t.Fatalf("scheduled update = %+v", result)
	}
	for _, above := range []int32{0, block.DefaultStateID("minecraft:oak_fence_gate"), block.DefaultStateID("minecraft:moving_piston")} {
		ctx.GetBlock = func(int, int, int) (int32, bool) { return above, true }
		if result := b.OnNeighborCheck(ctx); result != nil {
			t.Fatalf("allowed upper state %d converted farmland: %+v", above, result)
		}
	}
	ctx.GetBlock = func(int, int, int) (int32, bool) { return 0, false }
	if b.OnNeighborCheck(ctx) != nil {
		t.Fatal("unavailable upper cell converted farmland")
	}
}
