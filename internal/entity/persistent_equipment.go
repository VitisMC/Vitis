package entity

import (
	"fmt"

	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/nbt"
)

// saveEquipmentNBT writes vanilla hand, armor and body slots with full components.
func (m *MobEntity) saveEquipmentNBT(root *nbt.Compound) error {
	for _, group := range []struct {
		items, chances string
		slots          []EquipmentSlot
	}{
		{"HandItems", "HandDropChances", []EquipmentSlot{EquipmentMainHand, EquipmentOffHand}},
		{"ArmorItems", "ArmorDropChances", []EquipmentSlot{EquipmentFeet, EquipmentLegs, EquipmentChest, EquipmentHead}},
	} {
		items, chances := nbt.NewList(nbt.TagCompound), nbt.NewList(nbt.TagFloat)
		for _, slot := range group.slots {
			stack, err := inventory.ItemStackToNBT(m.equipment[slot])
			if err != nil {
				return fmt.Errorf("save equipment %d: %w", slot, err)
			}
			if stack == nil {
				stack = nbt.NewCompound()
			}
			items.Add(stack)
			chances.Add(m.equipmentDropChance[slot])
		}
		root.PutList(group.items, items).PutList(group.chances, chances)
	}
	body, err := inventory.ItemStackToNBT(m.equipment[EquipmentBody])
	if err != nil {
		return fmt.Errorf("save body equipment: %w", err)
	}
	if body != nil {
		root.PutCompound("body_armor_item", body)
	} else {
		root.Delete("body_armor_item")
	}
	root.PutFloat("body_armor_drop_chance", m.equipmentDropChance[EquipmentBody])
	return nil
}

// restoreEquipmentNBT validates all slots before publishing equipment and modifiers.
func (m *MobEntity) restoreEquipmentNBT(root *nbt.Compound) error {
	if err := ValidatePersistentTags(root, map[string]byte{"HandItems": nbt.TagList, "ArmorItems": nbt.TagList, "HandDropChances": nbt.TagList, "ArmorDropChances": nbt.TagList, "body_armor_item": nbt.TagCompound, "body_armor_drop_chance": nbt.TagFloat}); err != nil {
		return err
	}
	var stacks [7]inventory.ItemStack
	var chances [7]float32
	for slot := range stacks {
		stacks[slot], chances[slot] = inventory.EmptySlot(), .085
	}
	for _, group := range []struct {
		items, chances string
		start, count   int
	}{{"HandItems", "HandDropChances", 0, 2}, {"ArmorItems", "ArmorDropChances", 2, 4}} {
		if list, ok := root.GetList(group.items); ok {
			if list.ElementType() != nbt.TagCompound || list.Len() != group.count {
				return fmt.Errorf("invalid %s equipment list", group.items)
			}
			for index, value := range list.Elements() {
				raw, ok := value.(*nbt.Compound)
				if !ok {
					return fmt.Errorf("invalid equipment compound")
				}
				if len(raw.Keys()) == 0 {
					continue
				}
				stack, err := inventory.ItemStackFromNBT(raw)
				if err != nil {
					return fmt.Errorf("restore equipment: %w", err)
				}
				stacks[group.start+index], err = normalizeEquipmentStack(stack)
				if err != nil {
					return err
				}
			}
		}
		if list, ok := root.GetList(group.chances); ok {
			if list.ElementType() != nbt.TagFloat || list.Len() != group.count {
				return fmt.Errorf("invalid %s probability list", group.chances)
			}
			for index, value := range list.Elements() {
				chance, ok := value.(float32)
				if !ok || !finite(float64(chance)) || chance < 0 || chance > 2 {
					return fmt.Errorf("invalid equipment probability")
				}
				chances[group.start+index] = chance
			}
		}
	}
	if body, ok := root.GetCompound("body_armor_item"); ok {
		stack, err := inventory.ItemStackFromNBT(body)
		if err != nil {
			return fmt.Errorf("restore body armor: %w", err)
		}
		stacks[EquipmentBody], err = normalizeEquipmentStack(stack)
		if err != nil {
			return err
		}
	}
	if chance, ok := root.GetFloat("body_armor_drop_chance"); ok {
		if !finite(float64(chance)) || chance < 0 || chance > 2 {
			return fmt.Errorf("invalid body armor probability")
		}
		chances[EquipmentBody] = chance
	}
	modifiers, err := compileEquipmentModifiers(stacks)
	if err != nil {
		return err
	}
	if err := m.replaceEquipmentModifiers(modifiers); err != nil {
		return err
	}
	m.equipment, m.equipmentDropChance, m.equipmentDirty = stacks, chances, 0x7f
	m.refreshEquipmentArmor()
	return nil
}
