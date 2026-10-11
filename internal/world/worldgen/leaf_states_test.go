package worldgen

import (
	"github.com/vitismc/vitis/internal/block"
	"strconv"
	"testing"
)

// TestLeafStateTransitionsMatchProperties checks valid and unsupported transitions independently.
func TestLeafStateTransitionsMatchProperties(t *testing.T) {
	for id, distances := range leafDistanceStates {
		state, err := block.BlockStateFromID(int32(id))
		if err != nil {
			t.Fatal(err)
		}
		for distance := 1; distance < 7; distance++ {
			want := int32(-1)
			if updated, err := state.WithProperty("distance", strconv.Itoa(distance)); err == nil {
				want = updated.ID()
			}
			if got := distances[distance]; got != want {
				t.Fatalf("state %d distance %d: %d != %d", id, distance, got, want)
			}
		}
	}
}
