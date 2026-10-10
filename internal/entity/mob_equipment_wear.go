package entity

import (
	"fmt"
	"math/rand"

	"github.com/vitismc/vitis/internal/equipment"
	"github.com/vitismc/vitis/internal/inventory"
)

// WearSunHelmet protects this exposure tick and applies direct sunlight wear.
// Sunlight bypasses Unbreaking; unbreakable and nondamageable head items still protect.
func (m *MobEntity) WearSunHelmet() (bool, error) {
	if m.IsDead() || m.Removed() || m.fireImmune {
		return false, nil
	}
	stack := m.Equipment(EquipmentHead)
	if stack.Empty() {
		return false, nil
	}
	maximum, ok := equipment.MaxDamage(stack)
	if !ok {
		return true, nil
	}
	damage := int64(stack.Damage) + int64(rand.Intn(2))
	if damage >= int64(maximum) {
		if err := m.SetEquipment(EquipmentHead, inventory.EmptySlot()); err != nil {
			return true, fmt.Errorf("break sunlight helmet: %w", err)
		}
		m.equipmentBroken |= 1 << EquipmentHead
	} else if damage != int64(stack.Damage) {
		stack.SetDamage(int32(damage))
		// Wear preserves the already validated component graph and attribute modifiers.
		m.equipment[EquipmentHead] = stack
		m.equipmentDirty |= 1 << EquipmentHead
	}
	return true, nil
}

// ConsumeEquipmentBroken returns and clears pending equipment break events.
func (m *MobEntity) ConsumeEquipmentBroken() uint8 {
	broken := m.equipmentBroken
	m.equipmentBroken = 0
	return broken
}
