package entity

import (
	"fmt"
	"math/rand"

	attr "github.com/vitismc/vitis/internal/data/generated/attribute"
	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/equipment"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
)

// EquipPickedItem atomically equips an eligible full stack and returns any replaced-item drop.
// The world consumes exactly the returned equipped count from the source item entity.
func (m *MobEntity) EquipPickedItem(stack inventory.ItemStack, rules *equipment.PickupRules, random *rand.Rand) (inventory.ItemStack, inventory.ItemStack, error) {
	if !m.canPickUpLoot || m.IsDead() || m.Removed() || random == nil || !rules.CanHoldItem(m.TypeName(), stack) {
		return inventory.EmptySlot(), inventory.EmptySlot(), nil
	}
	normalized, err := normalizeEquipmentStack(stack)
	if err != nil {
		return inventory.EmptySlot(), inventory.EmptySlot(), fmt.Errorf("normalize picked equipment: %w", err)
	}
	slot := pickupEquipmentSlot(normalized)
	var trial [7]inventory.ItemStack
	trial[slot] = normalized
	if _, err := compileEquipmentModifiers(trial); err != nil {
		return inventory.EmptySlot(), inventory.EmptySlot(), err
	}
	current := m.Equipment(slot)
	replace := m.canReplacePickedItem(normalized, current, slot, rules)
	if slot >= EquipmentFeet && !replace {
		slot, current = EquipmentMainHand, m.Equipment(EquipmentMainHand)
		replace = current.Empty()
	}
	if !replace {
		return inventory.EmptySlot(), inventory.EmptySlot(), nil
	}
	if slot >= EquipmentFeet {
		normalized.ItemCount = 1
	}
	chance := m.EquipmentDropChance(slot)
	if err := m.SetEquipment(slot, normalized); err != nil {
		return inventory.EmptySlot(), inventory.EmptySlot(), fmt.Errorf("equip picked stack: %w", err)
	}
	m.equipmentDropChance[slot] = 2
	m.persistenceRequired = true
	m.noActionTime = 0
	if current.Empty() || max(random.Float32()-.1, 0) >= chance {
		current = inventory.EmptySlot()
	}
	return normalized.Clone(), current, nil
}

// pickupEquipmentSlot resolves effective equippable data and excludes humanoid body armor.
func pickupEquipmentSlot(stack inventory.ItemStack) EquipmentSlot {
	component, present := stack.Component(dc.ComponentEquippable)
	value, ok := component.(inventory.StructuredComponent)
	if !present || !ok {
		return EquipmentMainHand
	}
	fields, ok := value.Data.(map[string]any)
	if !ok {
		return EquipmentMainHand
	}
	switch fields["slot"] {
	case "off_hand":
		return EquipmentOffHand
	case "feet":
		return EquipmentFeet
	case "legs":
		return EquipmentLegs
	case "chest":
		return EquipmentChest
	case "head":
		return EquipmentHead
	default:
		return EquipmentMainHand
	}
}

// canReplacePickedItem applies species preference, attributes and equal-item tie breakers.
func (m *MobEntity) canReplacePickedItem(candidate, current inventory.ItemStack, slot EquipmentSlot, rules *equipment.PickupRules) bool {
	if m.TypeName() == "drowned" && !current.Empty() && current.ItemID == item.IDByName("minecraft:nautilus_shell") {
		return false
	}
	if current.Empty() {
		return true
	}
	attributes := []int32{attr.AttrAttackDamage}
	if slot >= EquipmentFeet {
		if rules.PreventsArmorChange(current) {
			return false
		}
		attributes = []int32{attr.AttrArmor, attr.AttrArmorToughness}
	} else if slot == EquipmentMainHand {
		preferred, heldPreferred := rules.IsPreferredWeapon(m.TypeName(), candidate), rules.IsPreferredWeapon(m.TypeName(), current)
		if preferred != heldPreferred {
			return preferred
		}
	} else {
		return false
	}
	for _, attribute := range attributes {
		base := m.attributes.GetBase(attribute)
		left, right := approximatePickedAttribute(candidate, slot, base), approximatePickedAttribute(current, slot, base)
		if left != right {
			return left > right
		}
	}
	if len(candidate.Enchantments) != len(current.Enchantments) {
		return len(candidate.Enchantments) > len(current.Enchantments)
	}
	if candidate.Damage != current.Damage {
		return candidate.Damage < current.Damage
	}
	_, named := candidate.Component(dc.ComponentCustomName)
	_, heldNamed := current.Component(dc.ComponentCustomName)
	return named && !heldNamed
}

// approximatePickedAttribute scores all matching-slot modifiers in component order.
func approximatePickedAttribute(stack inventory.ItemStack, slot EquipmentSlot, base float64) float64 {
	component, present := stack.Component(dc.ComponentAttributeModifiers)
	if !present {
		return base
	}
	fields := component.(inventory.StructuredComponent).Data.(map[string]any)
	value := base
	for _, raw := range fields["attributes"].([]any) {
		entry := raw.(map[string]any)
		if !equipmentSlotMatches(entry["slot"].(string), slot) {
			continue
		}
		amount := entry["value"].(float64)
		switch entry["operation"].(string) {
		case "add":
			value += amount
		case "multiply_base":
			value += amount * base
		case "multiply_total":
			value += amount * value
		}
	}
	return value
}
