package fluid

import (
	"github.com/vitismc/vitis/internal/block"
	generatedblock "github.com/vitismc/vitis/internal/data/generated/block"
	"testing"
)

// TestFluidStateIntervalsMatchRegistry verifies every state and out-of-range inputs.
func TestFluidStateIntervalsMatchRegistry(t *testing.T) {
	for state := int32(-1); state <= generatedblock.TotalStates; state++ {
		want := FluidTypeNone
		switch block.NameFromState(state) {
		case "minecraft:water":
			want = FluidTypeWater
		case "minecraft:lava":
			want = FluidTypeLava
		}
		if got := GetFluidType(state); got != want {
			t.Fatalf("state %d type %d != %d", state, got, want)
		}
	}
}

// TestContainedFluidTypePreservesPureFluidClassification covers embedded fluid states.
func TestContainedFluidTypePreservesPureFluidClassification(t *testing.T) {
	for _, test := range []struct {
		name            string
		props           map[string]string
		contained, pure FluidType
	}{
		{"minecraft:water", nil, FluidTypeWater, FluidTypeWater},
		{"minecraft:lava", nil, FluidTypeLava, FluidTypeLava},
		{"minecraft:oak_slab", map[string]string{"type": "bottom", "waterlogged": "true"}, FluidTypeWater, FluidTypeNone},
		{"minecraft:oak_slab", map[string]string{"type": "bottom", "waterlogged": "false"}, FluidTypeNone, FluidTypeNone},
		{"minecraft:seagrass", nil, FluidTypeWater, FluidTypeNone},
		{"minecraft:tall_seagrass", nil, FluidTypeWater, FluidTypeNone},
		{"minecraft:kelp", nil, FluidTypeWater, FluidTypeNone},
		{"minecraft:kelp_plant", nil, FluidTypeWater, FluidTypeNone},
		{"minecraft:bubble_column", nil, FluidTypeWater, FluidTypeNone},
		{"minecraft:stone", nil, FluidTypeNone, FluidTypeNone},
		{"minecraft:air", nil, FluidTypeNone, FluidTypeNone},
	} {
		t.Run(test.name+test.props["waterlogged"], func(t *testing.T) {
			state := block.DefaultStateID(test.name)
			if test.props != nil {
				state = block.StateID(test.name, test.props)
			}
			if got := ContainedFluidType(state); got != test.contained || GetFluidType(state) != test.pure {
				t.Fatalf("state %d: contained=%d pure=%d", state, got, GetFluidType(state))
			}
		})
	}
	if ContainedFluidType(-1) != FluidTypeNone {
		t.Fatal("unavailable block contains fluid")
	}
}
