package entity

import (
	"fmt"
	"math"
	"slices"
	"sort"

	attr "github.com/vitismc/vitis/internal/data/generated/attribute"
	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	ench "github.com/vitismc/vitis/internal/data/generated/enchantment"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
)

// EquipmentSlot identifies a protocol equipment slot, including animal body armor.
type EquipmentSlot uint8

const (
	EquipmentMainHand EquipmentSlot = iota
	EquipmentOffHand
	EquipmentFeet
	EquipmentLegs
	EquipmentChest
	EquipmentHead
	EquipmentBody
)

// equipmentModifierKey identifies one transient equipment attribute modifier.
type equipmentModifierKey struct {
	attribute int32
	id        string
}

// initializeEquipment assigns fixed species weapons and default drop probabilities.
func (m *MobEntity) initializeEquipment() {
	for slot := range m.equipment {
		m.equipment[slot] = inventory.EmptySlot()
		m.equipmentDropChance[slot] = .085
	}
	m.equipmentModifiers = make(map[equipmentModifierKey]Modifier)
	switch m.TypeName() {
	case "skeleton", "stray":
		m.equipment[EquipmentMainHand] = inventory.NewSlot(item.IDByName("minecraft:bow"), 1)
	case "wither_skeleton":
		m.equipment[EquipmentMainHand] = inventory.NewSlot(item.IDByName("minecraft:stone_sword"), 1)
		key := equipmentModifierKey{attr.AttrAttackDamage, "minecraft:base_attack_damage"}
		modifier := Modifier{ID: key.id, Amount: 4}
		m.equipmentModifiers[key] = modifier
		m.attributes.AddModifier(key.attribute, modifier)
	}
	m.refreshEquipmentArmor()
}

// Equipment returns a detached stack; equipment mutation belongs to the world tick.
func (m *MobEntity) Equipment(slot EquipmentSlot) inventory.ItemStack {
	if int(slot) >= len(m.equipment) {
		return inventory.EmptySlot()
	}
	return m.equipment[slot].Clone()
}

// SetEquipment validates and applies a complete stack and its modifiers atomically.
func (m *MobEntity) SetEquipment(slot EquipmentSlot, stack inventory.ItemStack) error {
	if int(slot) >= len(m.equipment) {
		return fmt.Errorf("invalid equipment slot %d", slot)
	}
	normalized, err := normalizeEquipmentStack(stack)
	if err != nil {
		return err
	}
	trial := m.equipment
	trial[slot] = normalized
	modifiers, err := compileEquipmentModifiers(trial)
	if err != nil {
		return err
	}
	if err := m.replaceEquipmentModifiers(modifiers); err != nil {
		return err
	}
	m.equipment = trial
	m.equipmentDirty |= 1 << slot
	m.refreshEquipmentArmor()
	return nil
}

// normalizeEquipmentStack bounds components and preserves opaque persistent data.
func normalizeEquipmentStack(stack inventory.ItemStack) (inventory.ItemStack, error) {
	if stack.ItemCount < 0 {
		return inventory.ItemStack{}, fmt.Errorf("negative equipment count")
	}
	if stack.Empty() {
		return inventory.EmptySlot(), nil
	}
	if stack.ItemCount > stack.MaxStackSize() {
		return inventory.ItemStack{}, fmt.Errorf("equipment exceeds stack size")
	}
	buffer := protocol.NewBuffer(128)
	if err := inventory.EncodeItemStack(buffer, stack); err != nil {
		return inventory.ItemStack{}, fmt.Errorf("invalid equipment: %w", err)
	}
	result, err := inventory.DecodeSlot(protocol.WrapBuffer(buffer.Bytes()))
	if err != nil {
		return inventory.ItemStack{}, fmt.Errorf("invalid equipment components: %w", err)
	}
	result.PersistentData = stack.PersistentData.Clone()
	return result, nil
}

// EquipmentDropChance returns the saved probability; values above one guarantee a drop.
func (m *MobEntity) EquipmentDropChance(slot EquipmentSlot) float32 {
	if int(slot) >= len(m.equipment) {
		return 0
	}
	return m.equipmentDropChance[slot]
}

// SetEquipmentDropChance validates a probability without changing persistence policy.
func (m *MobEntity) SetEquipmentDropChance(slot EquipmentSlot, chance float32) error {
	if int(slot) >= len(m.equipment) || !finite(float64(chance)) || chance < 0 || chance > 2 {
		return fmt.Errorf("invalid equipment drop probability")
	}
	m.equipmentDropChance[slot] = chance
	return nil
}

// ConsumeEquipmentDirty returns and clears changed slots, including cleared slots.
func (m *MobEntity) ConsumeEquipmentDirty() uint8 {
	dirty := m.equipmentDirty
	m.equipmentDirty = 0
	return dirty
}

// UsesBow reports whether either hand currently holds a bow.
func (m *MobEntity) UsesBow() bool {
	id := item.IDByName("minecraft:bow")
	return m.equipment[EquipmentMainHand].ItemID == id || m.equipment[EquipmentOffHand].ItemID == id
}

// KnockbackResistance returns the effective bounded resistance, including equipment.
func (m *MobEntity) KnockbackResistance() float64 {
	return max(0, min(1, m.attributes.GetValue(attr.AttrKnockbackResistance)))
}

