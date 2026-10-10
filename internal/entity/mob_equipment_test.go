package entity

import (
	"math"
	"math/rand"
	"reflect"
	"testing"

	attr "github.com/vitismc/vitis/internal/data/generated/attribute"
	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	ench "github.com/vitismc/vitis/internal/data/generated/enchantment"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestMobEquipmentAttributesReplaceAndClear verifies inherited components and modifier ownership.
func TestMobEquipmentAttributesReplaceAndClear(t *testing.T) {
	mob := NewMobEntityByName(1, protocol.UUID{1}, "wither_skeleton", Vec3{})
	if mob.Attributes().GetValue(attr.AttrAttackDamage) != 8 || mob.Equipment(EquipmentMainHand).ItemID != item.IDByName("minecraft:stone_sword") {
		t.Fatal("missing default sword")
	}
	mob.Attributes().AddModifier(attr.AttrMovementSpeed, Modifier{ID: "example:speed", Amount: .1})
	for range 3 {
		if err := mob.SetEquipment(EquipmentMainHand, inventory.NewSlot(item.IDByName("minecraft:diamond_sword"), 1)); err != nil {
			t.Fatal(err)
		}
		if mob.Attributes().GetValue(attr.AttrAttackDamage) != 10 {
			t.Fatal("weapon modifier accumulated")
		}
	}
	chest := inventory.NewSlot(item.IDByName("minecraft:diamond_chestplate"), 1)
	chest.SetEnchant(ench.EnchantmentIDByName("minecraft:protection"), 4)
	if err := mob.SetEquipment(EquipmentChest, chest); err != nil {
		t.Fatal(err)
	}
	if mob.ArmorDefense() != 8 || mob.ArmorToughness() != 2 || mob.ArmorEnchProtection() != 4 {
		t.Fatal("armor components did not reach damage caches")
	}
	if err := mob.SetEquipment(EquipmentFeet, inventory.NewSlot(item.IDByName("minecraft:netherite_boots"), 1)); err != nil {
		t.Fatal(err)
	}
	if math.Abs(mob.KnockbackResistance()-.1) > 1e-7 {
		t.Fatal("equipment resistance ignored")
	}
	if err := mob.SetEquipment(EquipmentOffHand, mob.Equipment(EquipmentMainHand)); err != nil {
		t.Fatal(err)
	}
	if err := mob.SetEquipment(EquipmentMainHand, inventory.EmptySlot()); err != nil {
		t.Fatal(err)
	}
	if mob.Attributes().GetValue(attr.AttrAttackDamage) != 4 || mob.Attributes().GetValue(attr.AttrMovementSpeed) != .35 {
		t.Fatal("clearing retained gear modifiers or removed unrelated state")
	}
	if mob.ConsumeEquipmentDirty() != 0x17 || mob.ConsumeEquipmentDirty() != 0 {
		t.Fatal("equipment dirty slots lost")
	}
}

// TestMobEquipmentRoundTripRetainsComponentsWithoutDuplicateModifiers exercises all slot encodings.
func TestMobEquipmentRoundTripRetainsComponentsWithoutDuplicateModifiers(t *testing.T) {
	mob := NewMobEntityByName(1, protocol.UUID{2}, "zombie", Vec3{Y: 64})
	stack, err := inventory.ItemStackFromNBT(nbt.NewCompound().PutString("id", "minecraft:diamond_sword").PutInt("count", 1).PutString("example:identity", "retained").PutCompound("components", nbt.NewCompound().PutInt("minecraft:damage", 7).PutCompound("example:unknown", nbt.NewCompound().PutString("keep", "yes"))))
	if err != nil {
		t.Fatal(err)
	}
	for slot := EquipmentMainHand; slot <= EquipmentBody; slot++ {
		if err := mob.SetEquipment(slot, stack); err != nil {
			t.Fatal(err)
		}
		if err := mob.SetEquipmentDropChance(slot, float32(slot)/3); err != nil {
			t.Fatal(err)
		}
	}
	copy := mob.Equipment(EquipmentMainHand)
	copy.SetDamage(100)
	copy.PersistentData.PutString("example:identity", "changed")
	mob.Attributes().AddModifier(attr.AttrAttackDamage, Modifier{ID: "example:attack", Amount: 2})
	for range 3 {
		raw, err := mob.PersistentNBT()
		if err != nil {
			t.Fatal(err)
		}
		restored, err := DecodePersistentEntity(2, raw)
		if err != nil {
			t.Fatal(err)
		}
		mob = restored.(*MobEntity)
		if mob.Attributes().GetValue(attr.AttrAttackDamage) != 11 || mob.ArmorDefense() != 2 {
			t.Fatal("restart changed equipment or innate armor")
		}
		for slot := EquipmentMainHand; slot <= EquipmentBody; slot++ {
			if !mob.Equipment(slot).StackableWith(stack) || mob.EquipmentDropChance(slot) != float32(slot)/3 {
				t.Fatal("saved equipment changed")
			}
		}
		again, err := mob.PersistentNBT()
		if err != nil || !reflect.DeepEqual(raw, again) {
			for _, key := range raw.Keys() {
				before, _, _ := raw.Get(key)
				after, _, _ := again.Get(key)
				if !reflect.DeepEqual(before, after) {
					a, _ := nbt.ToJSON(before)
					b, _ := nbt.ToJSON(after)
					t.Errorf("%s: %s -> %s", key, a, b)
				}
			}
			t.Fatalf("equipment NBT changed: %v", err)
		}
	}
}

// TestMobEquipmentRejectsMalformedStateAtomically covers slot, component and persistence bounds.
func TestMobEquipmentRejectsMalformedStateAtomically(t *testing.T) {
	mob := NewMobEntityByName(1, protocol.UUID{3}, "skeleton", Vec3{})
	invalid := inventory.NewSlot(item.IDByName("minecraft:stone_sword"), 1)
	invalid.SetComponent(dc.ComponentAttributeModifiers, inventory.StructuredComponent{Data: map[string]any{"attributes": []any{map[string]any{"typeId": attr.AttrAttackDamage, "name": "example:invalid", "value": math.Inf(1), "operation": "add", "slot": "main_hand"}}, "showTooltip": true}})
	for _, stack := range []inventory.ItemStack{invalid, inventory.NewSlot(-1, 1), inventory.NewSlot(1, -1), inventory.NewSlot(item.IDByName("minecraft:bow"), 2)} {
		if mob.SetEquipment(EquipmentMainHand, stack) == nil || !mob.UsesBow() || mob.Attributes().GetValue(attr.AttrAttackDamage) != 2 {
			t.Fatal("malformed gear mutated accepted state")
		}
	}
	if mob.SetEquipment(7, inventory.EmptySlot()) == nil || mob.SetEquipmentDropChance(EquipmentHead, float32(math.NaN())) == nil {
		t.Fatal("slot or chance validation bypassed")
	}
	raw, err := mob.PersistentNBT()
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*nbt.Compound){
		func(root *nbt.Compound) { root.PutList("HandItems", nbt.NewList(nbt.TagCompound)) },
		func(root *nbt.Compound) {
			list := nbt.NewList(nbt.TagFloat)
			list.Add(float32(0))
			list.Add(float32(math.Inf(1)))
			root.PutList("HandDropChances", list)
		},
		func(root *nbt.Compound) { root.PutFloat("body_armor_drop_chance", -1) },
	} {
		copy := raw.Clone()
		mutate(copy)
		if _, err := DecodePersistentEntity(2, copy); err == nil {
			t.Fatal("malformed saved equipment accepted")
		}
	}
}

