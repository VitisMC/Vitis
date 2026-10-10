package ai

import (
	"testing"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestSlimeGoalHopsAndUsesCurrentSizeForContactDamage verifies the runtime preset and target loss.
func TestSlimeGoalHopsAndUsesCurrentSizeForContactDamage(t *testing.T) {
	mob := entity.NewMobEntityByName(1, protocol.UUID{1}, "slime", entity.Vec3{Y: 64})
	mob.SetSlimeSize(4, true)
	mob.SetOnGround(true)
	mob.SetTarget(2)
	world := &actionTestWorld{visible: true}
	ctx := testContext(mob, []PlayerInfo{{EntityID: 2, Pos: entity.Vec3{X: .5, Y: 64}}})
	ctx.Blocks = world
	brain := NewBrain(mob.TypeDef())
	brain.Goals.Tick(ctx)
	if mob.Velocity().Y <= 0 || mob.Velocity().X <= 0 || world.melee != 1 {
		t.Fatalf("slime did not hop toward contact: velocity=%+v hits=%d", mob.Velocity(), world.melee)
	}
	mob.SetSlimeSize(1, true)
	brain.Goals.Tick(ctx)
	if world.melee != 1 {
		t.Fatal("tiny slime dealt damage")
	}
	mob.SetSlimeSize(4, true)
	world.visible = false
	brain.Goals.Tick(ctx)
	if mob.Target() != -1 || world.melee != 1 {
		t.Fatal("slime attacked through a wall or retained an invisible target")
	}
	world.visible = true
	ctx.Players = nil
	brain.Goals.Tick(ctx)
	if mob.Target() != -1 {
		t.Fatal("slime retained a vanished player")
	}
	mob.Damage(100, "generic")
	brain.Goals.Tick(ctx)
	if world.melee != 1 {
		t.Fatal("dead slime requested another attack")
	}
}

// TestSlimeTargetsRespectVerticalRangeAndGameMode prevents attacks on unreachable players.
func TestSlimeTargetsRespectVerticalRangeAndGameMode(t *testing.T) {
	for _, player := range []PlayerInfo{
		{EntityID: 2, Pos: entity.Vec3{Y: 69}},
		{EntityID: 2, Pos: entity.Vec3{Y: 64}, GameMode: 1},
		{EntityID: 2, Pos: entity.Vec3{Y: 64}, GameMode: 3},
	} {
		mob := entity.NewMobEntityByName(1, protocol.UUID{1}, "slime", entity.Vec3{Y: 64})
		brain := NewBrain(mob.TypeDef())
		brain.Targets.Tick(testContext(mob, []PlayerInfo{player}))
		if mob.Target() >= 0 {
			t.Fatalf("slime acquired invalid target: %+v", player)
		}
	}
}
