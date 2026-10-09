package inventory

import (
	"testing"

	"github.com/vitismc/vitis/internal/item"
)

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

func TestHandleClickRejectsStaleRevisionWithoutMutation(t *testing.T) {
	wm := NewWindowManager()
	wm.Inventory().Set(MainInventoryStart, NewSlot(1, 10))
	before := wm.Inventory().Get(MainInventoryStart)
	if wm.HandleClick(0, 0, MainInventoryStart, ButtonLeftClick, ClickTypeNormalClick, nil, EmptySlot()) {
		t.Fatal("stale state id was accepted")
	}
	if after := wm.Inventory().Get(MainInventoryStart); after.ItemID != before.ItemID || after.ItemCount != before.ItemCount {
		t.Fatalf("stale click mutated slot: before=%+v after=%+v", before, after)
	}
}

func TestShiftClickPreservesItemsWhenDestinationHasPartialCapacity(t *testing.T) {
	wm := NewWindowManager()
	for index := MainInventoryStart; index <= MainInventoryEnd; index++ {
		wm.Inventory().Set(index, NewSlot(1, 64))
	}
	wm.Inventory().Set(MainInventoryStart, NewSlot(1, 60))
	wm.Inventory().Set(HotbarStart, NewSlot(1, 10))

	if !wm.HandleClick(0, wm.CurrentStateID(), HotbarStart, ButtonLeftClick, ClickTypeShiftClick, nil, EmptySlot()) {
		t.Fatal("shift click was rejected")
	}
	if got := wm.Inventory().Get(MainInventoryStart).ItemCount; got != 64 {
		t.Fatalf("destination count = %d, want 64", got)
	}
	if got := wm.Inventory().Get(HotbarStart).ItemCount; got != 6 {
		t.Fatalf("source count = %d, want 6", got)
	}
}

func TestCloseWindowRetainsCursorWhenInventoryIsFull(t *testing.T) {
	wm := NewWindowManager()
	for index := MainInventoryStart; index <= HotbarEnd; index++ {
		wm.Inventory().Set(index, NewSlot(1, 64))
	}
	wm.OpenWindow(WindowTypeGeneric9x3, "Chest", NewContainer(27))
	wm.SetCursorItem(NewSlot(2, 3))
	drops := wm.CloseWindowWithDrops()
	if len(drops) != 1 || drops[0].ItemID != 2 || drops[0].ItemCount != 3 || !wm.CursorItem().Empty() {
		t.Fatalf("cursor conservation on close: drops=%+v cursor=%+v", drops, wm.CursorItem())
	}
}

func TestDropHeldItemPreservesRemainder(t *testing.T) {
	wm := NewWindowManager()
	wm.Inventory().Set(HotbarStart, NewSlot(1, 3))
	if dropped := wm.DropHeldItem(false); dropped.ItemCount != 1 || wm.HeldItem().ItemCount != 2 {
		t.Fatalf("drop one: dropped=%+v held=%+v", dropped, wm.HeldItem())
	}
	if dropped := wm.DropHeldItem(true); dropped.ItemCount != 2 || !wm.HeldItem().Empty() {
		t.Fatalf("drop stack: dropped=%+v held=%+v", dropped, wm.HeldItem())
	}
}

func TestWindowOutsideClickAndSlotDrop(t *testing.T) {
	wm := NewWindowManager()
	wm.SetCursorItem(NewSlot(1, 3))
	ok, dropped := wm.HandleClickWithDrop(0, wm.CurrentStateID(), -999, ButtonRightClick, ClickTypeNormalClick, nil, EmptySlot())
	if !ok || dropped.ItemCount != 1 || wm.CursorItem().ItemCount != 2 {
		t.Fatalf("outside right click: accepted=%v dropped=%+v cursor=%+v", ok, dropped, wm.CursorItem())
	}
	wm.Inventory().Set(MainInventoryStart, NewSlot(1, 4))
	ok, dropped = wm.HandleClickWithDrop(0, wm.CurrentStateID(), MainInventoryStart, 1, ClickTypeDrop, nil, EmptySlot())
	if !ok || dropped.ItemCount != 4 || !wm.Inventory().Get(MainInventoryStart).Empty() {
		t.Fatalf("drop stack: accepted=%v dropped=%+v", ok, dropped)
	}
}

func TestDragAndDoubleClickConserveItems(t *testing.T) {
	wm := NewWindowManager()
	wm.SetCursorItem(NewSlot(1, 10))
	click := func(slot int16, button int32, mode int32) {
		t.Helper()
		if !wm.HandleClick(0, wm.CurrentStateID(), slot, button, mode, nil, EmptySlot()) {
			t.Fatalf("click rejected: slot=%d button=%d mode=%d", slot, button, mode)
		}
	}
	click(-999, 0, ClickTypeDrag)
	if wm.CurrentStateID() != 1 {
		t.Fatal("drag start advanced state before commit")
	}
	click(MainInventoryStart, 1, ClickTypeDrag)
	click(MainInventoryStart, 1, ClickTypeDrag)
	click(MainInventoryStart+1, 1, ClickTypeDrag)
	click(-999, 2, ClickTypeDrag)
	if a, b, cursor := wm.GetSlot(MainInventoryStart).ItemCount, wm.GetSlot(MainInventoryStart+1).ItemCount, wm.CursorItem().ItemCount; a != 5 || b != 5 || cursor != 0 {
		t.Fatalf("drag duplicated items: slots=%d,%d cursor=%d", a, b, cursor)
	}
	click(MainInventoryStart, 0, ClickTypeNormalClick)
	click(MainInventoryStart+1, 0, ClickTypeDoubleClick)
	if got := wm.CursorItem().ItemCount; got != 10 {
		t.Fatalf("double click collected %d items, want 10", got)
	}
}

