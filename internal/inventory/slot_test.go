package inventory

import "testing"

func TestEmptySlot(t *testing.T) {
	s := EmptySlot()
	if !s.Empty() {
		t.Fatal("expected empty")
	}
}

func TestSlotNotEmpty(t *testing.T) {
	s := NewSlot(1, 64)
	if s.Empty() {
		t.Fatal("expected not empty")
	}
}

func TestPlayerInventory(t *testing.T) {
	inv := NewPlayerInventory()
	if inv.Size() != PlayerInventorySize {
		t.Fatalf("size = %d, want %d", inv.Size(), PlayerInventorySize)
	}
	if !inv.Get(0).Empty() {
		t.Fatal("slot 0 should be empty")
	}
}

func TestSetGet(t *testing.T) {
	inv := NewPlayerInventory()
	inv.Set(HotbarStart, NewSlot(10, 32))
	got := inv.Get(HotbarStart)
	if got.ItemID != 10 || got.ItemCount != 32 {
		t.Fatalf("got %+v, want {10, 32}", got)
	}
}

func TestHotbarSlot(t *testing.T) {
	inv := NewPlayerInventory()
	inv.SetHotbarSlot(0, NewSlot(5, 1))
	got := inv.HotbarSlot(0)
	if got.ItemID != 5 {
		t.Fatalf("hotbar 0 = %+v, want itemID=5", got)
	}
}

func TestClearAll(t *testing.T) {
	inv := NewPlayerInventory()
	inv.Set(10, NewSlot(1, 1))
	inv.ClearAll()
	if !inv.Get(10).Empty() {
		t.Fatal("expected empty after ClearAll")
	}
}

func TestOutOfBounds(t *testing.T) {
	inv := NewPlayerInventory()
	s := inv.Get(-1)
	if !s.Empty() {
		t.Fatal("out of bounds should return empty")
	}
	s = inv.Get(100)
	if !s.Empty() {
		t.Fatal("out of bounds should return empty")
	}
	inv.Set(-1, NewSlot(1, 1))
	inv.Set(100, NewSlot(1, 1))
}

func TestSwapSlots(t *testing.T) {
	inv := NewPlayerInventory()
	inv.Set(0, NewSlot(1, 10))
	inv.Set(1, NewSlot(2, 20))
	inv.SwapSlots(0, 1)
	if inv.Get(0).ItemID != 2 || inv.Get(1).ItemID != 1 {
		t.Fatal("swap failed")
	}
}

func TestFindFirst(t *testing.T) {
	inv := NewPlayerInventory()
	inv.Set(5, NewSlot(42, 1))
	idx := inv.FindFirst(func(s Slot) bool { return s.ItemID == 42 })
	if idx != 5 {
		t.Fatalf("expected 5, got %d", idx)
	}
	idx = inv.FindFirst(func(s Slot) bool { return s.ItemID == 999 })
	if idx != -1 {
		t.Fatalf("expected -1, got %d", idx)
	}
}

func TestSlots(t *testing.T) {
	inv := NewPlayerInventory()
	inv.Set(0, NewSlot(1, 1))
	slots := inv.Slots()
	if len(slots) != PlayerInventorySize {
		t.Fatalf("expected %d slots, got %d", PlayerInventorySize, len(slots))
	}
	if slots[0].ItemID != 1 {
		t.Fatal("snapshot should contain set item")
	}
	inv.Set(0, EmptySlot())
	if slots[0].ItemID != 1 {
		t.Fatal("snapshot should be independent of container")
	}
}
