package ai

import (
	"math"
	"testing"

	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestGoalMovementUsesLiveSpeedEffects exercises path navigation and direct goal motion.
func TestGoalMovementUsesLiveSpeedEffects(t *testing.T) {
	for _, test := range []struct {
		name  string
		tick  func(*Context)
		speed float64
	}{
		{"wander", func(c *Context) { g := &WanderGoal{Speed: .2, targetX: 20, walking: true}; g.Tick(c) }, .2},
		{"panic", func(c *Context) { g := &PanicGoal{Speed: .3, targetX: 20, timer: 60}; g.Tick(c) }, .3},
		{"flee", func(c *Context) { g := &FleeGoal{Speed: .3, FleeDist: 10}; g.Tick(c) }, .3},
		{"sunlight", func(c *Context) { g := &AvoidSunlightGoal{Speed: .3, targetX: 20, seeking: true}; g.Tick(c) }, .3},
		{"tempt", func(c *Context) { g := &TemptGoal{Speed: .2, target: &c.Players[0]}; g.Tick(c) }, .2},
		{"melee", func(c *Context) { g := &MeleeAttackGoal{Speed: .2}; g.Start(c); g.Tick(c) }, .2},
		{"ranged strafe", func(c *Context) {
			g := &RangedAttackGoal{Speed: .2, StrafeSpeed: .1, AttackDist: 15}
			g.Start(c)
			g.Tick(c)
		}, .1},
		{"navigation", func(c *Context) {
			n := NewNavigator(.2)
			n.SetPath(&Path{Nodes: []PathNode{{X: 20}}})
			n.FollowPath(c.Mob, nil)
		}, .2},
	} {
		t.Run(test.name, func(t *testing.T) {
			mob := entity.NewMobEntityByName(1, protocol.UUID{1}, "cow", entity.Vec3{})
			mob.SetTarget(2)
			context := &Context{Mob: mob, Players: []PlayerInfo{{EntityID: 2, Pos: entity.Vec3{X: 10}, HeldItem: "minecraft:wheat"}}}
			mob.Effects().Add(effect.Instance{ID: effectdata.EffectSlowness, Amplifier: 1, Duration: 20})
			test.tick(context)
			velocity := mob.Velocity()
			want := test.speed * (1 + float64(float32(-.15))*2)
			if actual := math.Hypot(velocity.X, velocity.Z); math.Abs(actual-want) > 1e-8 {
				t.Fatalf("slowness ignored: got=%v want=%v", actual, want)
			}
			mob.Effects().Add(effect.Instance{ID: effectdata.EffectSlowness, Amplifier: 255, Duration: 20})
			test.tick(context)
			velocity = mob.Velocity()
			if math.Hypot(velocity.X, velocity.Z) > 1e-9 {
				t.Fatal("negative effective speed moved the mob")
			}
		})
	}
}