func TestCraftOutputConsumesIngredients(t *testing.T) {
	wm := NewWindowManager()
	wm.SetCraftMatcher(func(grid []int32, width int) (int32, int32) {
		if width == 2 && grid[0] == 1 {
			return 2, 4
		}
		return 0, 0
	})
	wm.Inventory().Set(CraftInputStart, NewSlot(1, 2))
	wm.UpdateCraftOutput()
	if !wm.HandleClick(0, wm.CurrentStateID(), CraftOutputSlot, ButtonLeftClick, ClickTypeNormalClick, nil, EmptySlot()) {
		t.Fatal("craft output click rejected")
	}
	if wm.CursorItem().ItemCount != 4 || wm.GetSlot(CraftInputStart).ItemCount != 1 || wm.GetSlot(CraftOutputSlot).ItemCount != 4 {
		t.Fatalf("first craft incorrect: cursor=%+v input=%+v output=%+v", wm.CursorItem(), wm.GetSlot(CraftInputStart), wm.GetSlot(CraftOutputSlot))
	}
	if !wm.HandleClick(0, wm.CurrentStateID(), CraftOutputSlot, ButtonLeftClick, ClickTypeNormalClick, nil, EmptySlot()) {
		t.Fatal("second craft output click rejected")
	}
	if wm.CursorItem().ItemCount != 8 || !wm.GetSlot(CraftInputStart).Empty() || !wm.GetSlot(CraftOutputSlot).Empty() {
		t.Fatalf("second craft incorrect: cursor=%+v input=%+v output=%+v", wm.CursorItem(), wm.GetSlot(CraftInputStart), wm.GetSlot(CraftOutputSlot))
	}
}

func TestContainerWindowSharesPlayerInventory(t *testing.T) {
	wm := NewWindowManager()
	wm.Inventory().Set(MainInventoryStart, NewSlot(1, 8))
	backing := NewContainer(27)
	window := wm.OpenWindow(WindowTypeGeneric9x3, "Chest", backing)
	if window.Container.Size() != 63 || window.Container.Get(27).ItemCount != 8 {
		t.Fatalf("window layout incorrect: size=%d player=%+v", window.Container.Size(), window.Container.Get(27))
	}
	if !wm.HandleClick(window.ID, wm.CurrentStateID(), 27, ButtonLeftClick, ClickTypeShiftClick, nil, EmptySlot()) {
		t.Fatal("shift-click into chest rejected")
	}
	if backing.Get(0).ItemCount != 8 || !wm.Inventory().Get(MainInventoryStart).Empty() {
		t.Fatalf("inventory not transferred: chest=%+v player=%+v", backing.Get(0), wm.Inventory().Get(MainInventoryStart))
	}
	if !wm.HandleClick(window.ID, wm.CurrentStateID(), 0, ButtonLeftClick, ClickTypeShiftClick, nil, EmptySlot()) {
		t.Fatal("shift-click back to player rejected")
	}
	if !backing.Get(0).Empty() || wm.Inventory().Get(MainInventoryStart).ItemCount != 8 {
		t.Fatalf("reverse transfer failed: chest=%+v player=%+v", backing.Get(0), wm.Inventory().Get(MainInventoryStart))
	}
}

func TestCraftingTableWindowConsumesThreeByThreeInputs(t *testing.T) {
	wm := NewWindowManager()
	wm.SetCraftMatcher(func(grid []int32, width int) (int32, int32) {
		if width == 3 && grid[8] == 1 {
			return 2, 1
		}
		return 0, 0
	})
	w := wm.OpenWindow(WindowTypeCrafting, "Crafting", NewContainer(10))
	if w.Container.Size() != 46 {
		t.Fatalf("crafting table slot count = %d, want 46", w.Container.Size())
	}
	w.Container.Set(9, NewSlot(1, 1))
	w.Backing.Set(9, NewSlot(1, 1))
	wm.UpdateCraftTableOutput()
	if !wm.HandleClick(w.ID, wm.CurrentStateID(), 0, ButtonLeftClick, ClickTypeNormalClick, nil, EmptySlot()) {
		t.Fatal("crafting table output rejected")
	}
	if wm.CursorItem().ItemID != 2 || !w.Backing.Get(9).Empty() || !w.Backing.Get(0).Empty() {
		t.Fatalf("crafting table transaction failed: cursor=%+v grid=%+v output=%+v", wm.CursorItem(), w.Backing.Get(9), w.Backing.Get(0))
	}
}

