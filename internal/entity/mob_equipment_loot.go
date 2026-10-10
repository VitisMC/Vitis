package entity

import (
	"math/rand"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	ench "github.com/vitismc/vitis/internal/data/generated/enchantment"
	"github.com/vitismc/vitis/internal/inventory"
)

// RollEquipmentDrops consumes equipment selected by the world's one-shot death evaluator.
func (m *MobEntity) RollEquipmentDrops(rng *rand.Rand, playerCredit bool, looting int32) []inventory.ItemStack {
	if !m.IsDead() || rng == nil {
		return nil
	}
	var drops []inventory.ItemStack
	for slot, stack := range m.equipment {
		chance := m.equipmentDropChance[slot]
		guaranteed := chance > 1
		if stack.Empty() || chance == 0 || !playerCredit && !guaranteed || stack.EnchantLevel(ench.EnchantmentIDByName("minecraft:vanishing_curse")) > 0 {
			continue
		}
		probability := float64(chance) + .01*float64(max(0, min(255, looting)))
		if rng.Float64() >= probability {
			continue
		}
		stack = stack.Clone()
		if !guaranteed {
			if maximum, ok := stack.Component(dc.ComponentMaxDamage); ok {
				if _, unbreakable := stack.Component(dc.ComponentUnbreakable); !unbreakable {
					limit := int32(maximum.(inventory.VarIntComponent))
					if limit > 0 {
						stack.SetDamage(limit - int32(rng.Intn(1+rng.Intn(max(int(limit)-3, 1)))))
					}
				}
			}
		}
		drops = append(drops, stack)
		m.equipment[slot] = inventory.EmptySlot()
		m.equipmentDirty |= 1 << slot
	}
	return drops
}

// EquipmentXPBonus counts remaining non-guaranteed equipment after death drops.
func (m *MobEntity) EquipmentXPBonus(rng *rand.Rand) int32 {
	if rng == nil || !m.IsDead() || m.XPReward() <= 0 {
		return 0
	}
	var bonus int32
	for slot, stack := range m.equipment {
		if !stack.Empty() && m.equipmentDropChance[slot] <= 1 {
			bonus += 1 + int32(rng.Intn(3))
		}
	}
	return bonus
}
