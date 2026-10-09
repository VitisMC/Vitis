package equipment

import (
	"github.com/vitismc/vitis/internal/enchantment"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
)

// ArmorStats holds the cached defense and damage-specific protection of player armor.
type ArmorStats struct {
	Defense, Toughness, Protection float64
	Fire, Blast, Projectile, Fall  float64
}

// PlayerArmorStats calculates existing player armor rules from full inventory stacks.
func PlayerArmorStats(inv *inventory.Container) ArmorStats {
	var result ArmorStats
	for slot := inventory.ArmorStart; slot <= inventory.ArmorEnd; slot++ {
		stack := inv.Get(slot)
		if stack.Empty() {
			continue
		}
		if armor := GetArmor(item.NameByID(stack.ItemID)); armor != nil {
			result.Defense += armor.Defense
			result.Toughness += armor.Toughness
		}
		entries := make([]enchantment.Entry, 0, len(stack.Enchantments))
		for id, level := range stack.Enchantments {
			entries = append(entries, enchantment.Entry{ID: id, Level: level})
		}
		list := enchantment.FromEntries(entries)
		result.Protection += enchantment.ProtectionFactor(list)
		result.Fire += enchantment.FireProtectionFactor(list)
		result.Blast += enchantment.BlastProtectionFactor(list)
		result.Projectile += enchantment.ProjectileProtectionFactor(list)
		result.Fall += float64(enchantment.FeatherFallingLevel(list)) * 3
	}
	result.Protection = min(20, result.Protection)
	return result
}