// equipmentSlotMatches implements the item modifier slot groups.
func equipmentSlotMatches(group string, slot EquipmentSlot) bool {
	name := [...]string{"main_hand", "off_hand", "feet", "legs", "chest", "head", "body"}[slot]
	return group == "any" || group == name || group == "hand" && slot <= EquipmentOffHand || group == "armor" && slot >= EquipmentFeet
}

// compileEquipmentModifiers resolves effective components with bounded runtime work.
func compileEquipmentModifiers(stacks [7]inventory.ItemStack) (map[equipmentModifierKey]Modifier, error) {
	result := make(map[equipmentModifierKey]Modifier)
	for slot, stack := range stacks {
		if stack.Empty() {
			continue
		}
		component, ok := stack.Component(dc.ComponentAttributeModifiers)
		if !ok {
			continue
		}
		structured, ok := component.(inventory.StructuredComponent)
		if !ok {
			return nil, fmt.Errorf("invalid equipment attribute component")
		}
		fields, ok := structured.Data.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid equipment attribute fields")
		}
		entries, ok := fields["attributes"].([]any)
		if !ok || len(entries) > 128 {
			return nil, fmt.Errorf("equipment modifier limit exceeded")
		}
		seen := make(map[equipmentModifierKey]bool)
		for _, entry := range entries {
			fields, ok := entry.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid equipment modifier")
			}
			typeID, ok := fields["typeId"].(int32)
			id, idOK := fields["name"].(string)
			amount, amountOK := fields["value"].(float64)
			group, groupOK := fields["slot"].(string)
			operation, operationOK := fields["operation"].(string)
			key := equipmentModifierKey{typeID, id}
			op := slices.Index([]string{"add", "multiply_base", "multiply_total"}, operation)
			if !ok || attr.AttributeByID(typeID) == nil || !idOK || id == "" || !amountOK || !finite(amount) || !groupOK || !operationOK || op < 0 || seen[key] {
				return nil, fmt.Errorf("invalid or duplicate equipment modifier")
			}
			seen[key] = true
			if equipmentSlotMatches(group, EquipmentSlot(slot)) {
				result[key] = Modifier{ID: id, Amount: amount, Operation: int8(op)}
			}
		}
	}
	return result, nil
}

// replaceEquipmentModifiers retains unrelated modifiers and rejects invalid effective values.
func (m *MobEntity) replaceEquipmentModifiers(next map[equipmentModifierKey]Modifier) error {
	a := m.attributes
	a.mu.Lock()
	defer a.mu.Unlock()
	trial := make(map[int32]*Attribute, len(a.attrs))
	for id, current := range a.attrs {
		value := &Attribute{Base: current.Base}
		for _, modifier := range current.Modifiers {
			key := equipmentModifierKey{id, modifier.ID}
			_, previous := m.equipmentModifiers[key]
			_, replacement := next[key]
			if !previous && !replacement {
				value.Modifiers = append(value.Modifiers, modifier)
			}
		}
		trial[id] = value
	}
	keys := make([]equipmentModifierKey, 0, len(next))
	for key := range next {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].attribute != keys[j].attribute {
			return keys[i].attribute < keys[j].attribute
		}
		return keys[i].id < keys[j].id
	})
	for _, key := range keys {
		modifier := next[key]
		if value := trial[key.attribute]; value != nil {
			value.Modifiers = append(value.Modifiers, modifier)
		}
	}
	for id, value := range trial {
		if !finite(value.Value()) || id == attr.AttrMaxHealth && (value.Value() < 1 || value.Value() > math.MaxFloat32) {
			return fmt.Errorf("invalid effective equipment attribute")
		}
	}
	for id, value := range trial {
		if !slices.Equal(value.Modifiers, a.attrs[id].Modifiers) {
			a.attrs[id] = value
			a.dirty[id] = true
		}
	}
	m.equipmentModifiers = next
	return nil
}

// refreshEquipmentArmor caches effective defense and supported armor enchantments.
func (m *MobEntity) refreshEquipmentArmor() {
	var protection, fire, blast, projectile, fall float64
	for slot := EquipmentFeet; slot <= EquipmentBody; slot++ {
		stack := m.equipment[slot]
		protection += float64(stack.EnchantLevel(ench.EnchantmentIDByName("minecraft:protection")))
		fire += 2 * float64(stack.EnchantLevel(ench.EnchantmentIDByName("minecraft:fire_protection")))
		blast += 2 * float64(stack.EnchantLevel(ench.EnchantmentIDByName("minecraft:blast_protection")))
		projectile += 2 * float64(stack.EnchantLevel(ench.EnchantmentIDByName("minecraft:projectile_protection")))
		fall += 3 * float64(stack.EnchantLevel(ench.EnchantmentIDByName("minecraft:feather_falling")))
	}
	m.SetArmorStats(max(0, m.attributes.GetValue(attr.AttrArmor)), max(0, m.attributes.GetValue(attr.AttrArmorToughness)), protection)
	m.SetArmorDamageProtection(fire, blast, projectile, fall)
	maximum := m.attributes.GetValue(attr.AttrMaxHealth)
	m.maxHealth = float32(maximum)
	if m.health > m.maxHealth {
		m.SetHealth(m.maxHealth)
	}
}
