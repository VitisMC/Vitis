package ai

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/entity"
)

// actionTestWorld captures tick-owned AI actions and controls visibility and environment.
type actionTestWorld struct {
	visible, sunlit           bool
	state                     int32
	melee, arrows, explosions int
}

// GetBlockStateAt supplies one environmental block state for every position.
func (w *actionTestWorld) GetBlockStateAt(int, int, int) int32 { return w.state }

// MobCanSee supplies controlled line of sight.
func (w *actionTestWorld) MobCanSee(entity.Vec3, entity.Vec3) bool { return w.visible }

// MobSunlit supplies controlled exposure to sunlight.
func (w *actionTestWorld) MobSunlit(entity.Vec3) bool { return w.sunlit }

// MobBrightness supplies controlled daylight for species-specific goals.
func (w *actionTestWorld) MobBrightness(entity.Vec3) uint8 {
	if w.sunlit {
		return 15
	}
	return 0
}

// AttackMobTarget records melee and ranged execution after goal validation.
func (w *actionTestWorld) AttackMobTarget(mob *entity.MobEntity, target int32, ranged bool) bool {
	if ranged {
		w.arrows++
	} else {
		w.melee++
	}
	return true
}

// ExplodeMob records and removes a detonating mob without creating death rewards.
func (w *actionTestWorld) ExplodeMob(mob *entity.MobEntity, power float64) bool {
	if mob.Removed() {
		return false
	}
	w.explosions++
	mob.Remove()
	return true
}

// TestCompatibleGoalsExecuteMeleeSwimAndSunlightTogether covers independent AI controls.
func TestCompatibleGoalsExecuteMeleeSwimAndSunlightTogether(t *testing.T) {
	mob := testMob()
	mob.SetTarget(2)
	world := &actionTestWorld{visible: true, sunlit: true, state: block.DefaultStateID("minecraft:water")}
	ctx := testContext(mob, []PlayerInfo{{EntityID: 2, Pos: entity.Vec3{X: 1, Y: 64}}})
	ctx.Blocks = world
	selector := NewGoalSelector()
	selector.AddGoal(0, &SwimGoal{})
	selector.AddGoal(1, &BurnInSunlightGoal{})
	selector.AddGoal(2, &MeleeAttackGoal{AttackDist: 2, Cooldown: 20})
	for range 21 {
		selector.Tick(ctx)
	}
	if world.melee != 2 || mob.FireTicks() != 0 || mob.Velocity().Y <= 0 {
		t.Fatalf("melee=%d, fire=%d, velocity=%v", world.melee, mob.FireTicks(), mob.Velocity())
	}
	world.visible = false
	for range 30 {
		selector.Tick(ctx)
	}
	if world.melee != 2 {
		t.Fatal("melee crossed obstructed line of sight")
	}
	ctx.Players = nil
	selector.Tick(ctx)
	if mob.Target() >= 0 {
		t.Fatal("vanished target remained active")
	}
}

// TestRangedActionsRespectVisibilityCooldownAndTargetLoss verifies actual shot requests.
func TestRangedActionsRespectVisibilityCooldownAndTargetLoss(t *testing.T) {
	mob := testMob()
	mob.SetTarget(2)
	world := &actionTestWorld{visible: true}
	ctx := testContext(mob, []PlayerInfo{{EntityID: 2, Pos: entity.Vec3{X: 8, Y: 64}}})
	ctx.Blocks = world
	goal := &RangedAttackGoal{AttackDist: 15, Cooldown: 30}
	goal.Start(ctx)
	for range 31 {
		goal.Tick(ctx)
	}
	if world.arrows != 2 {
		t.Fatalf("shots=%d", world.arrows)
	}
	world.visible = false
	for range 40 {
		goal.Tick(ctx)
	}
	if world.arrows != 2 {
		t.Fatal("ranged goal fired through an obstruction")
	}
	ctx.Players = nil
	goal.Tick(ctx)
	if mob.Target() >= 0 {
		t.Fatal("ranged goal retained a disappeared target")
	}
}

// TestCreeperDetonatesOnceAndCancelsLostTargets verifies fuse completion and cancellation.
func TestCreeperDetonatesOnceAndCancelsLostTargets(t *testing.T) {
	mob := testMob()
	world := &actionTestWorld{visible: true}
	ctx := testContext(mob, []PlayerInfo{{EntityID: 2, Pos: entity.Vec3{X: 1, Y: 64}}})
	ctx.Blocks = world
	selector := NewGoalSelector()
	selector.AddGoal(1, &CreeperSwellGoal{FuseTime: 3, ExplosionPw: 3})
	selector.Tick(ctx)
	ctx.Players = nil
	selector.Tick(ctx)
	if world.explosions != 0 || mob.SwellDirection() != -1 {
		t.Fatal("lost target completed the fuse")
	}
	ctx.Players = []PlayerInfo{{EntityID: 2, Pos: entity.Vec3{X: 1, Y: 64}}}
	for range 10 {
		selector.Tick(ctx)
	}
	if world.explosions != 1 || !mob.Removed() || mob.IsDead() {
		t.Fatal("creeper did not execute one non-death explosion")
	}
}

// TestTemptationRequiresCurrentFoodAndWaterRequiresActualFluid removes placeholder environment checks.
func TestTemptationRequiresCurrentFoodAndWaterRequiresActualFluid(t *testing.T) {
	mob := entity.NewMobEntityByName(3, testMob().UUID(), "cow", entity.Vec3{Y: 64})
	ctx := testContext(mob, []PlayerInfo{{EntityID: 2, Pos: entity.Vec3{X: 2, Y: 64}}})
	goal := &TemptGoal{}
	if goal.CanStart(ctx) {
		t.Fatal("empty hand attracted a cow")
	}
	ctx.Players[0].OffhandItem = "minecraft:wheat"
	if !goal.CanStart(ctx) {
		t.Fatal("offhand wheat failed to attract a cow")
	}
	goal.Start(ctx)
	ctx.Players[0].OffhandItem = "minecraft:stone"
	if goal.CanContinue(ctx) {
		t.Fatal("cow followed a stale food snapshot")
	}
	world := &actionTestWorld{visible: true}
	ctx.Blocks = world
	mob.SetPosition(entity.Vec3{Y: 10})
	swim := &SwimGoal{}
	if swim.CanStart(ctx) {
		t.Fatal("low altitude incorrectly implied water")
	}
	world.state = block.DefaultStateID("minecraft:water")
	if !swim.CanStart(ctx) {
		t.Fatal("actual water failed to activate swimming")
	}
}
