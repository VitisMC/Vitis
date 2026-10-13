package block

import (
	"testing"

	genblock "github.com/vitismc/vitis/internal/data/generated/block"
)

// TestBlockStateRoundTrip verifies every generated state and its properties.
func TestBlockStateRoundTrip(t *testing.T) {
	for id := int32(0); id < genblock.TotalStates; id++ {
		state, err := BlockStateFromID(id)
		if err != nil {
			t.Fatalf("state %d: %v", id, err)
		}
		resolved, err := BlockStateFromProperties(state.Name(), state.Properties())
		if err != nil || resolved != state {
			t.Fatalf("state %d: resolved %d, error %v", id, resolved.ID(), err)
		}
		for name, want := range state.Properties() {
			got, ok := state.Property(name)
			if !ok || got != want {
				t.Fatalf("state %d property %q: got %q, %v; want %q", id, name, got, ok, want)
			}
		}
	}
}

// TestBlockStateValidation rejects unknown blocks, properties, and IDs.
func TestBlockStateValidation(t *testing.T) {
	for _, id := range []int32{-1, genblock.TotalStates} {
		if _, err := BlockStateFromID(id); err == nil {
			t.Fatalf("accepted invalid state ID %d", id)
		}
	}
	if _, err := DefaultBlockState("minecraft:missing"); err == nil {
		t.Fatal("accepted unknown block")
	}
	for _, properties := range []map[string]string{
		{"missing": "value"},
		{"axis": "invalid"},
	} {
		if _, err := BlockStateFromProperties("minecraft:oak_log", properties); err == nil {
			t.Fatalf("accepted invalid properties %v", properties)
		}
	}
	if _, err := BlockStateFromProperties("minecraft:stone", map[string]string{"axis": "y"}); err == nil {
		t.Fatal("accepted property on propertyless block")
	}
}

// TestBlockStateWithProperty verifies immutable transitions and default values.
func TestBlockStateWithProperty(t *testing.T) {
	initial, err := DefaultBlockState("minecraft:oak_log")
	if err != nil {
		t.Fatal(err)
	}
	next, err := initial.WithProperty("axis", "x")
	if err != nil {
		t.Fatal(err)
	}
	if next == initial {
		t.Fatal("property change kept the same state")
	}
	if got, _ := initial.Property("axis"); got != "y" {
		t.Fatalf("mutated original state: axis=%q", got)
	}
	if got, _ := next.Property("axis"); got != "x" {
		t.Fatalf("new axis=%q, want x", got)
	}
	properties := next.Properties()
	properties["axis"] = "z"
	if got, _ := next.Property("axis"); got != "x" {
		t.Fatalf("external map mutated state: axis=%q", got)
	}
	if _, err := next.WithProperty("axis", "invalid"); err == nil {
		t.Fatal("accepted invalid transition")
	}
}
