package entity

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestFireImmuneSpeciesRejectIgnitionAndFireDamage checks immunity independently of AI and restoration.
func TestFireImmuneSpeciesRejectIgnitionAndFireDamage(t *testing.T) {
	for _, name := range []string{"blaze", "wither_skeleton"} {
		mob := NewMobEntityByName(1, protocol.UUID{1}, name, Vec3{Y: 64})
		mob.SetFireTicks(160)
		for _, kind := range []DamageType{DamageTypeFire, DamageTypeFireTick, DamageTypeLava} {
			if mob.ApplyDamage(NewDamageSource(kind, 100), 0, 0, 0) != 0 || mob.Health() != 20 || mob.FireTicks() != 0 {
				t.Fatalf("%s accepted fire source %d", name, kind)
			}
		}
		for _, name := range []string{"minecraft:fire", "minecraft:lava"} {
			state := block.DefaultStateID(name)
			damages := mob.TickEnvironment(func(int, int, int) int32 { return state })
			if len(damages) != 0 || mob.FireTicks() != 0 {
				t.Fatal("resident fire or lava ignited an immune species")
			}
		}
		root, err := mob.PersistentNBT()
		if err != nil {
			t.Fatal(err)
		}
		root.PutShort("Fire", 160)
		loaded, err := DecodePersistentEntity(2, root)
		if err != nil {
			t.Fatal(err)
		}
		restored := loaded.(*MobEntity)
		if restored.FireTicks() != 0 || restored.ApplyDamage(NewDamageSource(DamageTypeLava, 100), 0, 0, 0) != 0 {
			t.Fatal("restored species lost fire immunity")
		}
		if restored.ApplyDamage(NewDamageSource(DamageTypeFall, 1), 0, 0, 0) != 1 {
			t.Fatal("fire immunity suppressed unrelated damage")
		}
	}
	zombie := NewMobEntityByName(3, protocol.UUID{3}, "zombie", Vec3{Y: 64})
	zombie.SetFireTicks(160)
	if zombie.FireTicks() != 160 || zombie.ApplyDamage(NewDamageSource(DamageTypeFire, 2), 0, 0, 0) != 2 {
		t.Fatal("ordinary species acquired fire immunity")
	}
}
