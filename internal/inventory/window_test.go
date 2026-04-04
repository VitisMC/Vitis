package inventory

import "testing"

func TestWindowManagerInit(t *testing.T) {
	wm := NewWindowManager()
	if wm.Inventory() == nil {
		t.Fatal("inventory should not be nil")
	}
	if wm.Inventory().Size() != PlayerInventorySize {
		t.Fatalf("inventory size = %d, want %d", wm.Inventory().Size(), PlayerInventorySize)
	}
	if wm.HeldSlot() != 0 {
		t.Fatal("initial held slot should be 0")
	}
	if !wm.CursorItem().Empty() {
		t.Fatal("initial cursor should be empty")
	}
}

func TestHeldSlot(t *testing.T) {
	wm := NewWindowManager()
	wm.SetHeldSlot(5)
	if wm.HeldSlot() != 5 {
		t.Fatalf("held slot = %d, want 5", wm.HeldSlot())
	}
	wm.SetHeldSlot(9)
	if wm.HeldSlot() != 5 {
		t.Fatal("out-of-range slot should be rejected")
	}
	wm.SetHeldSlot(-1)
	if wm.HeldSlot() != 5 {
		t.Fatal("negative slot should be rejected")
	}
}

func TestHeldItem(t *testing.T) {
	wm := NewWindowManager()
	wm.Inventory().SetHotbarSlot(3, NewSlot(42, 16))
	wm.SetHeldSlot(3)
	item := wm.HeldItem()
	if item.ItemID != 42 || item.ItemCount != 16 {
		t.Fatalf("held item = %+v, want {42, 16}", item)
	}
}

func TestCursorItem(t *testing.T) {
	wm := NewWindowManager()
	wm.SetCursorItem(NewSlot(10, 1))
	if wm.CursorItem().ItemID != 10 {
		t.Fatal("cursor should have item 10")
	}
}

func TestOpenCloseWindow(t *testing.T) {
	wm := NewWindowManager()
	w := wm.OpenWindow(WindowTypeGeneric9x3, "Chest", NewContainer(27))
	if w == nil {
		t.Fatal("window should not be nil")
	}
	if w.ID == 0 {
		t.Fatal("window ID should be non-zero")
	}
	if wm.ActiveWindow() == nil {
		t.Fatal("active window should not be nil")
	}
	wm.SetCursorItem(NewSlot(5, 1))
	wm.CloseWindow()
	if wm.ActiveWindow() != nil {
		t.Fatal("active window should be nil after close")
	}
	if !wm.CursorItem().Empty() {
		t.Fatal("cursor should be empty after close")
	}
}

func TestCreativeSet(t *testing.T) {
	wm := NewWindowManager()
	wm.HandleCreativeSet(HotbarStart, NewSlot(100, 64))
	got := wm.Inventory().Get(HotbarStart)
	if got.ItemID != 100 || got.ItemCount != 64 {
		t.Fatalf("creative set = %+v, want {100, 64}", got)
	}
}

func TestStateID(t *testing.T) {
	wm := NewWindowManager()
	id1 := wm.StateID()
	id2 := wm.StateID()
	if id2 != id1+1 {
		t.Fatalf("state IDs should increment: %d, %d", id1, id2)
	}
}

func TestAllocWindowID(t *testing.T) {
	id := AllocWindowID()
	if id == 0 {
		t.Fatal("window ID should be non-zero")
	}
}
