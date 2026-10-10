package entity

import (
	"math"
	"testing"

	ench "github.com/vitismc/vitis/internal/data/generated/enchantment"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestArmorProtectionMatchesDamageSource prevents elemental protection from reducing melee.
func TestArmorProtectionMatchesDamageSource(t *testing.T) {
	for _, test := range []struct {
		source DamageType
		want   float32
	}{{DamageTypeMobAttack, 10}, {DamageTypeFire, 8.4}, {DamageTypeExplosion, 6.8}, {DamageTypeProjectile, 5.2}, {DamageTypeFall, 7.6}} {
		living := NewLivingEntity(NewEntity(1, protocol.UUID{}, EntityTypePlayer, Vec3{}, Vec2{}), 20)
		living.SetArmorDamageProtection(4, 8, 12, 6)
		actual := living.ApplyDamage(NewDamageSource(test.source, 10), 0, 0, 0)
		if math.Abs(float64(actual-test.want)) > 1e-5 {
			t.Fatalf("damage type %d: actual=%v want=%v", test.source, actual, test.want)
		}
	}
	for _, mode := range []int32{1, 3} {
		living := NewLivingEntity(NewEntity(1, protocol.UUID{}, EntityTypePlayer, Vec3{}, Vec2{}), 20)
		living.SetGameMode(mode)
		if living.ApplyDamage(NewDamageSource(DamageTypeExplosion, 100), 0, 0, 0) != 0 || living.IsDead() {
			t.Fatal("ordinary damage bypassed game-mode immunity")
		}
	}
}

// TestRepeatedDamageUsesStrongerHitDelta covers immunity without discarding stronger hits.
func TestRepeatedDamageUsesStrongerHitDelta(t *testing.T) {
	living := NewLivingEntity(NewEntity(1, protocol.UUID{}, EntityTypeMob, Vec3{}, Vec2{}), 20)
	for _, test := range []struct{ amount, actual, health float32 }{{8, 8, 12}, {5, 0, 12}, {10, 2, 10}} {
		actual := living.ApplyDamage(NewAttackDamageSource(DamageTypeMobAttack, test.amount, 3), 0, 0, 0)
		if actual != test.actual || living.Health() != test.health {
			t.Fatalf("damage %v: actual %v, health %v", test.amount, actual, living.Health())
		}
	}
	for range 11 {
		living.TickLiving()
	}
	if living.ApplyDamage(NewDamageSource(DamageTypeFall, 3), 0, 0, 0) != 3 {
		t.Fatal("new hit remained invulnerable after ten ticks")
	}
	for _, invalid := range []float32{float32(math.NaN()), float32(math.Inf(1)), -1} {
		if living.ApplyDamage(NewDamageSource(DamageTypeVoid, invalid), 0, 0, 0) != 0 {
			t.Fatal("accepted invalid damage")
		}
	}
}

// TestEnvironmentalDeathClearsWeaponButRetainsKillCredit separates death cause from reward eligibility.
func TestEnvironmentalDeathClearsWeaponButRetainsKillCredit(t *testing.T) {
	mob := NewMobEntityByName(1, protocol.UUID{1}, "cow", Vec3{Y: 64})
	weapon := inventory.NewSlot(item.IDByName("minecraft:diamond_sword"), 1)
	source := NewAttackDamageSource(DamageTypeProjectile, 2, 99)
	source.PlayerCredit, source.AttackerType, source.Weapon = true, "minecraft:player", weapon
	source.DirectID, source.DirectType = 100, "minecraft:arrow"
	mob.ApplyDamage(source, 0, 0, 0)
	root, err := mob.PersistentNBT()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := DecodePersistentEntity(2, root)
	if err != nil {
		t.Fatal(err)
	}
	restored := loaded.(*MobEntity)
	if restored.LastAttackerType() != "minecraft:player" || restored.LastDirectAttackerType() != "minecraft:arrow" {
		t.Fatal("shooter and projectile became the same death entity after reload")
	}
	restored.ApplyDamage(NewDamageSource(DamageTypeFire, 100), 0, 0, 0)
	if !restored.IsDead() || !restored.HasPlayerKillCredit() || !restored.DeathWeapon().Empty() || restored.LastAttackerType() != "" || restored.LastDirectAttackerType() != "" {
		t.Fatal("environmental death reused an earlier attack's loot context")
	}
}

// TestKillCreditWeaponAndPersistence retains death context independently of live inventory.
func TestKillCreditWeaponAndPersistence(t *testing.T) {
	mob := NewMobEntityByName(1, protocol.UUID{1}, "cow", Vec3{Y: 64})
	mob.SetPersistenceRequired(true)
	weapon := inventory.NewSlot(item.IDByName("minecraft:diamond_sword"), 1)
	looting := ench.EnchantmentIDByName("minecraft:looting")
	weapon.SetEnchant(looting, 3)
	source := NewAttackDamageSource(DamageTypePlayerAttack, 2, 99)
	source.AttackerType, source.Weapon = "minecraft:player", weapon
	mob.ApplyDamage(source, 0, 0, 0)
	weapon.SetEnchant(looting, 1)
	if !mob.HasPlayerKillCredit() || mob.DeathWeapon().EnchantLevel(looting) != 3 {
		t.Fatal("kill credit retained mutable inventory state")
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
	if !restored.PersistenceRequired() || !restored.HasPlayerKillCredit() || restored.DeathWeapon().EnchantLevel(looting) != 3 || restored.LastAttackerType() != "minecraft:player" {
		t.Fatal("death context did not survive reload")
	}
	for range 100 {
		restored.TickLiving()
	}
	if restored.HasPlayerKillCredit() {
		t.Fatal("player kill credit never expired")
	}
}

// TestDeathRewardsAreIssuedOnceAndNeverForDespawn verifies the manager's death boundary.
func TestDeathRewardsAreIssuedOnceAndNeverForDespawn(t *testing.T) {
	manager := NewManager()
	mobs := NewMobManager(manager, manager.AllocateID)
	deaths := 0
	mobs.SetDeathLootHandler(func(mob *MobEntity) []MobDeathDrop { deaths++; return []MobDeathDrop{{XP: 5}} })
	mob := mobs.SpawnMob("zombie", Vec3{Y: 64})
	mob.Damage(100, "generic")
	if len(mobs.Tick(nil)) != 1 {
		t.Fatal("missing initial death rewards")
	}
	for range 30 {
		if len(mobs.Tick(nil)) != 0 {
			t.Fatal("duplicate death rewards")
		}
	}
	despawned := mobs.SpawnMob("zombie", Vec3{Y: 64})
	despawned.Remove()
	if len(mobs.Tick(nil)) != 0 || deaths != 1 {
		t.Fatal("despawn awarded death rewards")
	}
}

// TestCountedOrbsMergeAndConsumeEqualAwards preserves denominations and prevents duplicate pickups.
func TestCountedOrbsMergeAndConsumeEqualAwards(t *testing.T) {
	orb := NewXPOrb(1, protocol.UUID{1}, Vec3{Y: 64}, 7)
	if orb.TryMerge(orb) || orb.Count() != 1 {
		t.Fatal("orb merged its own experience")
	}
	other := NewXPOrb(41, protocol.UUID{2}, Vec3{Y: 64}, 7)
	if !orb.TryMerge(other) || orb.Count() != 2 || orb.XPValue() != 7 {
		t.Fatal("merge changed the XP denomination")
	}
	if orb.TryMerge(NewXPOrb(2, protocol.UUID{}, Vec3{}, 3)) {
		t.Fatal("merged distinct denominations")
	}
	root, err := orb.PersistentNBT()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := DecodePersistentEntity(3, root)
	if err != nil {
		t.Fatal(err)
	}
	orb = loaded.(*XPOrb)
	if orb.TakeOne() != 7 || orb.Removed() || orb.Count() != 1 {
		t.Fatal("first award consumed the entire merged orb")
	}
	if orb.TakeOne() != 7 || !orb.Removed() || orb.TakeOne() != 0 {
		t.Fatal("exhausted orb allowed another pickup")
	}
	root.PutShort("Age", -1)
	if _, err := DecodePersistentEntity(4, root); err == nil {
		t.Fatal("accepted negative orb age")
	}
}

// TestOrbManagerDeletesMergedEntitiesImmediately avoids iteration-order dependent duplicate visibility.
func TestOrbManagerDeletesMergedEntitiesImmediately(t *testing.T) {
	manager := NewXPOrbManager()
	for _, id := range []int32{1, 41} {
		orb := NewXPOrb(id, protocol.UUID{uint64(id)}, Vec3{Y: 64}, 7)
		orb.age, orb.vel = 19, Vec3{}
		manager.Add(orb)
	}
	removed := manager.TickAll(&simulationTestWorld{active: true}, nil)
	if len(removed) != 1 || manager.Count() != 1 {
		t.Fatalf("merged removals = %v, count = %d", removed, manager.Count())
	}
	for _, orb := range manager.All() {
		if orb.Count() != 2 {
			t.Fatal("merge lost awards")
		}
	}
}