func TestCraftingTableCloseReturnsInputs(t *testing.T) {
	wm := NewWindowManager()
	backing := NewContainer(10)
	backing.Set(1, NewSlot(1, 3))
	wm.OpenWindow(WindowTypeCrafting, "Crafting", backing)
	if drops := wm.CloseWindowWithDrops(); len(drops) != 0 {
		t.Fatalf("unexpected drops: %+v", drops)
	}
	if !backing.Get(1).Empty() || wm.Inventory().Get(MainInventoryStart).ItemCount != 3 {
		t.Fatalf("crafting inputs not returned: grid=%+v player=%+v", backing.Get(1), wm.Inventory().Get(MainInventoryStart))
	}
}

func TestShiftCraftConsumesAllAvailableInputs(t *testing.T) {
	wm := NewWindowManager()
	wm.SetCraftMatcher(func(grid []int32, width int) (int32, int32) {
		if width == 2 && grid[0] == 1 {
			return 2, 2
		}
		return 0, 0
	})
	wm.Inventory().Set(CraftInputStart, NewSlot(1, 3))
	wm.UpdateCraftOutput()
	if !wm.HandleClick(0, wm.CurrentStateID(), CraftOutputSlot, ButtonLeftClick, ClickTypeShiftClick, nil, EmptySlot()) {
		t.Fatal("shift craft rejected")
	}
	if !wm.GetSlot(CraftInputStart).Empty() || !wm.GetSlot(CraftOutputSlot).Empty() || wm.GetSlot(MainInventoryStart).ItemCount != 6 {
		t.Fatalf("shift craft did not consume all inputs: input=%+v output=%+v result=%+v", wm.GetSlot(CraftInputStart), wm.GetSlot(CraftOutputSlot), wm.GetSlot(MainInventoryStart))
	}
}

func TestDoubleChestWritesEachHalfSeparately(t *testing.T) {
	wm := NewWindowManager()
	left, right := NewContainer(27), NewContainer(27)
	left.Set(0, NewSlot(1, 2))
	right.Set(0, NewSlot(2, 3))
	w := wm.OpenDoubleChest("Large Chest", left, right)
	if w == nil || w.Container.Size() != 90 || w.Container.Get(27).ItemCount != 3 {
		t.Fatalf("double chest layout = %+v", w)
	}
	if !wm.HandleClick(w.ID, wm.CurrentStateID(), 27, ButtonLeftClick, ClickTypeNormalClick, nil, EmptySlot()) {
		t.Fatal("right-half click rejected")
	}
	if !right.Get(0).Empty() || left.Get(0).ItemCount != 2 || wm.CursorItem().ItemCount != 3 {
		t.Fatalf("half ownership corrupted: left=%+v right=%+v cursor=%+v", left.Get(0), right.Get(0), wm.CursorItem())
	}
}

func TestFurnaceShiftClickRoutesFuelAndInput(t *testing.T) {
	wm := NewWindowManager()
	furnace := NewContainer(3)
	w := wm.OpenWindow(WindowTypeFurnace, "Furnace", furnace)
	w.Container.Set(3, NewSlot(item.IDByName("minecraft:coal"), 2))
	w.Backing.Set(0, EmptySlot())
	wm.Inventory().Set(MainInventoryStart, NewSlot(item.IDByName("minecraft:coal"), 2))
	if !wm.HandleClick(w.ID, wm.CurrentStateID(), 3, ButtonLeftClick, ClickTypeShiftClick, nil, EmptySlot()) {
		t.Fatal("fuel shift click rejected")
	}
	if furnace.Get(1).ItemCount != 2 || !furnace.Get(0).Empty() {
		t.Fatalf("fuel routed incorrectly: input=%+v fuel=%+v", furnace.Get(0), furnace.Get(1))
	}
	wm.Inventory().Set(MainInventoryStart+1, NewSlot(item.IDByName("minecraft:sand"), 3))
	if !wm.HandleClick(w.ID, wm.CurrentStateID(), 4, ButtonLeftClick, ClickTypeShiftClick, nil, EmptySlot()) {
		t.Fatal("input shift click rejected")
	}
	if furnace.Get(0).ItemCount != 3 {
		t.Fatalf("input routed incorrectly: %+v", furnace.Get(0))
	}
}

// TestShiftClickLastHotbarSlot verifies the inclusive hotbar boundary.
func TestShiftClickLastHotbarSlot(t *testing.T) {
	wm := NewWindowManager()
	wm.Inventory().Set(HotbarEnd, NewSlot(1, 10))
	if !wm.HandleClick(0, wm.CurrentStateID(), HotbarEnd, ButtonLeftClick, ClickTypeShiftClick, nil, EmptySlot()) {
		t.Fatal("shift click rejected")
	}
	if !wm.Inventory().Get(HotbarEnd).Empty() || wm.Inventory().Get(MainInventoryStart).ItemCount != 10 {
		t.Fatal("last hotbar slot moved to wrong range")
	}
}
