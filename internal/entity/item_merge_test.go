package entity

import (
	"math"
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestItemMergeUsesEffectiveLimitsAndConsumesSource prevents equipment overstacking and repeated transfers.
func TestItemMergeUsesEffectiveLimitsAndConsumesSource(t *testing.T) {
	for _, name := range []string{"minecraft:diamond_sword", "minecraft:bow", "minecraft:iron_helmet"} {
		stack := inventory.NewSlot(item.IDByName(name), 1)
		left, right := NewItemEntity(1, protocol.UUID{1}, Vec3{}, stack), NewItemEntity(2, protocol.UUID{2}, Vec3{}, stack)
		if left.TryMerge(right) || left.Stack().ItemCount != 1 || right.Stack().ItemCount != 1 {
			t.Fatalf("merged non-stackable %s", name)
		}
	}
	stack := inventory.NewSlot(item.IDByName("minecraft:stone"), 8)
	stack.SetComponent(dc.ComponentMaxStackSize, inventory.VarIntComponent(16))
	left, right := NewItemEntity(1, protocol.UUID{1}, Vec3{}, stack), NewItemEntity(2, protocol.UUID{2}, Vec3{}, stack)
	left.age, right.age = 50, 10
	left.SetPickupDelay(2)
	right.SetPickupDelay(29)
	if !left.TryMerge(right) || left.Stack().ItemCount != 16 || !right.Stack().Empty() || left.Age() != 10 || left.PickupDelay() != 29 || left.Dirty()&DirtyItemStack == 0 {
		t.Fatal("merge lost count, age or synchronization")
	}
	if left.TryMerge(right) || left.TryMerge(left) || left.TryMerge(nil) || left.Stack().ItemCount != 16 {
		t.Fatal("merge consumed a source more than once")
	}
	for _, count := range []int32{9, math.MaxInt32} {
		stack.ItemCount = count
		left, right := NewItemEntity(1, protocol.UUID{1}, Vec3{}, stack), NewItemEntity(2, protocol.UUID{2}, Vec3{}, stack)
		if left.TryMerge(right) || left.Stack().ItemCount != count || right.Stack().ItemCount != count {
			t.Fatal("effective limit or integer overflow changed a stack")
		}
	}
	stack.ItemCount = 40
	stack.SetComponent(dc.ComponentMaxStackSize, inventory.VarIntComponent(99))
	left, right = NewItemEntity(1, protocol.UUID{1}, Vec3{}, stack), NewItemEntity(2, protocol.UUID{2}, Vec3{}, stack)
	if left.TryMerge(right) || left.Stack().ItemCount != 40 || right.Stack().ItemCount != 40 {
		t.Fatal("full merge exceeded the item-entity limit of 64")
	}
}
