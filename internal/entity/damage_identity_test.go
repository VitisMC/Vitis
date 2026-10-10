package entity

import (
	"testing"

	damagetype "github.com/vitismc/vitis/internal/data/generated/damage_type"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestEffectDamageAliasesPreserveRegistryIdentity keeps poison and magic damage consistent with loot tags.
func TestEffectDamageAliasesPreserveRegistryIdentity(t *testing.T) {
	for _, source := range []string{"poison", "instant_damage"} {
		mob := NewMobEntityByName(1, protocol.UUID{1}, "cow", Vec3{Y: 64})
		mob.Damage(1, source)
		if mob.LastDamageType() != DamageTypeMagic || mob.LastDamageRegistryName() != "minecraft:magic" {
			t.Fatalf("effect %s lost its damage type", source)
		}
		root, err := mob.PersistentNBT()
		if err != nil {
			t.Fatal(err)
		}
		root.Delete("vitis:last_damage_type")
		root.Delete("vitis:last_damage_registry")
		root.Delete("vitis:last_damage_direct")
		loaded, err := DecodePersistentEntity(2, root)
		if err != nil || loaded.(*MobEntity).LastDamageType() != DamageTypeMagic {
			t.Fatalf("legacy effect %s: %v", source, err)
		}
	}
}

// TestDamageIdentityPersistsAllSupportedCauses preserves loot predicates across unload and restart.
func TestDamageIdentityPersistsAllSupportedCauses(t *testing.T) {
	for kind := DamageTypeGeneric; kind <= DamageTypeSweetBerryBush; kind++ {
		mob := NewMobEntityByName(1, protocol.UUID{1}, "cow", Vec3{Y: 64})
		if mob.ApplyDamage(NewDamageSource(kind, 1), 0, 0, 0) <= 0 {
			t.Fatalf("damage %d was not accepted", kind)
		}
		root, err := mob.PersistentNBT()
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := DecodePersistentEntity(2, root)
		if err != nil {
			t.Fatal(err)
		}
		restored := loaded.(*MobEntity)
		if restored.LastDamageType() != kind || restored.LastDamageRegistryName() != kind.RegistryName() || !restored.LastDamageIsDirect() || damagetype.DamageTypeIDByName(restored.LastDamageRegistryName()) < 0 {
			t.Fatalf("damage %d changed after restoration", kind)
		}
		root.Delete("vitis:last_damage_type")
		root.Delete("vitis:last_damage_registry")
		loaded, err = DecodePersistentEntity(3, root)
		if err != nil || loaded.(*MobEntity).LastDamageType() != kind || !loaded.(*MobEntity).LastDamageIsDirect() {
			t.Fatalf("legacy damage %d: %v", kind, err)
		}
	}
}

// TestRejectedHitDoesNotReplaceDeathCause prevents weaker cooldown hits from changing loot context.
func TestRejectedHitDoesNotReplaceDeathCause(t *testing.T) {
	mob := NewMobEntityByName(1, protocol.UUID{1}, "cow", Vec3{Y: 64})
	arrow := NewAttackDamageSource(DamageTypeProjectile, 5, 99)
	arrow.DirectID, arrow.DirectType, arrow.AttackerType = 100, "minecraft:arrow", "minecraft:skeleton"
	if mob.ApplyDamage(arrow, 0, 0, 0) != 5 {
		t.Fatal("initial arrow hit was not accepted")
	}
	if mob.ApplyDamage(NewDamageSource(DamageTypeLightning, 4), 0, 0, 0) != 0 || mob.LastDamageRegistryName() != "minecraft:arrow" || mob.LastDamageIsDirect() {
		t.Fatal("rejected hit replaced the accepted cause")
	}
	if mob.ApplyDamage(NewDamageSource(DamageTypeLightning, 6), 0, 0, 0) != 1 || mob.LastDamageRegistryName() != "minecraft:lightning_bolt" || !mob.LastDamageIsDirect() {
		t.Fatal("stronger hit failed to replace the cause")
	}
}

// TestDamageRegistryVariants distinguishes thrown weapons and player explosions from arrows and mob explosions.
func TestDamageRegistryVariants(t *testing.T) {
	for _, test := range []struct {
		kind                   DamageType
		direct, attacker, want string
	}{
		{DamageTypeProjectile, "minecraft:arrow", "minecraft:skeleton", "minecraft:arrow"},
		{DamageTypeProjectile, "minecraft:trident", "minecraft:player", "minecraft:trident"},
		{DamageTypeProjectile, "minecraft:snowball", "minecraft:player", "minecraft:thrown"},
		{DamageTypeProjectile, "minecraft:small_fireball", "", "minecraft:unattributed_fireball"},
		{DamageTypeExplosion, "minecraft:tnt", "minecraft:player", "minecraft:player_explosion"},
		{DamageTypeExplosion, "minecraft:creeper", "minecraft:creeper", "minecraft:explosion"},
	} {
		mob := NewMobEntityByName(1, protocol.UUID{1}, "cow", Vec3{Y: 64})
		source := NewDamageSource(test.kind, 100)
		source.DirectType, source.AttackerType = test.direct, test.attacker
		source.DirectID = 9
		source.AttackerID = 8
		mob.ApplyDamage(source, 0, 0, 0)
		root, err := mob.PersistentNBT()
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := DecodePersistentEntity(2, root)
		if err != nil || loaded.(*MobEntity).LastDamageRegistryName() != test.want || loaded.(*MobEntity).LastDamageIsDirect() {
			t.Fatalf("variant %s/%s: %v", test.direct, test.attacker, err)
		}
	}
}

// TestPersistentDamageRejectsMalformedIdentity prevents invalid saved causes from entering simulation.
func TestPersistentDamageRejectsMalformedIdentity(t *testing.T) {
	mob := NewMobEntityByName(1, protocol.UUID{1}, "cow", Vec3{Y: 64})
	for _, invalid := range []DamageType{-1, 22} {
		if mob.ApplyDamage(NewDamageSource(invalid, 100), 0, 0, 0) != 0 || mob.IsDead() {
			t.Fatal("invalid damage type entered combat state")
		}
	}
	root, err := mob.PersistentNBT()
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*nbt.Compound){
		func(raw *nbt.Compound) { raw.PutInt("vitis:last_damage_type", -1) },
		func(raw *nbt.Compound) { raw.PutInt("vitis:last_damage_type", 22) },
		func(raw *nbt.Compound) { raw.PutString("vitis:last_damage_type", "fall") },
		func(raw *nbt.Compound) { raw.PutString("vitis:last_damage_registry", "minecraft:missing") },
		func(raw *nbt.Compound) { raw.PutInt("vitis:last_damage_direct", 1) },
	} {
		copy := root.Clone()
		mutate(copy)
		if _, err := DecodePersistentEntity(2, copy); err == nil {
			t.Fatal("malformed saved damage identity was accepted")
		}
	}
}
