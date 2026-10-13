package inventory

import (
	"fmt"
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/item"
)

// stonecuttingTestMenu creates a selected, component-aware two-slot menu.
func stonecuttingTestMenu(t *testing.T, count int32) (*WindowManager, *Window) {
	t.Helper()
	wm := NewWindowManager()
	wm.SetStonecuttingMatcher(func(id int32) []StonecuttingChoice {
		if id != item.IDByName("minecraft:stone") {
			return nil
		}
		result := NewSlot(item.IDByName("minecraft:stone_slab"), 2)
		result.SetComponent(dc.ComponentMaxStackSize, VarIntComponent(16))
		return []StonecuttingChoice{{RecipeID: "test:slabs", Output: result}}
	})
	backing := NewContainer(2)
	input := NewSlot(item.IDByName("minecraft:stone"), count)
	input.SetComponent(dc.ComponentMaxStackSize, VarIntComponent(32))
	backing.Set(0, input)
	w := wm.OpenWindow(WindowTypeStonecutter, "Stonecutter", backing)
	if !wm.SelectStonecuttingRecipe(w.ID, 0) {
		t.Fatal("selection rejected")
	}
	return wm, w
}

// TestStonecuttingPreviewAndTransaction verifies guarded costs and full result stacks.
func TestStonecuttingPreviewAndTransaction(t *testing.T) {
	wm, w := stonecuttingTestMenu(t, 3)
	for i := 0; i < 3; i++ {
		wm.Snapshot(w.ID)
	}
	if w.Backing.Get(0).ItemCount != 3 || wm.SelectedStonecuttingRecipe() != 0 || !wm.CursorItem().Empty() {
		t.Fatal("preview consumed input")
	}
	oldState := wm.CurrentStateID()
	if !wm.HandleClick(w.ID, oldState, 1, 0, ClickTypeNormalClick, map[int16]Slot{0: NewSlot(item.IDByName("minecraft:diamond"), 64)}, NewSlot(item.IDByName("minecraft:diamond"), 64)) {
		t.Fatal("take rejected")
	}
	if w.Backing.Get(0).ItemCount != 2 || wm.CursorItem().ItemCount != 2 || wm.CursorItem().MaxStackSize() != 16 {
		t.Fatal("cost or result components incorrect")
	}
	if wm.HandleClick(w.ID, oldState, 1, 0, ClickTypeNormalClick, nil, EmptySlot()) {
		t.Fatal("stale revision accepted twice")
	}
	if w.Backing.Get(0).MaxStackSize() != 32 || w.Backing.Get(0).ItemCount != 2 {
		t.Fatal("input metadata or count lost")
	}
	if recipes := wm.TakeCraftedRecipes(); len(recipes) != 1 || recipes[0] != "test:slabs" {
		t.Fatalf("committed recipes = %v", recipes)
	}
}

// TestStonecuttingShiftCapacityStopsWithoutLoss verifies bounded batch transactions.
func TestStonecuttingShiftCapacityStopsWithoutLoss(t *testing.T) {
	wm, w := stonecuttingTestMenu(t, 5)
	for i := MainInventoryStart; i <= HotbarEnd; i++ {
		wm.Inventory().Set(i, NewSlot(item.IDByName("minecraft:diamond"), 64))
	}
	if wm.HandleClick(w.ID, wm.CurrentStateID(), 1, 0, ClickTypeShiftClick, nil, EmptySlot()) {
		t.Fatal("full inventory accepted result")
	}
	if w.Backing.Get(0).ItemCount != 5 {
		t.Fatal("full inventory consumed input")
	}
	wm.Inventory().Set(HotbarEnd, w.Backing.Get(1))
	stack := wm.Inventory().Get(HotbarEnd)
	stack.ItemCount = 13
	wm.Inventory().Set(HotbarEnd, stack)
	if !wm.HandleClick(w.ID, wm.CurrentStateID(), 1, 0, ClickTypeShiftClick, nil, EmptySlot()) {
		t.Fatal("batch rejected")
	}
	if wm.Inventory().Get(HotbarEnd).ItemCount != 15 || w.Backing.Get(0).ItemCount != 4 {
		t.Fatal("batch exceeded capacity or lost input")
	}
}

// TestStonecuttingOutputCannotBeInsertedOrCollected prevents preview duplication.
func TestStonecuttingOutputCannotBeInsertedOrCollected(t *testing.T) {
	for _, mode := range []int32{ClickTypeNormalClick, ClickTypeNumberKey, ClickTypeMiddleClick, ClickTypeDoubleClick} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			wm, w := stonecuttingTestMenu(t, 2)
			wm.SetCursorItem(NewSlot(item.IDByName("minecraft:diamond"), 1))
			wm.Inventory().Set(HotbarStart, NewSlot(item.IDByName("minecraft:diamond"), 1))
			if wm.HandleClick(w.ID, wm.CurrentStateID(), 1, 0, mode, nil, EmptySlot()) {
				t.Fatal("unsafe output click accepted")
			}
			if w.Backing.Get(0).ItemCount != 2 {
				t.Fatal("rejected click consumed input")
			}
		})
	}
	wm, w := stonecuttingTestMenu(t, 2)
	wm.SetCursorItem(w.Backing.Get(1))
	wm.HandleClick(w.ID, wm.CurrentStateID(), 2, 0, ClickTypeDoubleClick, nil, EmptySlot())
	if wm.CursorItem().ItemCount != 2 || w.Backing.Get(0).ItemCount != 2 {
		t.Fatal("double click collected preview")
	}
	for _, request := range []struct {
		slot   int16
		button int32
	}{{-999, 0}, {1, 1}, {-999, 2}} {
		wm.HandleClick(w.ID, wm.CurrentStateID(), request.slot, request.button, ClickTypeDrag, nil, EmptySlot())
	}
	if wm.CursorItem().ItemCount != 2 || w.Backing.Get(1).ItemCount != 2 {
		t.Fatal("drag modified output")
	}
}

