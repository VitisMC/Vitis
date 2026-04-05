package dimension

import "testing"

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m.Count() != 3 {
		t.Fatalf("expected 3 dimensions, got %d", m.Count())
	}
}

func TestGetOverworld(t *testing.T) {
	m := NewManager()
	d, ok := m.Get("minecraft:overworld")
	if !ok {
		t.Fatal("overworld not found")
	}
	if d.MinY != -64 {
		t.Fatalf("overworld MinY = %d, want -64", d.MinY)
	}
	if !d.HasSkylight {
		t.Fatal("overworld should have skylight")
	}
}

func TestGetNether(t *testing.T) {
	m := NewManager()
	d, ok := m.Get("minecraft:the_nether")
	if !ok {
		t.Fatal("nether not found")
	}
	if d.CoordinateScale != 8.0 {
		t.Fatalf("nether scale = %f, want 8.0", d.CoordinateScale)
	}
	if !d.Ultrawarm {
		t.Fatal("nether should be ultrawarm")
	}
}

func TestGetEnd(t *testing.T) {
	m := NewManager()
	d, ok := m.Get("minecraft:the_end")
	if !ok {
		t.Fatal("end not found")
	}
	if d.Height != 256 {
		t.Fatalf("end height = %d, want 256", d.Height)
	}
}

func TestNames(t *testing.T) {
	m := NewManager()
	names := m.Names()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
}

func TestGetNotFound(t *testing.T) {
	m := NewManager()
	_, ok := m.Get("minecraft:nonexistent")
	if ok {
		t.Fatal("expected not found")
	}
}
