package world

import (
	"testing"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/projectile"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestCombatDifficultyScalesPlayersWithoutChangingSharedSource exercises melee, arrows and explosions.
func TestCombatDifficultyScalesPlayersWithoutChangingSharedSource(t *testing.T) {
	for _, test := range []struct {
		difficulty int32
		damage     float32
	}{{0, 0}, {1, 5}, {2, 8}, {3, 12}} {
		for _, kind := range []string{"melee", "arrow", "explosion"} {
			w, player, _, _ := survivalTestWorld(t, nil)
			w.SetDifficulty(test.difficulty)
			mob := w.SpawnMob("wither_skeleton", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
			source := entity.NewAttackDamageSource(entity.DamageTypeProjectile, 8, mob.ID())
			source.AttackerType = "minecraft:wither_skeleton"
			switch kind {
			case "melee":
				w.AttackMobTarget(mob, player.ID(), false)
			case "arrow":
				arrow := projectile.NewArrow(w.entities.AllocateID(), protocol.GenerateUUID(), mob.Position(), entity.Vec3{X: -1}, mob.ID(), mob.UUID())
				w.SpawnArrow(arrow)
				source.DirectID, source.DirectType = arrow.ID(), "minecraft:arrow"
				w.applyProjectileDamage(player.ID(), source, mob.ID())
			case "explosion":
				w.applyExplosionDamage(entity.ExplosionEntityEffect{EntityID: player.ID(), Damage: 8})
			}
			if player.Health() != 20-test.damage || source.Amount != 8 {
				t.Fatalf("difficulty=%d kind=%s health=%v amount=%v", test.difficulty, kind, player.Health(), source.Amount)
			}
			if kind == "arrow" {
				cow := w.SpawnMob("cow", entity.Vec3{X: 6.5, Y: 64, Z: 4.5})
				w.applyProjectileDamage(cow.ID(), source, mob.ID())
				if cow.Health() != 2 {
					t.Fatal("player difficulty leaked into damage against a mob")
				}
			}
		}
	}
	w, player, _, _ := survivalTestWorld(t, nil)
	w.SetDifficulty(3)
	source := entity.NewAttackDamageSource(entity.DamageTypeProjectile, 4, player.ID())
	w.applyProjectileDamage(player.ID(), source, player.ID())
	if player.Health() != 16 {
		t.Fatal("player-fired projectile scaled with difficulty")
	}
}
