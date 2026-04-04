package fluid

import (
	"testing"
)

func TestFluidLevel(t *testing.T) {
	tests := []struct {
		name     string
		stateID  int32
		expected int
	}{
		{"air", 0, 0},
		{"water source", 86, 0},
		{"water level 1", 87, 1},
		{"water level 7", 93, 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := FluidLevel(tt.stateID)
			if level != tt.expected {
				t.Errorf("FluidLevel(%d) = %d, want %d", tt.stateID, level, tt.expected)
			}
		})
	}
}

func TestIsSource(t *testing.T) {
	if !IsSource(86) {
		t.Error("water source (86) should be a source")
	}
	if IsSource(87) {
		t.Error("water level 1 (87) should not be a source")
	}
}

func TestEffectiveLevel(t *testing.T) {
	tests := []struct {
		stateID  int32
		expected int
	}{
		{86, 8},
		{87, 7},
		{93, 1},
	}

	for _, tt := range tests {
		level := EffectiveLevel(tt.stateID)
		if level != tt.expected {
			t.Errorf("EffectiveLevel(%d) = %d, want %d", tt.stateID, level, tt.expected)
		}
	}
}

func TestCanFluidFlowInto(t *testing.T) {
	if !CanFluidFlowInto(0, FluidTypeWater) {
		t.Error("water should flow into air (0)")
	}
}

func TestFindFlowDirections(t *testing.T) {
	getBlock := func(x, y, z int) int32 {
		if y < 64 {
			return 1
		}
		if x == 0 && y == 64 && z == 0 {
			return 86
		}
		return 0
	}

	directions := FindFlowDirections(0, 64, 0, FluidTypeWater, 4, getBlock)
	if len(directions) > 4 {
		t.Errorf("expected at most 4 flow directions, got %d", len(directions))
	}
}

func TestFindFlowDirectionsWithHole(t *testing.T) {
	getBlock := func(x, y, z int) int32 {
		if x == 1 && y == 63 && z == 0 {
			return 0
		}
		if y < 64 {
			return 1
		}
		if x == 0 && y == 64 && z == 0 {
			return 86
		}
		return 0
	}

	directions := FindFlowDirections(0, 64, 0, FluidTypeWater, 4, getBlock)
	if len(directions) != 1 {
		t.Errorf("expected 1 flow direction toward hole, got %d", len(directions))
	}
	if len(directions) > 0 && directions[0] != DirEast {
		t.Errorf("expected flow direction East toward hole, got %v", directions[0])
	}
}

func TestComputeFlow(t *testing.T) {
	getBlock := func(x, y, z int) int32 {
		if y < 64 {
			return 1
		}
		if x == 0 && y == 64 && z == 0 {
			return 86
		}
		return 0
	}

	result := ComputeFlow(0, 64, 0, WaterConfig, getBlock)
	if len(result.NewStates) == 0 {
		t.Error("expected flow result to have new states")
	}
}

func TestInfiniteSourceFormation(t *testing.T) {
	getBlock := func(x, y, z int) int32 {
		if y < 64 {
			return 1
		}
		if y == 64 && ((x == -1 && z == 0) || (x == 1 && z == 0)) {
			return 86
		}
		if y == 64 && x == 0 && z == 0 {
			return 87
		}
		return 0
	}

	newState := ComputeNewFluidState(0, 64, 0, WaterConfig, getBlock)
	if newState == 0 {
		t.Error("expected infinite source formation to produce a valid state, got 0")
	}
}
