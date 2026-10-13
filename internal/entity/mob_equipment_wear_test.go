package entity

import (
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestSunHelmetRetainsStacksBreaksOnceAndRestoresArmor checks component ownership and restart.
func TestSunHelmetRetainsStacksBreaksOnceAndRestoresArmor(t *testing.T) {
	mob := NewMobEntityByName(1, protocol.UUID{1}, "zombie", Vec3{})
	helmet := inventory.NewSlot(item.IDByName("minecraft:diamond_helmet"), 1)
	helmet.SetDamage(10)
	helmet.SetComponent(dc.ComponentMaxDamage, inventory.VarIntComponent(30))
	helmet.SetComponent(dc.ComponentCustomData, inventory.NBTComponent{Value: nbt.NewCompound().PutString("identity", "sun")})
	helmet.SetEnchant(item.EnchantUnbreakingID, 255)
	if err := mob.SetEquipment(EquipmentHead, helmet); err != nil {
		t.Fatal(err)
	}
	for range 512 {
		previous := mob.Equipment(EquipmentHead)
		protected, err := mob.WearSunHelmet()
		if err != nil || !protected {
			t.Fatal("sunlight lost protection before breaking the item")
		}
		current := mob.Equipment(EquipmentHead)
		if current.Empty() {
			break
		}
		if current.Damage < previous.Damage || current.Damage > previous.Damage+1 || helmet.Damage != 10 {
			t.Fatal("sunlight mutated an external stack or used combat wear")
		}
	}
	if !mob.Equipment(EquipmentHead).Empty() || mob.ArmorDefense() != 2 || mob.ArmorToughness() != 0 || mob.ConsumeEquipmentBroken() != 1<<EquipmentHead || mob.ConsumeEquipmentBroken() != 0 {
		t.Fatal("sunlight retained broken gear, its modifiers or repeated the break event")
	}
	if protected, err := mob.WearSunHelmet(); err != nil || protected {
		t.Fatal("empty head slot still protected the next exposure")
	}
	raw, err := mob.PersistentNBT()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := DecodePersistentEntity(2, raw)
	if err != nil || !restored.(*MobEntity).Equipment(EquipmentHead).Empty() || restored.(*MobEntity).ConsumeEquipmentBroken() != 0 {
		t.Fatal("restart restored broken equipment or replayed its event")
	}
}

// TestSunHelmetHonorsEffectiveDamageabilityAndMobState covers harmless head items and immunity.
func TestSunHelmetHonorsEffectiveDamageabilityAndMobState(t *testing.T) {
	for _, variant := range []string{"nondamageable", "unbreakable", "removed_damage", "removed_max_damage", "damage_resistant"} {
		mob := NewMobEntityByName(1, protocol.UUID{1}, "skeleton", Vec3{})
		stack := inventory.NewSlot(item.IDByName("minecraft:diamond_helmet"), 1)
		switch variant {
		case "nondamageable":
			stack = inventory.NewSlot(item.IDByName("minecraft:carved_pumpkin"), 1)
		case "unbreakable":
			stack.SetComponent(dc.ComponentUnbreakable, inventory.BoolComponent(true))
		case "removed_damage":
			stack.RemoveComponent(dc.ComponentDamage)
		case "removed_max_damage":
			stack.RemoveComponent(dc.ComponentMaxDamage)
		case "damage_resistant":
			stack.SetComponent(dc.ComponentDamageResistant, inventory.StructuredComponent{Data: "minecraft:is_fire"})
			stack.SetDamage(item.MaxDurability(stack.ItemID))
		}
		if err := mob.SetEquipment(EquipmentHead, stack); err != nil {
			t.Fatal(err)
		}
		protected, err := mob.WearSunHelmet()
		if err != nil || !protected {
			t.Fatalf("%s failed protection: %v", variant, err)
		}
		if variant == "damage_resistant" {
			if !mob.Equipment(EquipmentHead).Empty() {
				t.Fatal("sun wear incorrectly used the fire resistance tag")
			}
		} else if !mob.Equipment(EquipmentHead).StackableWith(stack) {
			t.Fatal("nondamageable head item was changed")
		}
		mob.SetNoAI(true)
		if protected, err := mob.WearSunHelmet(); err != nil || protected != !mob.Equipment(EquipmentHead).Empty() {
			t.Fatal("NoAI disabled environmental head protection")
		}
	}
	immune := NewMobEntityByName(2, protocol.UUID{2}, "wither_skeleton", Vec3{})
	if err := immune.SetEquipment(EquipmentHead, inventory.NewSlot(item.IDByName("minecraft:diamond_helmet"), 1)); err != nil {
		t.Fatal(err)
	}
	if protected, err := immune.WearSunHelmet(); err != nil || protected || immune.Equipment(EquipmentHead).Damage != 0 {
		t.Fatal("fire-immune mob wore its helmet")
	}
}
