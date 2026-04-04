package item

import (
	"testing"

	genitem "github.com/vitismc/vitis/internal/data/generated/item"
)

func TestInfoByName(t *testing.T) {
	info := InfoByName("minecraft:diamond_sword")
	if info == nil {
		t.Fatal("expected diamond_sword info")
	}
	if info.StackSize != 1 {
		t.Fatalf("diamond_sword stackSize = %d, want 1", info.StackSize)
	}
	if info.MaxDurability != 1561 {
		t.Fatalf("diamond_sword durability = %d, want 1561", info.MaxDurability)
	}
}

func TestIDByName(t *testing.T) {
	id := IDByName("minecraft:stone")
	if id != 1 {
		t.Fatalf("stone ID = %d, want 1", id)
	}
	id = IDByName("minecraft:nonexistent")
	if id != -1 {
		t.Fatalf("nonexistent ID = %d, want -1", id)
	}
}

func TestNameByID(t *testing.T) {
	name := NameByID(0)
	if name != "minecraft:air" {
		t.Fatalf("ID 0 name = %q, want minecraft:air", name)
	}
}

func TestIsAir(t *testing.T) {
	if !IsAir(0) {
		t.Fatal("0 should be air")
	}
	if IsAir(1) {
		t.Fatal("1 should not be air")
	}
}

func TestStackSize(t *testing.T) {
	if s := StackSize(1); s != 64 {
		t.Fatalf("stone stackSize = %d, want 64", s)
	}
	diamondSwordID := IDByName("minecraft:diamond_sword")
	if s := StackSize(diamondSwordID); s != 1 {
		t.Fatalf("diamond_sword stackSize = %d, want 1", s)
	}
}

func TestItemCount(t *testing.T) {
	if genitem.ItemCount < 1300 {
		t.Fatalf("expected at least 1300 items, got %d", genitem.ItemCount)
	}
}

func BenchmarkIDByName(b *testing.B) {
	for i := 0; i < b.N; i++ {
		IDByName("minecraft:diamond_sword")
	}
}

func BenchmarkInfo(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Info(int32(i % genitem.ItemCount))
	}
}
