package inventory

import (
	"github.com/vitismc/vitis/internal/item"
	"testing"
)

// TestCraftTransactionRemainderAndPreview verifies pure preview and committed leftovers.
func TestCraftTransactionRemainderAndPreview(t *testing.T) {
	wm := NewWindowManager()
	milk := item.IDByName("minecraft:milk_bucket")
	bucket := item.IDByName("minecraft:bucket")
	diamond := item.IDByName("minecraft:diamond")
	wm.SetCraftEvaluator(func(grid []ItemStack, width int) (CraftResult, bool) {
		if grid[0].ItemID != milk || grid[0].Empty() {
			return CraftResult{}, false
		}
		return CraftResult{RecipeID: "test:recipe", Output: NewSlot(diamond, 1), Consume: []int32{1, 0, 0, 0}, Remainders: []ItemStack{NewSlot(bucket, 1), {}, {}, {}}}, true
	})
	commits := 0
	wm.SetCraftCommit(func(stack ItemStack) (ItemStack, error) { commits++; return stack, nil })
	wm.Inventory().Set(1, NewSlot(milk, 1))
	wm.UpdateCraftOutput()
	wm.UpdateCraftOutput()
	if commits != 0 || wm.Inventory().Get(1).ItemID != milk {
		t.Fatal("preview committed ingredients")
	}
	if !wm.HandleClick(0, wm.CurrentStateID(), 0, 0, ClickTypeNormalClick, nil, EmptySlot()) {
		t.Fatal("craft rejected")
	}
	if commits != 1 || wm.CursorItem().ItemID != diamond || wm.Inventory().Get(1).ItemID != bucket {
		t.Fatal("result or remainder missing")
	}
}

// TestCraftShiftClickStopsAtCapacity verifies full inventory does not consume a grid.
func TestCraftShiftClickStopsAtCapacity(t *testing.T) {
	wm := NewWindowManager()
	stone := item.IDByName("minecraft:stone")
	diamond := item.IDByName("minecraft:diamond")
	wm.SetCraftMatcher(func(grid []int32, width int) (int32, int32) {
		if grid[0] == stone {
			return diamond, 1
		}
		return 0, 0
	})
	wm.Inventory().Set(1, NewSlot(stone, 4))
	for i := MainInventoryStart; i <= HotbarEnd; i++ {
		wm.Inventory().Set(i, NewSlot(stone, 64))
	}
	wm.UpdateCraftOutput()
	if wm.HandleClick(0, wm.CurrentStateID(), 0, 0, ClickTypeShiftClick, nil, EmptySlot()) {
		t.Fatal("craft accepted with no space")
	}
	if wm.Inventory().Get(1).ItemCount != 4 {
		t.Fatal("full inventory consumed ingredient")
	}
	wm.Inventory().Set(HotbarEnd, NewSlot(diamond, 62))
	if !wm.HandleClick(0, wm.CurrentStateID(), 0, 0, ClickTypeShiftClick, nil, EmptySlot()) {
		t.Fatal("shift craft rejected")
	}
	if wm.Inventory().Get(1).ItemCount != 2 || wm.Inventory().Get(HotbarEnd).ItemCount != 64 {
		t.Fatal("shift crafting exceeded capacity")
	}
}

// TestRecipePlacementBacktrackingAndRollback verifies overlap and returning a full grid.
func TestRecipePlacementBacktrackingAndRollback(t *testing.T) {
	wm := NewWindowManager()
	oak := item.IDByName("minecraft:oak_planks")
	spruce := item.IDByName("minecraft:spruce_planks")
	wm.Inventory().Set(9, NewSlot(oak, 1))
	wm.Inventory().Set(10, NewSlot(spruce, 1))
	requirements := [][]int32{{oak, spruce}, {oak}, nil, nil}
	if !wm.PlaceRecipe(0, requirements, false) {
		t.Fatal("overlapping ingredients failed")
	}
	if wm.Inventory().Get(1).ItemID != spruce || wm.Inventory().Get(2).ItemID != oak {
		t.Fatal("ingredient assignment did not backtrack")
	}
	for i := MainInventoryStart; i <= HotbarEnd; i++ {
		wm.Inventory().Set(i, NewSlot(item.IDByName("minecraft:stone"), 64))
	}
	before := wm.Inventory().Slots()
	if wm.PlaceRecipe(0, requirements, true) {
		t.Fatal("returned grid into a full inventory")
	}
	after := wm.Inventory().Slots()
	for i := range before {
		if before[i].ItemCount != after[i].ItemCount || !before[i].StackableWith(after[i]) {
			t.Fatalf("rollback mutated slot %d", i)
		}
	}
}
