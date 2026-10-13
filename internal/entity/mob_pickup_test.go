package entity

import (
	"math"
	"math/rand"
	"testing"

	attr "github.com/vitismc/vitis/internal/data/generated/attribute"
	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	ench "github.com/vitismc/vitis/internal/data/generated/enchantment"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestMobPickupEquipsFullStacksAndPersistsGuarantees checks transaction ownership and restoration.
func TestMobPickupEquipsFullStacksAndPersistsGuarantees(t *testing.T) {
	mob := NewMobEntityByName(1, protocol.UUID{1}, "zombie", Vec3{})
	mob.SetNoAI(true)
	mob.SetCanPickUpLoot(true)
	mob.SetNoActionTime(500)
	stack := inventory.NewSlot(item.IDByName("minecraft:diamond_sword"), 1)
	stack.SetDamage(7)
	stack.SetComponent(dc.ComponentCustomData, inventory.NBTComponent{Value: nbt.NewCompound().PutString("identity", "pickup")})
	picked, replaced, err := mob.EquipPickedItem(stack, nil, rand.New(rand.NewSource(1)))
	if err != nil || !picked.StackableWith(stack) || !replaced.Empty() || !mob.PersistenceRequired() || mob.NoActionTime() != 0 || mob.EquipmentDropChance(EquipmentMainHand) != 2 || mob.Attributes().GetValue(attr.AttrAttackDamage) != 9 {
		t.Fatalf("picked equipment did not become persistent: %v", err)
	}
	picked.SetDamage(50)
	stack.SetDamage(80)
	if mob.Equipment(EquipmentMainHand).Damage != 7 {
		t.Fatal("equipment shares mutable input or returned components")
	}
	raw, err := mob.PersistentNBT()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := DecodePersistentEntity(2, raw)
	if err != nil {
		t.Fatal(err)
	}
	saved := restored.(*MobEntity)
	if !saved.CanPickUpLoot() || !saved.PersistenceRequired() || saved.Equipment(EquipmentMainHand).Damage != 7 || saved.EquipmentDropChance(EquipmentMainHand) != 2 || saved.Attributes().GetValue(attr.AttrAttackDamage) != 9 {
		t.Fatal("pickup state was lost or modifiers accumulated on restart")
	}
	raw.PutString("CanPickUpLoot", "invalid")
	if _, err := DecodePersistentEntity(3, raw); err == nil {
		t.Fatal("malformed pickup capability was restored")
	}
}

// TestMobPickupReplacementUsesSpeciesAndArmorRules checks preferences, binding and fallback slots.
func TestMobPickupReplacementUsesSpeciesAndArmorRules(t *testing.T) {
	for _, test := range []struct {
		species, candidate string
		accepted           bool
	}{{"skeleton", "diamond_sword", false}, {"stray", "netherite_sword", false}, {"wither_skeleton", "bow", false}, {"wither_skeleton", "crossbow", false}, {"wither_skeleton", "diamond_sword", true}, {"zombie", "diamond_sword", true}} {
		mob := NewMobEntityByName(1, protocol.UUID{1}, test.species, Vec3{})
		mob.SetCanPickUpLoot(true)
		picked, _, err := mob.EquipPickedItem(inventory.NewSlot(item.IDByName("minecraft:"+test.candidate), 1), nil, rand.New(rand.NewSource(1)))
		if err != nil || !picked.Empty() != test.accepted {
			t.Fatalf("%s selected %s: %v", test.species, test.candidate, err)
		}
	}
	mob := NewMobEntityByName(1, protocol.UUID{1}, "zombie", Vec3{})
	mob.SetCanPickUpLoot(true)
	bound := inventory.NewSlot(item.IDByName("minecraft:iron_helmet"), 1)
	bound.SetEnchant(ench.EnchantmentIDByName("minecraft:binding_curse"), 1)
	if err := mob.SetEquipment(EquipmentHead, bound); err != nil {
		t.Fatal(err)
	}
	diamond := inventory.NewSlot(item.IDByName("minecraft:diamond_helmet"), 1)
	picked, _, err := mob.EquipPickedItem(diamond, nil, rand.New(rand.NewSource(1)))
	if err != nil || picked.Empty() || !mob.Equipment(EquipmentHead).StackableWith(bound) || !mob.Equipment(EquipmentMainHand).StackableWith(diamond) {
		t.Fatal("bound armor changed or empty-hand armor fallback was lost")
	}
	if picked, _, err := mob.EquipPickedItem(diamond, nil, rand.New(rand.NewSource(1))); err != nil || !picked.Empty() {
		t.Fatal("bound armor was replaced with an occupied fallback hand")
	}
	drowned := NewMobEntityByName(2, protocol.UUID{2}, "drowned", Vec3{})
	drowned.SetCanPickUpLoot(true)
	if err := drowned.SetEquipment(EquipmentMainHand, inventory.NewSlot(item.IDByName("minecraft:trident"), 1)); err != nil {
		t.Fatal(err)
	}
	if picked, _, err := drowned.EquipPickedItem(inventory.NewSlot(item.IDByName("minecraft:netherite_sword"), 1), nil, rand.New(rand.NewSource(1))); err != nil || !picked.Empty() {
		t.Fatal("drowned replaced its preferred trident")
	}
}

// TestMobPickupLimitsArmorCountsAndRejectsMalformedChanges checks partial transfers and atomic failure.
func TestMobPickupLimitsArmorCountsAndRejectsMalformedChanges(t *testing.T) {
	mob := NewMobEntityByName(1, protocol.UUID{1}, "zombie", Vec3{})
	mob.SetCanPickUpLoot(true)
	stack := inventory.NewSlot(item.IDByName("minecraft:iron_helmet"), 5)
	stack.RemoveComponent(dc.ComponentMaxDamage)
	stack.RemoveComponent(dc.ComponentDamage)
	stack.SetComponent(dc.ComponentMaxStackSize, inventory.VarIntComponent(16))
	picked, _, err := mob.EquipPickedItem(stack, nil, rand.New(rand.NewSource(1)))
	if err != nil || picked.ItemCount != 1 || mob.Equipment(EquipmentHead).ItemCount != 1 || stack.ItemCount != 5 {
		t.Fatalf("armor pickup count incorrect: %v", err)
	}
	invalid := inventory.NewSlot(item.IDByName("minecraft:diamond_sword"), 1)
	invalid.SetComponent(dc.ComponentAttributeModifiers, inventory.StructuredComponent{Data: map[string]any{"attributes": []any{map[string]any{"typeId": attr.AttrAttackDamage, "name": "example:bad", "value": math.Inf(1), "operation": "add", "slot": "main_hand"}}, "showTooltip": true}})
	for _, stack := range []inventory.ItemStack{invalid, inventory.NewSlot(item.IDByName("minecraft:diamond_sword"), 2)} {
		before := mob.Equipment(EquipmentMainHand)
		picked, replaced, err := mob.EquipPickedItem(stack, nil, rand.New(rand.NewSource(1)))
		if err == nil || !picked.Empty() || !replaced.Empty() || !mob.Equipment(EquipmentMainHand).StackableWith(before) {
			t.Fatal("failed pickup published equipment or a replacement drop")
		}
	}
}

// TestMobPickupTieBreakersAndReplacementDrops checks distinct enchantments, damage and guaranteed old gear.
func TestMobPickupTieBreakersAndReplacementDrops(t *testing.T) {
	mob := NewMobEntityByName(1, protocol.UUID{1}, "zombie", Vec3{})
	mob.SetCanPickUpLoot(true)
	old := inventory.NewSlot(item.IDByName("minecraft:diamond_sword"), 1)
	old.SetDamage(20)
	old.SetEnchant(ench.EnchantmentIDByName("minecraft:sharpness"), 5)
	if err := mob.SetEquipment(EquipmentMainHand, old); err != nil {
		t.Fatal(err)
	}
	if err := mob.SetEquipmentDropChance(EquipmentMainHand, 2); err != nil {
		t.Fatal(err)
	}
	better := old.Clone()
	better.SetDamage(30)
	better.SetEnchant(ench.EnchantmentIDByName("minecraft:unbreaking"), 1)
	picked, replaced, err := mob.EquipPickedItem(better, nil, rand.New(rand.NewSource(1)))
	if err != nil || !picked.StackableWith(better) || !replaced.StackableWith(old) {
		t.Fatal("distinct-enchantment tie breaker or guaranteed replacement drop failed")
	}
	better.SetDamage(10)
	if picked, _, err := mob.EquipPickedItem(better, nil, rand.New(rand.NewSource(1))); err != nil || picked.Empty() {
		t.Fatal("equal enchantments ignored lower damage")
	}
	mob.SetCanPickUpLoot(false)
	if picked, _, err := mob.EquipPickedItem(inventory.NewSlot(item.IDByName("minecraft:netherite_sword"), 1), nil, rand.New(rand.NewSource(1))); err != nil || !picked.Empty() {
		t.Fatal("disabled pickup capability equipped an item")
	}
}

// TestMobPickupScoresSlotModifiersInComponentOrder covers approximate selection and actual attributes separately.
func TestMobPickupScoresSlotModifiersInComponentOrder(t *testing.T) {
	mob := NewMobEntityByName(1, protocol.UUID{1}, "zombie", Vec3{})
	mob.SetCanPickUpLoot(true)
	if err := mob.SetEquipment(EquipmentMainHand, inventory.NewSlot(item.IDByName("minecraft:netherite_sword"), 1)); err != nil {
		t.Fatal(err)
	}
	stack := inventory.NewSlot(item.IDByName("minecraft:stick"), 1)
	stack.SetComponent(dc.ComponentAttributeModifiers, inventory.StructuredComponent{Data: map[string]any{"attributes": []any{
		map[string]any{"typeId": attr.AttrArmor, "name": "example:armor", "value": float64(2), "operation": "add", "slot": "main_hand"},
		map[string]any{"typeId": attr.AttrArmorToughness, "name": "example:toughness", "value": .5, "operation": "multiply_base", "slot": "main_hand"},
		map[string]any{"typeId": attr.AttrKnockbackResistance, "name": "example:knockback", "value": float64(1), "operation": "multiply_total", "slot": "main_hand"},
		map[string]any{"typeId": attr.AttrAttackDamage, "name": "example:other_slot", "value": float64(999), "operation": "add", "slot": "off_hand"},
	}, "showTooltip": true}})
	if score := approximatePickedAttribute(stack, EquipmentMainHand, 3); score != 13 {
		t.Fatalf("ordered approximate score=%v", score)
	}
	if picked, _, err := mob.EquipPickedItem(stack, nil, rand.New(rand.NewSource(1))); err != nil || picked.Empty() || mob.Attributes().GetValue(attr.AttrAttackDamage) != 3 || mob.ArmorDefense() != 4 {
		t.Fatalf("selection score was used as actual attack damage: %v", err)
	}
}
