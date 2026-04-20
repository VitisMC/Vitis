package item

import (
	"math/rand"

	genitem "github.com/vitismc/vitis/internal/data/generated/item"
)

const EnchantUnbreakingID int32 = 39

// MaxDurability returns the maximum durability for an item, or 0 if non-durable.
func MaxDurability(itemID int32) int32 {
	if itemID < 0 || itemID >= int32(len(genitem.Items)) {
		return 0
	}
	return genitem.Items[itemID].MaxDurability
}

// IsDurable returns true if the item has durability (tools, weapons, armor).
func IsDurable(itemID int32) bool {
	return MaxDurability(itemID) > 0
}

// ShouldConsumeDurability returns true if this use should consume a durability point,
// taking the Unbreaking enchantment into account.
// For tools/weapons: chance to not consume = unbreakingLevel / (unbreakingLevel + 1)
// For armor: chance to not consume = 0.6 + 0.4 / (unbreakingLevel + 1)
func ShouldConsumeDurability(unbreakingLevel int32, isArmor bool) bool {
	if unbreakingLevel <= 0 {
		return true
	}
	if isArmor {
		chance := 0.6 + 0.4/float64(unbreakingLevel+1)
		return rand.Float64() < chance
	}
	// Tools/weapons: 1/(level+1) chance to consume
	return rand.Intn(int(unbreakingLevel)+1) == 0
}

// ApplyDamage adds damage to an item and returns whether the item broke.
// currentDamage is the item's current damage value, amount is how much to add.
// Returns the new damage value and whether the item is broken.
func ApplyDamage(itemID int32, currentDamage int32, amount int32) (newDamage int32, broken bool) {
	maxDur := MaxDurability(itemID)
	if maxDur <= 0 {
		return currentDamage, false
	}
	newDamage = currentDamage + amount
	if newDamage >= maxDur {
		return maxDur, true
	}
	return newDamage, false
}
