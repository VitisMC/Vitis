package inventory

import (
	"testing"

	"github.com/vitismc/vitis/internal/item"
)

// TestDropAllOnDeathDropsEachStackOnce includes armor, cursor, and open crafting input.
func TestDropAllOnDeathDropsEachStackOnce(t *testing.T) {
	wm := NewWindowManager()
	stone := item.IDByName("minecraft:stone")
	wm.Inventory().Set(MainInventoryStart, NewSlot(stone, 2))
	wm.Inventory().Set(ArmorStart, NewSlot(stone, 1))
	wm.SetCursorItem(NewSlot(stone, 3))
	w := wm.OpenWindow(WindowTypeCrafting, "Crafting", NewContainer(10))
	w.Backing.Set(1, NewSlot(stone, 4))
	drops := wm.DropAllOnDeath()
	count := int32(0)
	for _, stack := range drops {
		count += stack.ItemCount
	}
	if count != 10 {
		t.Fatalf("dropped %d items, want 10", count)
	}
	if len(wm.DropAllOnDeath()) != 0 {
		t.Fatal("death drops were produced twice")
	}
}
