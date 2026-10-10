package ai

import (
	"testing"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestSkeletonGoalsFollowLiveHandEquipment switches weapons without rebuilding a brain.
func TestSkeletonGoalsFollowLiveHandEquipment(t *testing.T) {
	for _, name := range []string{"skeleton", "stray", "wither_skeleton"} {
		mob := entity.NewMobEntityByName(1, protocol.UUID{1}, name, entity.Vec3{Y: 64})
		mob.SetTarget(2)
		world := &actionTestWorld{visible: true}
		ctx := testContext(mob, []PlayerInfo{{EntityID: 2, Pos: entity.Vec3{X: 1, Y: 64}}})
		ctx.Blocks = world
		brain := NewBrain(mob.TypeDef())
		if err := mob.SetEquipment(entity.EquipmentMainHand, inventory.EmptySlot()); err != nil {
			t.Fatal(err)
		}
		for range 40 {
			brain.Goals.Tick(ctx)
		}
		if world.melee != 2 || world.arrows != 0 {
			t.Fatalf("%s unarmed combat: %+v", name, world)
		}
		if err := mob.SetEquipment(entity.EquipmentOffHand, inventory.NewSlot(item.IDByName("minecraft:bow"), 1)); err != nil {
			t.Fatal(err)
		}
		for range 40 {
			brain.Goals.Tick(ctx)
		}
		if world.melee != 2 || world.arrows != 2 {
			t.Fatalf("%s bow combat: %+v", name, world)
		}
		if err := mob.SetEquipment(entity.EquipmentOffHand, inventory.EmptySlot()); err != nil {
			t.Fatal(err)
		}
		brain.Goals.Tick(ctx)
		if world.melee != 3 || world.arrows != 2 {
			t.Fatal("removed bow left a stale ranged goal")
		}
	}
}

// TestSunAvoidanceUsesLiveExposure cancels sun avoidance when weather or shelter removes exposure.
func TestSunAvoidanceUsesLiveExposure(t *testing.T) {
	mob := entity.NewMobEntityByName(1, protocol.UUID{1}, "skeleton", entity.Vec3{Y: 64})
	world := &actionTestWorld{visible: true}
	ctx := testContext(mob, nil)
	ctx.Blocks = world
	goal := &AvoidSunlightGoal{}
	if goal.CanStart(ctx) {
		t.Fatal("sun avoidance started solely from height or tick count")
	}
	world.sunlit = true
	if !goal.CanStart(ctx) {
		t.Fatal("exposure was ignored")
	}
	goal.Start(ctx)
	world.sunlit = false
	if goal.CanContinue(ctx) {
		t.Fatal("sun avoidance continued after exposure ended")
	}
}
