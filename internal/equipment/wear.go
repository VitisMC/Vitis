package equipment

import (
	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
)

// MaxDamage resolves effective damageability, including explicitly removed defaults.
func MaxDamage(stack inventory.ItemStack) (int32, bool) {
	if stack.Empty() {
		return 0, false
	}
	if _, present := stack.Component(dc.ComponentUnbreakable); present {
		return 0, false
	}
	if _, present := stack.Component(dc.ComponentDamage); !present {
		return 0, false
	}
	component, present := stack.Component(dc.ComponentMaxDamage)
	value, ok := component.(inventory.VarIntComponent)
	return int32(value), present && ok && value > 0
}

// WearsOnHurt reports whether an effective equippable component permits combat wear.
func WearsOnHurt(stack inventory.ItemStack) bool {
	component, present := stack.Component(dc.ComponentEquippable)
	value, ok := component.(inventory.StructuredComponent)
	if !present || !ok {
		return false
	}
	fields, ok := value.Data.(map[string]any)
	if !ok {
		return false
	}
	wears, ok := fields["damageable"].(bool)
	return ok && wears
}

// Wear applies component durability and Unbreaking, retaining full stacks until breakage.
func Wear(stack inventory.ItemStack, points int32) (inventory.ItemStack, bool) {
	maximum, damageable := MaxDamage(stack)
	if !damageable || points <= 0 {
		return stack, false
	}
	loss := item.DurabilityLoss(points, stack.EnchantLevel(item.EnchantUnbreakingID), IsArmor(item.NameByID(stack.ItemID)))
	if loss == 0 {
		return stack, false
	}
	damage := int64(stack.Damage) + int64(loss)
	if damage >= int64(maximum) {
		return inventory.EmptySlot(), true
	}
	stack = stack.Clone()
	stack.SetDamage(int32(damage))
	return stack, false
}