// TestEquipmentDeathRulesPreserveForcedDropsAndBoundExperience verifies credit, curse and Looting.
func TestEquipmentDeathRulesPreserveForcedDropsAndBoundExperience(t *testing.T) {
	newMob := func(chance float32) *MobEntity {
		mob := NewMobEntityByName(1, protocol.UUID{4}, "skeleton", Vec3{})
		stack := inventory.NewSlot(item.IDByName("minecraft:diamond_sword"), 1)
		stack.SetDamage(7)
		if err := mob.SetEquipment(EquipmentMainHand, stack); err != nil {
			t.Fatal(err)
		}
		if err := mob.SetEquipmentDropChance(EquipmentMainHand, chance); err != nil {
			t.Fatal(err)
		}
		mob.SetHealth(0)
		return mob
	}
	rng := rand.New(rand.NewSource(42))
	forced := newMob(2)
	drops := forced.RollEquipmentDrops(rng, false, 0)
	if len(drops) != 1 || drops[0].Damage != 7 || !forced.Equipment(EquipmentMainHand).Empty() || forced.EquipmentXPBonus(rng) != 0 || len(forced.RollEquipmentDrops(rng, true, 0)) != 0 {
		t.Fatal("forced drop damaged, repeated or rewarded twice")
	}
	ordinary := newMob(.5)
	if len(ordinary.RollEquipmentDrops(rng, false, 255)) != 0 {
		t.Fatal("uncredited ordinary equipment dropped")
	}
	if bonus := ordinary.EquipmentXPBonus(rng); bonus < 1 || bonus > 3 {
		t.Fatal("gear experience bonus missing")
	}
	if drops := ordinary.RollEquipmentDrops(rng, true, 255); len(drops) != 1 || drops[0].Damage < 3 || drops[0].Damage > 1561 {
		t.Fatal("Looting or randomized durability missing")
	}
	zero := newMob(0)
	if len(zero.RollEquipmentDrops(rng, true, 255)) != 0 {
		t.Fatal("Looting overrode explicit zero probability")
	}
	cursed := newMob(2)
	stack := cursed.Equipment(EquipmentMainHand)
	stack.SetEnchant(ench.EnchantmentIDByName("minecraft:vanishing_curse"), 1)
	if err := cursed.SetEquipment(EquipmentMainHand, stack); err != nil {
		t.Fatal(err)
	}
	if len(cursed.RollEquipmentDrops(rng, true, 255)) != 0 {
		t.Fatal("Vanishing dropped forced gear")
	}
}
