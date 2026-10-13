package ai

import (
	"math"
	"math/rand"

	"github.com/vitismc/vitis/internal/block/fluid"
	genattr "github.com/vitismc/vitis/internal/data/generated/attribute"
	"github.com/vitismc/vitis/internal/entity/physics"
)

// SlimeHopGoal combines hopping, fluid escape, facing and validated contact attacks.
type SlimeHopGoal struct {
	jumpDelay int
	lookDelay int
}

// CanStart permits hopping only while the slime is alive and active.
func (g *SlimeHopGoal) CanStart(ctx *Context) bool {
	return !ctx.Mob.IsDead() && !ctx.Mob.Removed() && !ctx.Mob.NoAI()
}

// Start initializes the first hop without retaining references to players.
func (g *SlimeHopGoal) Start(ctx *Context) { g.jumpDelay, g.lookDelay = 0, 0 }

// CanContinue stops actions immediately when the slime becomes unavailable.
func (g *SlimeHopGoal) CanContinue(ctx *Context) bool { return g.CanStart(ctx) }

// Stop releases movement control without clearing vertical physics.
func (g *SlimeHopGoal) Stop(ctx *Context) {
	velocity := ctx.Mob.Velocity()
	velocity.X, velocity.Z = 0, 0
	ctx.Mob.SetVelocity(velocity)
}

// Tick reads the current target and size before choosing a hop and requesting damage.
func (g *SlimeHopGoal) Tick(ctx *Context) {
	if !g.CanStart(ctx) {
		return
	}
	mob := ctx.Mob
	position := mob.Position()
	var target *PlayerInfo
	for index := range ctx.Players {
		player := &ctx.Players[index]
		if player.EntityID == mob.Target() && player.GameMode != 1 && player.GameMode != 3 && math.Abs(player.Pos.Y-position.Y) <= 4 && mob.DistanceToSq(player.Pos) <= mob.TypeDef().FollowRange*mob.TypeDef().FollowRange && ctx.CanSee(player.Pos) {
			target = player
			break
		}
	}
	rotation := mob.Rotation()
	if target != nil {
		rotation.X = float32(math.Atan2(position.X-target.Pos.X, target.Pos.Z-position.Z) * 180 / math.Pi)
	} else {
		mob.SetTarget(-1)
		g.lookDelay--
		if g.lookDelay <= 0 && mob.OnGround() {
			g.lookDelay = 40 + rand.Intn(60)
			rotation.X = float32(rand.Intn(360))
		}
	}
	mob.SetRotation(rotation)
	velocity := mob.Velocity()
	mob.SyncEffectMovement()
	speed := max(0, mob.Attributes().GetValue(genattr.AttrMovementSpeed))
	inFluid := false
	if ctx.Blocks != nil {
		inFluid = fluid.ContainedFluidType(ctx.Blocks.GetBlockStateAt(int(math.Floor(position.X)), int(math.Floor(position.Y)), int(math.Floor(position.Z)))) != fluid.FluidTypeNone
	}
	if inFluid {
		speed *= 1.2
		if rand.Float32() < .8 {
			velocity.Y = max(velocity.Y, .42)
		}
	} else if mob.OnGround() {
		if g.jumpDelay <= 0 {
			g.jumpDelay = 10 + rand.Intn(20)
			if target != nil && mob.SlimeSize() > 1 {
				g.jumpDelay /= 3
			}
			velocity.Y = .42
		} else {
			g.jumpDelay--
			speed = 0
		}
	}
	yaw := float64(rotation.X) * math.Pi / 180
	velocity.X, velocity.Z = -math.Sin(yaw)*speed, math.Cos(yaw)*speed
	mob.SetVelocity(velocity)
	if mob.SlimeSize() <= 1 {
		return
	}
	box := (physics.EntityDimensions{Width: mob.TypeDef().Width, Height: mob.TypeDef().Height}).MakeBoundingBox(position.X, position.Y, position.Z)
	if actions, ok := ctx.Blocks.(MobActions); ok {
		for _, player := range ctx.Players {
			playerBox := physics.PlayerDimensions().MakeBoundingBox(player.Pos.X, player.Pos.Y, player.Pos.Z)
			if player.GameMode != 1 && player.GameMode != 3 && box.Grow(.2).Intersects(playerBox) && ctx.CanSee(player.Pos) {
				actions.AttackMobTarget(mob, player.EntityID, false)
			}
		}
	}
}
