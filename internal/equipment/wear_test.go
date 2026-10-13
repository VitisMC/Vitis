package equipment

import (
	"math"
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
)

// TestEquipmentWearUsesComponentsAndPreservesStacks checks removals and wide damage arithmetic.
func TestEquipmentWearUsesComponentsAndPreservesStacks(t *testing.T) {
	stack := inventory.NewSlot(item.IDByName("minecraft:diamond_chestplate"), 1)
	stack.SetDamage(7)
	stack.SetComponent(dc.ComponentCustomData, inventory.NBTComponent{Value: nbt.NewCompound().PutString("identity", "wear")})
	if !WearsOnHurt(stack) {
		t.Fatal("default equippable component missing")
	}
	worn, broken := Wear(stack, 5)
	if broken || worn.Damage != 12 || stack.Damage != 7 {
		t.Fatal("wear ignored damage or changed input ownership")
	}
	worn.SetDamage(7)
	if !worn.StackableWith(stack) {
		t.Fatal("wear removed item components")
	}
	for _, component := range []int32{dc.ComponentDamage, dc.ComponentMaxDamage} {
		copy := stack.Clone()
		copy.RemoveComponent(component)
		if value, broken := Wear(copy, 100); broken || value.Damage != copy.Damage {
			t.Fatal("removed component was inherited again")
		}
	}
	stack.SetComponent(dc.ComponentUnbreakable, inventory.BoolComponent(true))
	if value, broken := Wear(stack, 1000); broken || value.Damage != 7 {
		t.Fatal("unbreakable armor wore out")
	}
	stack.RemoveComponent(dc.ComponentUnbreakable)
	stack.SetComponent(dc.ComponentMaxDamage, inventory.VarIntComponent(math.MaxInt32))
	stack.SetDamage(math.MaxInt32 - 2)
	if value, broken := Wear(stack, math.MaxInt32); !broken || !value.Empty() {
		t.Fatal("damage addition overflowed")
	}
	component, _ := stack.Component(dc.ComponentEquippable)
	value := component.(inventory.StructuredComponent)
	value.Data.(map[string]any)["damageable"] = false
	stack.SetComponent(dc.ComponentEquippable, value)
	if WearsOnHurt(stack) {
		t.Fatal("damage_on_hurt=false ignored")
	}
	stack.RemoveComponent(dc.ComponentEquippable)
	if WearsOnHurt(stack) {
		t.Fatal("removed equippable component ignored")
	}
}
