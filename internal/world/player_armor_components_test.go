package world

import (
	"math"
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
)

// TestPlayerArmorWearHonorsFullStackComponents covers resistant, unbreakable and removed behavior.
func TestPlayerArmorWearHonorsFullStackComponents(t *testing.T) {
	for _, variant := range []string{"resistant", "unbreakable", "removed_damage", "removed_max_damage", "no_equippable", "damage_on_hurt_false", "large_damage"} {
		_, player, windows, _ := survivalTestWorld(t, nil)
		stack := inventory.NewSlot(item.IDByName("minecraft:diamond_chestplate"), 1)
		switch variant {
		case "resistant":
			stack = inventory.NewSlot(item.IDByName("minecraft:netherite_chestplate"), 1)
		case "unbreakable":
			stack.SetComponent(dc.ComponentUnbreakable, inventory.BoolComponent(true))
		case "removed_damage":
			stack.RemoveComponent(dc.ComponentDamage)
		case "removed_max_damage":
			stack.RemoveComponent(dc.ComponentMaxDamage)
		case "no_equippable":
			stack.RemoveComponent(dc.ComponentEquippable)
		case "damage_on_hurt_false":
			component, _ := stack.Component(dc.ComponentEquippable)
			value := component.(inventory.StructuredComponent)
			value.Data.(map[string]any)["damageable"] = false
			stack.SetComponent(dc.ComponentEquippable, value)
		case "large_damage":
			stack.SetComponent(dc.ComponentMaxDamage, inventory.VarIntComponent(math.MaxInt32))
			stack.SetEnchant(item.EnchantUnbreakingID, 3)
		}
		equipTestPlayer(player, windows, stack)
		amount := float32(4)
		if variant == "large_damage" {
			amount = math.MaxFloat32
		}
		living := player.Living()
		living.ApplyDamage(entity.NewDamageSource(entity.DamageTypeLava, amount), living.ArmorDefense(), living.ArmorToughness(), living.ArmorEnchProtection())
		for slot := inventory.ArmorStart; slot <= inventory.ArmorEnd; slot++ {
			worn := windows.Inventory().Get(slot)
			if variant == "large_damage" {
				if worn.Damage < 1 || worn.Damage > math.MaxInt32 || worn.Empty() {
					t.Fatal("large Unbreaking wear overflowed or used linear work")
				}
			} else if !worn.StackableWith(stack) {
				t.Fatalf("%s ignored its effective component", variant)
			}
		}
	}
}
