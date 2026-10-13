package entity

import (
	"testing"

	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestStarvationDifficulty verifies the lower health bound for each difficulty.
func TestStarvationDifficulty(t *testing.T) {
	for _, tc := range []struct {
		difficulty int32
		want       float32
		dead       bool
	}{
		{1, 10, false}, {2, 1, false}, {3, 0, true},
	} {
		living := NewLivingEntity(NewEntity(1, protocol.UUID{}, EntityTypePlayer, Vec3{}, Vec2{}), 20)
		living.SetFoodLevel(0)
		living.SetHealth(11)
		for tick := 0; tick < 1000 && !living.IsDead(); tick++ {
			living.TickLiving()
			living.TickHungerWithDifficulty(tc.difficulty)
		}
		if living.Health() != tc.want || living.IsDead() != tc.dead {
			t.Errorf("difficulty %d: health=%v dead=%v, want %v/%v", tc.difficulty, living.Health(), living.IsDead(), tc.want, tc.dead)
		}
	}
}

// TestRespawnResetsSurvivalState verifies death bookkeeping and effects are cleared.
func TestRespawnResetsSurvivalState(t *testing.T) {
	living := NewLivingEntity(NewEntity(1, protocol.UUID{}, EntityTypePlayer, Vec3{}, Vec2{}), 20)
	living.SetFoodLevel(4)
	living.SetFoodExhaustion(3)
	living.SetXP(5, 55, 0.5)
	living.Effects().Add(effect.Instance{ID: effectdata.EffectPoison, Duration: 100})
	living.Damage(20, "test")
	if !living.IsDead() || living.DeathHandled() {
		t.Fatal("death was not recorded")
	}
	living.MarkDeathHandled()
	living.Respawn()
	if living.IsDead() || living.DeathHandled() || living.Health() != 20 || living.FoodLevel() != 20 || living.FoodExhaustion() != 0 || living.XPTotal() != 0 || living.Effects().Count() != 0 {
		t.Fatalf("respawn did not reset survival state: %+v", living)
	}
}

// TestRespawnKeepExperience preserves XP when the keepInventory rule is active.
func TestRespawnKeepExperience(t *testing.T) {
	living := NewLivingEntity(NewEntity(1, protocol.UUID{}, EntityTypePlayer, Vec3{}, Vec2{}), 20)
	living.SetXP(5, 55, 0.5)
	living.Damage(20, "test")
	living.RespawnWithKeepExperience(true)
	if living.IsDead() || living.XPLevel() != 5 || living.XPTotal() != 55 || living.XPBar() != 0.5 {
		t.Fatalf("keepInventory did not retain XP: level=%d total=%d bar=%v", living.XPLevel(), living.XPTotal(), living.XPBar())
	}
}

// TestPoisonCannotKillPlayer preserves the minimum one-heart-half health bound.
func TestPoisonCannotKillPlayer(t *testing.T) {
	living := NewLivingEntity(NewEntity(1, protocol.UUID{}, EntityTypePlayer, Vec3{}, Vec2{}), 20)
	living.SetHealth(1)
	living.Effects().Add(effect.Instance{ID: effectdata.EffectPoison, Duration: 25})
	living.TickEffects()
	if living.IsDead() || living.Health() != 1 {
		t.Fatalf("poison killed player: health=%v", living.Health())
	}
}

// TestDefensiveEffectsReduceDamage verifies resistance and fire immunity.
func TestDefensiveEffectsReduceDamage(t *testing.T) {
	living := NewLivingEntity(NewEntity(1, protocol.UUID{}, EntityTypePlayer, Vec3{}, Vec2{}), 20)
	living.Effects().Add(effect.Instance{ID: effectdata.EffectResistance, Duration: 100})
	if dealt := living.ApplyDamage(NewDamageSource(DamageTypeGeneric, 10), 0, 0, 0); dealt != 8 {
		t.Fatalf("resistance dealt %v, want 8", dealt)
	}
	living.TickLiving()
	for i := 0; i < 10; i++ {
		living.TickLiving()
	}
	living.Effects().Add(effect.Instance{ID: effectdata.EffectFireResistance, Duration: 100})
	if dealt := living.ApplyDamage(NewDamageSource(DamageTypeLava, 4), 0, 0, 0); dealt != 0 {
		t.Fatalf("fire resistance dealt %v, want 0", dealt)
	}
}
