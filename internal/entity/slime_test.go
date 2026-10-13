package entity

import (
	"math"
	"testing"

	genattr "github.com/vitismc/vitis/internal/data/generated/attribute"
	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestSlimeSizeControlsInstancePropertiesAndPersistence checks bounds, isolation and saved health.
func TestSlimeSizeControlsInstancePropertiesAndPersistence(t *testing.T) {
	base := *GetMobTypeDef("slime")
	for _, requested := range []int{-1, 1, 2, 4, 127, 128} {
		mob := NewMobEntityByName(1, protocol.UUID{1}, "slime", Vec3{Y: 64})
		mob.SetSlimeSize(requested, true)
		size := max(1, min(127, requested))
		_, _, _, lootSize := mob.LootTraits()
		if mob.SlimeSize() != size || lootSize != size || mob.Health() != float32(size*size) || mob.XPReward() != int32(size) || mob.TypeDef().Width != base.Width*float64(size) || mob.TypeDef().Height != base.Height*float64(size) || mob.Attributes().GetBase(genattr.AttrAttackDamage) != float64(size) {
			t.Fatalf("size %d: inconsistent properties", requested)
		}
		mob.ApplyDamage(NewDamageSource(DamageTypeGeneric, .5), 0, 0, 0)
		root, err := mob.PersistentNBT()
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := DecodePersistentEntity(2, root)
		if err != nil {
			t.Fatal(err)
		}
		restored := loaded.(*MobEntity)
		if restored.SlimeSize() != size || restored.Health() != mob.Health() || restored.MaxHealth() != float32(size*size) || restored.TypeDef().Width != mob.TypeDef().Width {
			t.Fatalf("size %d changed after reload", size)
		}
	}
	if GetMobTypeDef("slime").Width != base.Width || GetMobTypeDef("slime").MaxHealth != base.MaxHealth {
		t.Fatal("instance size mutated shared type definitions")
	}
}

// TestSlimePersistenceRejectsInvalidSize prevents malformed NBT from changing collision bounds.
func TestSlimePersistenceRejectsInvalidSize(t *testing.T) {
	mob := NewMobEntityByName(1, protocol.UUID{1}, "slime", Vec3{Y: 64})
	root, err := mob.PersistentNBT()
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*nbt.Compound){
		func(raw *nbt.Compound) { raw.PutInt("Size", -1) },
		func(raw *nbt.Compound) { raw.PutInt("Size", math.MaxInt32) },
		func(raw *nbt.Compound) { raw.PutByte("Size", 3) },
	} {
		copy := root.Clone()
		mutate(copy)
		if _, err := DecodePersistentEntity(2, copy); err == nil {
			t.Fatal("malformed slime size was accepted")
		}
	}
}

// TestSlimeSplitsOnceWithoutInheritingDeathRewards verifies child lifecycle and supported conversion state.
func TestSlimeSplitsOnceWithoutInheritingDeathRewards(t *testing.T) {
	manager := NewManager()
	mobs := NewMobManager(manager, manager.AllocateID)
	spawned, deaths := 0, 0
	mobs.SetSpawnHandler(func(*MobEntity) { spawned++ })
	mobs.SetDeathLootHandler(func(mob *MobEntity) []MobDeathDrop { deaths++; return []MobDeathDrop{{XP: mob.XPReward()}} })
	parent := mobs.SpawnMob("slime", Vec3{Y: 64})
	parent.SetSlimeSize(4, true)
	parent.SetNoAI(true)
	parent.SetPersistenceRequired(true)
	parent.Effects().Add(effect.Instance{ID: effectdata.EffectSpeed, Duration: 100, Amplifier: 1})
	parent.persistent = nbt.NewCompound().PutString("CustomName", `{"text":"named"}`).PutInt("unrelated", 9)
	source := NewAttackDamageSource(DamageTypePlayerAttack, 100, 99)
	source.AttackerType = "minecraft:player"
	parent.ApplyDamage(source, 0, 0, 0)
	for range 30 {
		mobs.Tick(nil)
	}
	children := mobs.Count()
	if children < 2 || children > 4 || deaths != 1 || spawned != children+1 || mobs.Get(parent.ID()) != nil {
		t.Fatalf("children=%d deaths=%d spawns=%d", children, deaths, spawned)
	}
	for _, child := range mobs.Mobs() {
		if child.SlimeSize() != 2 || child.Health() != 4 || child.IsDead() || child.DeathHandled() || child.HasPlayerKillCredit() || !child.DeathWeapon().Empty() || child.NoActionTime() != 0 || !child.PersistenceRequired() || !child.NoAI() || !child.Effects().Has(effectdata.EffectSpeed) {
			t.Fatal("child inherited death state or lost conversion state")
		}
		if _, _, exists := child.persistent.Get("unrelated"); exists {
			t.Fatal("child inherited unrelated parent NBT")
		}
		root, err := child.PersistentNBT()
		if err != nil {
			t.Fatal(err)
		}
		if name, _ := root.GetString("CustomName"); name != `{"text":"named"}` {
			t.Fatal("child lost its saved custom name")
		}
		if _, err := DecodePersistentEntity(100, root); err != nil {
			t.Fatal(err)
		}
	}
	for range 30 {
		mobs.Tick(nil)
	}
	if mobs.Count() != children || spawned != children+1 || deaths != 1 {
		t.Fatal("completed death split ran again")
	}
}

// TestSlimeRemovalAndTinyDeathNeverSplit covers despawn, eviction and smallest-size death.
func TestSlimeRemovalAndTinyDeathNeverSplit(t *testing.T) {
	for _, mode := range []string{"despawn", "eviction", "discard dead", "tiny death"} {
		t.Run(mode, func(t *testing.T) {
			manager := NewManager()
			mobs := NewMobManager(manager, manager.AllocateID)
			mob := mobs.SpawnMob("slime", Vec3{Y: 64})
			mob.SetSlimeSize(4, true)
			deaths := 0
			mobs.SetDeathLootHandler(func(*MobEntity) []MobDeathDrop { deaths++; return nil })
			switch mode {
			case "despawn":
				mob.Remove()
			case "eviction":
				mobs.Evict(mob.ID())
			case "discard dead":
				mob.Damage(100, "generic")
				mob.Remove()
			case "tiny death":
				mob.SetSlimeSize(1, true)
				mob.Damage(100, "generic")
			}
			for range 30 {
				mobs.Tick(nil)
			}
			if mobs.Count() != 0 || deaths != map[string]int{"tiny death": 1}[mode] {
				t.Fatalf("removed slime produced children or rewards: mobs=%d deaths=%d", mobs.Count(), deaths)
			}
		})
	}
}
