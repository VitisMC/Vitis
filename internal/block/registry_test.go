package block

import (
	"testing"

	genblock "github.com/vitismc/vitis/internal/data/generated/block"
)

func TestInfoByName(t *testing.T) {
	info := InfoByName("minecraft:stone")
	if info == nil {
		t.Fatal("expected stone block info")
	}
	if info.Name != "minecraft:stone" {
		t.Fatalf("expected minecraft:stone, got %s", info.Name)
	}
	if info.DefaultState != 1 {
		t.Fatalf("expected default state 1, got %d", info.DefaultState)
	}
}

func TestInfoByNameNotFound(t *testing.T) {
	info := InfoByName("minecraft:nonexistent_block")
	if info != nil {
		t.Fatal("expected nil for nonexistent block")
	}
}

func TestDefaultStateID(t *testing.T) {
	tests := []struct {
		name    string
		want    int32
		wantErr bool
	}{
		{"minecraft:air", 0, false},
		{"minecraft:stone", 1, false},
		{"minecraft:bedrock", -1, true},
		{"minecraft:nonexistent", -1, true},
	}

	for _, tt := range tests {
		got := DefaultStateID(tt.name)
		if tt.wantErr {
			if tt.name == "minecraft:nonexistent" && got != -1 {
				t.Errorf("DefaultStateID(%q) = %d, want -1", tt.name, got)
			}
		} else {
			if got != tt.want {
				t.Errorf("DefaultStateID(%q) = %d, want %d", tt.name, got, tt.want)
			}
		}
	}
}

func TestBlockIDFromState(t *testing.T) {
	bid := BlockIDFromState(0)
	if bid != 0 {
		t.Fatalf("state 0 should map to block 0 (air), got %d", bid)
	}

	bid = BlockIDFromState(1)
	if bid != 1 {
		t.Fatalf("state 1 should map to block 1 (stone), got %d", bid)
	}

	bid = BlockIDFromState(-1)
	if bid != -1 {
		t.Fatalf("state -1 should return -1, got %d", bid)
	}
}

func TestIsAir(t *testing.T) {
	if !IsAir(0) {
		t.Fatal("state 0 should be air")
	}
	if IsAir(1) {
		t.Fatal("state 1 should not be air")
	}
}

func TestIsSolid(t *testing.T) {
	if IsSolid(0) {
		t.Fatal("air should not be solid")
	}
	if !IsSolid(1) {
		t.Fatal("stone should be solid")
	}
}

func TestGrassBlockStates(t *testing.T) {
	info := InfoByName("minecraft:grass_block")
	if info == nil {
		t.Fatal("expected grass_block info")
	}
	if len(info.Properties) != 1 {
		t.Fatalf("expected 1 property, got %d", len(info.Properties))
	}
	if info.Properties[0].Name != "snowy" {
		t.Fatalf("expected property 'snowy', got %q", info.Properties[0].Name)
	}
	if info.MaxStateID-info.MinStateID+1 != 2 {
		t.Fatalf("expected 2 states for grass_block, got %d", info.MaxStateID-info.MinStateID+1)
	}
}

func TestStateIDComputation(t *testing.T) {
	sid := StateID("minecraft:grass_block", map[string]string{"snowy": "true"})
	if sid < 0 {
		t.Fatal("expected valid state ID for grass_block snowy=true")
	}
	info := InfoByName("minecraft:grass_block")
	if sid < info.MinStateID || sid > info.MaxStateID {
		t.Fatalf("state ID %d out of range [%d, %d]", sid, info.MinStateID, info.MaxStateID)
	}

	sidFalse := StateID("minecraft:grass_block", map[string]string{"snowy": "false"})
	if sidFalse == sid {
		t.Fatal("snowy=true and snowy=false should have different state IDs")
	}

	sidDefault := StateID("minecraft:stone", nil)
	if sidDefault != 1 {
		t.Fatalf("stone default state should be 1, got %d", sidDefault)
	}
}

func TestStateIDOakLog(t *testing.T) {
	info := InfoByName("minecraft:oak_log")
	if info == nil {
		t.Fatal("expected oak_log info")
	}

	sidY := StateID("minecraft:oak_log", map[string]string{"axis": "y"})
	if sidY != info.DefaultState {
		t.Fatalf("oak_log axis=y should be default state %d, got %d", info.DefaultState, sidY)
	}

	sidX := StateID("minecraft:oak_log", map[string]string{"axis": "x"})
	sidZ := StateID("minecraft:oak_log", map[string]string{"axis": "z"})
	if sidX == sidY || sidY == sidZ || sidX == sidZ {
		t.Fatalf("oak_log axes should have distinct states: x=%d y=%d z=%d", sidX, sidY, sidZ)
	}
}

func TestPropertiesFromState(t *testing.T) {
	info := InfoByName("minecraft:oak_log")
	if info == nil {
		t.Fatal("expected oak_log info")
	}

	props := PropertiesFromState(info.DefaultState)
	if props == nil {
		t.Fatal("expected properties for oak_log default state")
	}
	if props["axis"] != "y" {
		t.Fatalf("expected axis=y for oak_log default, got axis=%s", props["axis"])
	}
}

func TestStateIDRoundTrip(t *testing.T) {
	names := []string{"minecraft:oak_stairs", "minecraft:oak_log", "minecraft:grass_block"}
	for _, name := range names {
		info := InfoByName(name)
		if info == nil {
			t.Fatalf("expected info for %s", name)
			continue
		}
		for sid := info.MinStateID; sid <= info.MaxStateID; sid++ {
			props := PropertiesFromState(sid)
			computed := StateID(name, props)
			if computed != sid {
				t.Fatalf("round-trip failed for %s state %d: props=%v → computed=%d", name, sid, props, computed)
			}
		}
	}
}

func TestTotalStates(t *testing.T) {
	if genblock.TotalStates < 27000 {
		t.Fatalf("expected at least 27000 total states, got %d", genblock.TotalStates)
	}
}

func BenchmarkStateID(b *testing.B) {
	props := map[string]string{"facing": "north", "half": "bottom", "shape": "straight", "waterlogged": "false"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		StateID("minecraft:oak_stairs", props)
	}
}

func BenchmarkBlockIDFromState(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BlockIDFromState(int32(i % genblock.TotalStates))
	}
}

func BenchmarkPropertiesFromState(b *testing.B) {
	info := InfoByName("minecraft:oak_stairs")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PropertiesFromState(info.MinStateID + int32(i%80))
	}
}
