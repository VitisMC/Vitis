package world

import (
	"testing"

	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	"github.com/vitismc/vitis/internal/entity"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// TestWitherSkeletonMeleeAppliesEffectOnlyOnAcceptedHits exercises actual AI, shielding and synchronization.
func TestWitherSkeletonMeleeAppliesEffectOnlyOnAcceptedHits(t *testing.T) {
	w, player, _, sender := survivalTestWorld(t, nil)
	player.SetRotation(entity.Vec2{X: -90})
	player.SetBlocking(true)
	mob := w.SpawnMob("wither_skeleton", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
	w.tickMobs()
	if player.Health() != 20 || player.Effects().Has(effectdata.EffectWither) || w.projectiles.ArrowCount() != 0 {
		t.Fatal("shielded skeleton attack applied damage, a potion effect or a shot")
	}
	player.SetBlocking(false)
	for range 20 {
		w.tickMobs()
	}
	instance := player.Effects().Get(effectdata.EffectWither)
	if player.Health() != 12 || instance == nil || instance.Duration != 200 || instance.Amplifier != 0 || w.projectiles.ArrowCount() != 0 {
		t.Fatalf("melee effect missing: health=%v wither=%+v", player.Health(), instance)
	}
	seen := false
	for _, packet := range sender.packets {
		if applied, ok := packet.(*play.EntityEffect); ok && applied.EntityID == player.ID() && applied.EffectID == effectdata.EffectWither && applied.Duration == 200 {
			seen = true
		}
	}
	if !seen {
		t.Fatal("accepted melee effect was not synchronized")
	}
	player.Effects().Remove(effectdata.EffectWither)
	if w.AttackMobTarget(mob, player.ID(), false) || player.Effects().Has(effectdata.EffectWither) {
		t.Fatal("rejected cooldown hit reissued Wither")
	}
	mob.Remove()
	if w.AttackMobTarget(mob, player.ID(), false) || player.Effects().Has(effectdata.EffectWither) {
		t.Fatal("removed skeleton applied Wither")
	}
}

// TestCaveSpiderPoisonDependsOnDifficulty checks melee acceptance and duration at every difficulty.
func TestCaveSpiderPoisonDependsOnDifficulty(t *testing.T) {
	for _, test := range []struct {
		difficulty int32
		duration   int32
	}{{0, 0}, {1, 0}, {2, 140}, {3, 300}} {
		w, player, _, _ := survivalTestWorld(t, nil)
		w.SetDifficulty(test.difficulty)
		mob := w.SpawnMob("cave_spider", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
		accepted := w.AttackMobTarget(mob, player.ID(), false)
		if accepted != (test.difficulty != 0) {
			t.Fatal("cave spider failed accepted melee")
		}
		instance := player.Effects().Get(effectdata.EffectPoison)
		if test.duration == 0 {
			if instance != nil {
				t.Fatal("poison applied below Normal")
			}
		} else if instance == nil || instance.Duration != test.duration {
			t.Fatalf("difficulty=%d effect=%+v", test.difficulty, instance)
		}
		player.Effects().Remove(effectdata.EffectPoison)
		if w.AttackMobTarget(mob, player.ID(), false) || player.Effects().Has(effectdata.EffectPoison) {
			t.Fatal("rejected hit applied poison")
		}
	}
}