// TestStonecuttingCloseAndDeathReturnOnlyInputs verifies preview cleanup and single returns.
func TestStonecuttingCloseAndDeathReturnOnlyInputs(t *testing.T) {
	wm, w := stonecuttingTestMenu(t, 3)
	for i := MainInventoryStart; i <= HotbarEnd; i++ {
		wm.Inventory().Set(i, NewSlot(item.IDByName("minecraft:diamond"), 64))
	}
	drops := wm.CloseWindowWithDrops()
	if len(drops) != 1 || drops[0].ItemID != item.IDByName("minecraft:stone") || drops[0].ItemCount != 3 || drops[0].MaxStackSize() != 32 {
		t.Fatalf("close drops = %+v", drops)
	}
	if !w.Backing.Get(0).Empty() || !w.Backing.Get(1).Empty() || len(wm.CloseWindowWithDrops()) != 0 {
		t.Fatal("close retained or duplicated menu items")
	}
	wm, _ = stonecuttingTestMenu(t, 3)
	drops = wm.DropAllOnDeath()
	if len(drops) != 1 || drops[0].ItemCount != 3 || drops[0].ItemID != item.IDByName("minecraft:stone") || len(wm.DropAllOnDeath()) != 0 {
		t.Fatalf("death drops = %+v", drops)
	}
}

// TestStonecuttingInputAndReloadInvalidateSelection verifies resource identity and slot changes.
func TestStonecuttingInputAndReloadInvalidateSelection(t *testing.T) {
	wm, w := stonecuttingTestMenu(t, 3)
	if wm.SelectStonecuttingRecipe(w.ID+1, 0) || wm.SelectStonecuttingRecipe(w.ID, -1) || wm.SelectStonecuttingRecipe(w.ID, 1) {
		t.Fatal("invalid selection accepted")
	}
	wm.SetStonecuttingMatcher(func(int32) []StonecuttingChoice {
		return []StonecuttingChoice{{RecipeID: "test:other", Output: NewSlot(item.IDByName("minecraft:diamond"), 1)}}
	})
	wm.Snapshot(w.ID)
	if !w.Backing.Get(1).Empty() || wm.SelectedStonecuttingRecipe() != -1 {
		t.Fatal("removed recipe retained its preview")
	}
	if wm.HandleClick(w.ID, wm.CurrentStateID(), 1, 0, ClickTypeNormalClick, nil, EmptySlot()) {
		t.Fatal("removed recipe produced result")
	}
	wm.SelectStonecuttingRecipe(w.ID, 0)
	w.Backing.Set(0, NewSlot(item.IDByName("minecraft:dirt"), 1))
	wm.Snapshot(w.ID)
	if !w.Backing.Get(1).Empty() || wm.SelectedStonecuttingRecipe() != -1 {
		t.Fatal("changed item kept previous selection")
	}
}

// TestStonecuttingDropAndNumberKeyPayInput verifies alternate output acquisition paths.
func TestStonecuttingDropAndNumberKeyPayInput(t *testing.T) {
	for _, mode := range []int32{ClickTypeDrop, ClickTypeNumberKey} {
		wm, w := stonecuttingTestMenu(t, 2)
		accepted, dropped := wm.HandleClickWithDrop(w.ID, wm.CurrentStateID(), 1, 0, mode, nil, EmptySlot())
		if !accepted || w.Backing.Get(0).ItemCount != 1 {
			t.Fatal("alternate take did not pay input")
		}
		if mode == ClickTypeDrop && dropped.ItemCount != 1 {
			t.Fatal("drop count incorrect")
		}
		if mode == ClickTypeNumberKey && wm.Inventory().Get(HotbarStart).ItemCount != 2 {
			t.Fatal("hotbar output missing")
		}
	}
}

// TestMenuChangeCancelsDrag prevents slot indices from one menu affecting another.
func TestMenuChangeCancelsDrag(t *testing.T) {
	wm := NewWindowManager()
	chest := wm.OpenWindow(WindowTypeGeneric9x1, "Chest", NewContainer(9))
	wm.SetCursorItem(NewSlot(item.IDByName("minecraft:diamond"), 5))
	for _, request := range []struct {
		slot   int16
		button int32
	}{{-999, 0}, {1, 1}} {
		if !wm.HandleClick(chest.ID, wm.CurrentStateID(), request.slot, request.button, ClickTypeDrag, nil, EmptySlot()) {
			t.Fatal("initial drag rejected")
		}
	}
	menu := wm.OpenWindow(WindowTypeStonecutter, "Stonecutter", NewContainer(2))
	if wm.HandleClick(menu.ID, wm.CurrentStateID(), -999, 2, ClickTypeDrag, nil, EmptySlot()) {
		t.Fatal("drag survived a menu change")
	}
	if wm.CursorItem().ItemCount != 5 || !menu.Backing.Get(1).Empty() {
		t.Fatal("old drag modified the new result slot")
	}
}
