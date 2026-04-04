package operator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

func tempOpsFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "ops.json")
}

func TestNewListEmpty(t *testing.T) {
	l := NewList(tempOpsFile(t))
	if err := l.Load(); err != nil {
		t.Fatal(err)
	}
	if l.Count() != 0 {
		t.Fatalf("expected 0 ops, got %d", l.Count())
	}
}

func TestAddAndGet(t *testing.T) {
	path := tempOpsFile(t)
	l := NewList(path)

	uuid := protocol.UUID{0x0102030405060708, 0x090a0b0c0d0e0f10}
	if err := l.Add(Operator{UUID: uuid, Name: "TestPlayer", Level: 4}); err != nil {
		t.Fatal(err)
	}

	if !l.IsOp(uuid) {
		t.Fatal("expected IsOp=true")
	}
	if l.GetLevel(uuid) != 4 {
		t.Fatalf("expected level 4, got %d", l.GetLevel(uuid))
	}
	if l.Count() != 1 {
		t.Fatalf("expected 1 op, got %d", l.Count())
	}

	op := l.Get(uuid)
	if op == nil {
		t.Fatal("expected non-nil operator")
	}
	if op.Name != "TestPlayer" {
		t.Fatalf("expected name TestPlayer, got %s", op.Name)
	}
}

func TestRemove(t *testing.T) {
	path := tempOpsFile(t)
	l := NewList(path)

	uuid := protocol.UUID{0x0102, 0x0304}
	_ = l.Add(Operator{UUID: uuid, Name: "Player", Level: 2})
	if err := l.Remove(uuid); err != nil {
		t.Fatal(err)
	}
	if l.IsOp(uuid) {
		t.Fatal("expected IsOp=false after remove")
	}
	if l.GetLevel(uuid) != 0 {
		t.Fatalf("expected level 0 after remove, got %d", l.GetLevel(uuid))
	}
}

func TestLevelClamp(t *testing.T) {
	path := tempOpsFile(t)
	l := NewList(path)

	uuid := protocol.UUID{5, 0}
	_ = l.Add(Operator{UUID: uuid, Name: "Low", Level: -5})
	if l.GetLevel(uuid) != 1 {
		t.Fatalf("expected level clamped to 1, got %d", l.GetLevel(uuid))
	}

	uuid2 := protocol.UUID{6, 0}
	_ = l.Add(Operator{UUID: uuid2, Name: "High", Level: 10})
	if l.GetLevel(uuid2) != 4 {
		t.Fatalf("expected level clamped to 4, got %d", l.GetLevel(uuid2))
	}
}

func TestPersistence(t *testing.T) {
	path := tempOpsFile(t)
	l := NewList(path)

	uuid := protocol.UUID{10, 20}
	_ = l.Add(Operator{UUID: uuid, Name: "Persist", Level: 3, BypassesPlayerLimit: true})

	l2 := NewList(path)
	if err := l2.Load(); err != nil {
		t.Fatal(err)
	}
	if !l2.IsOp(uuid) {
		t.Fatal("expected operator to persist")
	}
	if l2.GetLevel(uuid) != 3 {
		t.Fatalf("expected level 3, got %d", l2.GetLevel(uuid))
	}
	if !l2.BypassesPlayerLimit(uuid) {
		t.Fatal("expected BypassesPlayerLimit=true")
	}
}

func TestAll(t *testing.T) {
	path := tempOpsFile(t)
	l := NewList(path)
	_ = l.Add(Operator{UUID: protocol.UUID{1, 0}, Name: "A", Level: 1})
	_ = l.Add(Operator{UUID: protocol.UUID{2, 0}, Name: "B", Level: 2})

	all := l.All()
	if len(all) != 2 {
		t.Fatalf("expected 2, got %d", len(all))
	}
}

func TestNonExistentGet(t *testing.T) {
	l := NewList(tempOpsFile(t))
	uuid := protocol.UUID{99, 0}
	if l.IsOp(uuid) {
		t.Fatal("expected false for non-existent")
	}
	if l.Get(uuid) != nil {
		t.Fatal("expected nil for non-existent")
	}
	if l.BypassesPlayerLimit(uuid) {
		t.Fatal("expected false for non-existent")
	}
}

func TestLoadCorruptFile(t *testing.T) {
	path := tempOpsFile(t)
	_ = os.WriteFile(path, []byte("not json"), 0644)
	l := NewList(path)
	if err := l.Load(); err == nil {
		t.Fatal("expected error for corrupt file")
	}
}

func TestLevelName(t *testing.T) {
	tests := []struct {
		level int
		name  string
	}{
		{LevelNormal, "normal"},
		{LevelModerator, "moderator"},
		{LevelGamemaster, "gamemaster"},
		{LevelAdmin, "admin"},
		{LevelOwner, "owner"},
		{99, "unknown"},
	}
	for _, tt := range tests {
		if got := LevelName(tt.level); got != tt.name {
			t.Errorf("LevelName(%d) = %q, want %q", tt.level, got, tt.name)
		}
	}
}
