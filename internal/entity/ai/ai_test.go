package ai

import (
	"testing"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol"
)

func testMob() *entity.MobEntity {
	def := entity.GetMobTypeDef("zombie")
	return entity.NewMobEntity(1, protocol.UUID{}, def, entity.Vec3{X: 0, Y: 64, Z: 0}, entity.Vec2{})
}

func testContext(mob *entity.MobEntity, players []PlayerInfo) *Context {
	return &Context{
		Mob:     mob,
		Players: players,
		Tick:    1,
	}
}

func TestGoalSelectorEmpty(t *testing.T) {
	gs := NewGoalSelector()
	mob := testMob()
	ctx := testContext(mob, nil)
	gs.Tick(ctx)
	if gs.Active() != nil {
		t.Fatal("expected no active goal")
	}
}

func TestLookAtPlayerGoal(t *testing.T) {
	mob := testMob()
	goal := &LookAtPlayerGoal{MaxDist: 16}
	players := []PlayerInfo{{EntityID: 2, Pos: entity.Vec3{X: 5, Y: 64, Z: 5}, GameMode: 0}}
	ctx := testContext(mob, players)

	if !goal.CanStart(ctx) {
		t.Fatal("expected LookAtPlayer to start")
	}
	goal.Start(ctx)
	goal.Tick(ctx)

	rot := mob.Rotation()
	if rot.X == 0 && rot.Y == 0 {
		t.Fatal("expected rotation to change")
	}
}

func TestLookAtPlayerIgnoresCreative(t *testing.T) {
	mob := testMob()
	goal := &LookAtPlayerGoal{MaxDist: 16}
	players := []PlayerInfo{{EntityID: 2, Pos: entity.Vec3{X: 5, Y: 64, Z: 5}, GameMode: 1}}
	ctx := testContext(mob, players)

	if goal.CanStart(ctx) {
		t.Fatal("expected LookAtPlayer to not start for creative player")
	}
}

func TestWanderGoal(t *testing.T) {
	mob := testMob()
	goal := &WanderGoal{Speed: 0.2, Interval: 0}
	ctx := testContext(mob, nil)

	if !goal.CanStart(ctx) {
		t.Fatal("expected wander to start with interval=0")
	}
	goal.Start(ctx)
	goal.Tick(ctx)

	pos := mob.Position()
	if pos.X == 0 && pos.Z == 0 {
		t.Fatal("expected mob to move")
	}
}

func TestMeleeAttackGoalNoTarget(t *testing.T) {
	mob := testMob()
	goal := &MeleeAttackGoal{Speed: 0.2, AttackDist: 2.0, Cooldown: 20}
	ctx := testContext(mob, nil)

	if goal.CanStart(ctx) {
		t.Fatal("expected melee attack to not start without target")
	}
}

func TestMeleeAttackGoalWithTarget(t *testing.T) {
	mob := testMob()
	mob.SetTarget(2)
	goal := &MeleeAttackGoal{Speed: 0.2, AttackDist: 2.0, Cooldown: 20}
	players := []PlayerInfo{{EntityID: 2, Pos: entity.Vec3{X: 5, Y: 64, Z: 0}, GameMode: 0}}
	ctx := testContext(mob, players)

	if !goal.CanStart(ctx) {
		t.Fatal("expected melee attack to start with target")
	}
	goal.Start(ctx)
	goal.Tick(ctx)

	pos := mob.Position()
	if pos.X <= 0 {
		t.Fatal("expected mob to move toward target")
	}
}

func TestNearestAttackableTargetGoal(t *testing.T) {
	mob := testMob()
	goal := &NearestAttackableTargetGoal{MaxDist: 35, Interval: 0}
	players := []PlayerInfo{{EntityID: 2, Pos: entity.Vec3{X: 10, Y: 64, Z: 0}, GameMode: 0}}
	ctx := testContext(mob, players)

	goal.timer = 0
	if !goal.CanStart(ctx) {
		t.Fatal("expected target goal to start")
	}
	goal.Start(ctx)

	if mob.Target() != 2 {
		t.Fatalf("expected target 2, got %d", mob.Target())
	}
}

func TestPanicGoal(t *testing.T) {
	def := entity.GetMobTypeDef("cow")
	mob := entity.NewMobEntity(1, protocol.UUID{}, def, entity.Vec3{X: 0, Y: 64, Z: 0}, entity.Vec2{})
	mob.Damage(1, "player")
	goal := &PanicGoal{Speed: 0.3}
	ctx := testContext(mob, nil)

	if !goal.CanStart(ctx) {
		t.Fatal("expected panic to start when hurt")
	}
	goal.Start(ctx)
	goal.Tick(ctx)

	pos := mob.Position()
	if pos.X == 0 && pos.Z == 0 {
		t.Fatal("expected mob to flee")
	}
}

func TestBrainRegistry(t *testing.T) {
	mob := testMob()
	reg := NewBrainRegistry()
	reg.RegisterMob(mob)

	if reg.Get(mob.ID()) == nil {
		t.Fatal("expected brain to be registered")
	}

	mobs := map[int32]*entity.MobEntity{mob.ID(): mob}
	players := []PlayerInfo{{EntityID: 2, Pos: entity.Vec3{X: 5, Y: 64, Z: 5}, GameMode: 0}}
	reg.TickAll(mobs, players, 1, nil)

	if reg.Get(mob.ID()) == nil {
		t.Fatal("expected brain to persist after tick")
	}
}

func TestGoalSelectorPriority(t *testing.T) {
	gs := NewGoalSelector()
	mob := testMob()
	mob.SetTarget(2)

	low := &LookAtPlayerGoal{MaxDist: 16}
	high := &MeleeAttackGoal{Speed: 0.2, AttackDist: 2.0, Cooldown: 20}

	gs.AddGoal(5, low)
	gs.AddGoal(1, high)

	players := []PlayerInfo{{EntityID: 2, Pos: entity.Vec3{X: 5, Y: 64, Z: 5}, GameMode: 0}}
	ctx := testContext(mob, players)

	gs.Tick(ctx)
	if gs.Active() != high {
		t.Fatal("expected higher priority goal to be active")
	}
}
