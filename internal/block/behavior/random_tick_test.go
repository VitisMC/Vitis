package behavior

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
)

// TestCropGrowthConditions checks dry soil, irrigation, crowding, and missing neighbors.
func TestCropGrowthConditions(t *testing.T) {
	wheat := block.StateID("minecraft:wheat", map[string]string{"age": "0"})
	for _, tc := range []struct {
		name     string
		moisture string
		crowded  bool
		missing  bool
		bound    int32
	}{
		{"dry", "0", false, false, 7},
		{"hydrated", "7", false, false, 3},
		{"crowded", "7", true, false, 6},
		{"unavailable", "7", false, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &RandomTickContext{
				Y: 64, StateID: wheat, Light: 15,
				GetBlock: func(x, y, z int) (int32, bool) {
					if tc.missing && x == -1 && z == -1 {
						return 0, false
					}
					if y == 63 {
						return block.StateID("minecraft:farmland", map[string]string{"moisture": tc.moisture}), true
					}
					if tc.crowded && x == 1 && z == 1 {
						return wheat, true
					}
					return 0, true
				},
				RandomBound: func(bound int32) int32 {
					if bound != tc.bound {
						t.Fatalf("growth bound = %d, want %d", bound, tc.bound)
					}
					return 0
				},
			}
			result := GetByState(wheat).(RandomTickBehavior).OnRandomTick(ctx)
			if (result == nil) != tc.missing {
				t.Fatalf("growth = %+v, missing=%v", result, tc.missing)
			}
		})
	}
}

// TestBeetrootRandomGate verifies its additional one-in-three growth attempt.
func TestBeetrootRandomGate(t *testing.T) {
	state := block.StateID("minecraft:beetroots", map[string]string{"age": "0"})
	ctx := &RandomTickContext{StateID: state, Light: 15,
		GetBlock: func(int, int, int) (int32, bool) { return 0, true },
		RandomBound: func(bound int32) int32 {
			if bound != 3 {
				t.Fatalf("beetroot gate = %d", bound)
			}
			return 1
		},
	}
	if GetByState(state).(RandomTickBehavior).OnRandomTick(ctx) != nil {
		t.Fatal("beetroot bypassed initial growth gate")
	}
}
