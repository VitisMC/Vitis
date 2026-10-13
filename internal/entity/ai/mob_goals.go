package ai

import (
	"math"
	"math/rand"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/block/fluid"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/logger"
)

// --- BurnInSunlightGoal ---

// BurnInSunlightGoal sets the mob on fire during daytime if exposed to sky.
type BurnInSunlightGoal struct {
	random func() float32
}

// CanStart checks live exposure and the mob's environmental eligibility.
func (g *BurnInSunlightGoal) CanStart(ctx *Context) bool {
	environment, ok := ctx.Blocks.(MobEnvironment)
	return ok && !ctx.Mob.IsDead() && !ctx.Mob.Removed() && !ctx.Mob.TypeDef().FireImmune && environment.MobSunlit(ctx.Mob.Position())
}

// Start leaves existing fire and equipment state unchanged.
func (g *BurnInSunlightGoal) Start(ctx *Context) {}

// Tick samples current brightness and wears head protection before ignition.
func (g *BurnInSunlightGoal) Tick(ctx *Context) {
	if !g.CanStart(ctx) {
		return
	}
	pos := ctx.Mob.Position()
	state := ctx.Blocks.GetBlockStateAt(int(math.Floor(pos.X)), int(math.Floor(pos.Y)), int(math.Floor(pos.Z)))
	if fluid.ContainedFluidType(state) == fluid.FluidTypeWater || block.NameFromState(state) == "minecraft:powder_snow" {
		return
	}
	light := float32(min(uint8(15), ctx.Blocks.(MobEnvironment).MobBrightness(pos))) / 15
	brightness := light / (4 - 3*light)
	sample := rand.Float32
	if g.random != nil {
		sample = g.random
	}
	if brightness <= .5 || sample()*30 >= (brightness-.4)*2 {
		return
	}
	protected, err := ctx.Mob.WearSunHelmet()
	if err != nil {
		logger.Error("wear sunlight helmet failed", "entity_id", ctx.Mob.ID(), "error", err)
	}
	// Refresh fire while the live environment still exposes the mob to sunlight.
	if !protected {
		ctx.Mob.SetFireTicks(160)
	}
}

// CanContinue rechecks exposure before the next environmental update.
func (g *BurnInSunlightGoal) CanContinue(ctx *Context) bool {
	return !ctx.Mob.IsDead() && g.CanStart(ctx)
}

// Stop leaves ongoing fire to the living entity's environment simulation.
func (g *BurnInSunlightGoal) Stop(ctx *Context) {}

// --- CreeperSwellGoal ---

// CreeperSwellGoal makes a creeper start swelling when close to a player and explode.
type CreeperSwellGoal struct {
	SwellDist   float64
	FuseTime    int // ticks to explode (default 30 = 1.5s)
	ExplosionPw float32
	fuse        int
	swelling    bool
}

func (g *CreeperSwellGoal) CanStart(ctx *Context) bool {
	p := ctx.NearestPlayer(g.swellDist())
	return p != nil
}

func (g *CreeperSwellGoal) Start(ctx *Context) {
	g.fuse = 0
	g.swelling = true
	ctx.Mob.SetSwellDirection(1)
}

func (g *CreeperSwellGoal) Tick(ctx *Context) {
	p := ctx.NearestPlayer(g.swellDist())
	if p == nil {
		g.swelling = false
		ctx.Mob.SetSwellDirection(-1)
		return
	}

	// Face the player
	pos := ctx.Mob.Position()
	dx := p.Pos.X - pos.X
	dz := p.Pos.Z - pos.Z
	yaw := float32(math.Atan2(-dx, dz) * 180.0 / math.Pi)
	ctx.Mob.SetRotation(entity.Vec2{X: yaw, Y: 0})

	g.fuse++
	fuseTime := g.fuseTime()
	if g.fuse >= fuseTime {
		if actions, ok := ctx.Blocks.(MobActions); ok && actions.ExplodeMob(ctx.Mob, float64(g.ExplosionPw)) {
			g.swelling = false
		}
	}
}

func (g *CreeperSwellGoal) CanContinue(ctx *Context) bool {
	return g.swelling && !ctx.Mob.IsDead()
}

func (g *CreeperSwellGoal) Stop(ctx *Context) {
	g.fuse = 0
	g.swelling = false
	ctx.Mob.SetSwellDirection(-1)
}

func (g *CreeperSwellGoal) swellDist() float64 {
	if g.SwellDist <= 0 {
		return 3.0
	}
	return g.SwellDist
}

func (g *CreeperSwellGoal) fuseTime() int {
	if g.FuseTime <= 0 {
		return 30
	}
	return g.FuseTime
}

// FuseTicks returns the current fuse progress.
func (g *CreeperSwellGoal) FuseTicks() int { return g.fuse }

// IsSwelling returns whether the creeper is currently swelling.
func (g *CreeperSwellGoal) IsSwelling() bool { return g.swelling }

// --- RangedAttackGoal ---

// RangedAttackGoal makes a mob strafe and shoot at its target (skeleton behavior).
type RangedAttackGoal struct {
	Speed       float64
	AttackDist  float64
	Cooldown    int
	StrafeSpeed float64
	RequiresBow bool
	cooldown    int
	strafeDir   int // -1 left, 1 right
	strafeTick  int
}

func (g *RangedAttackGoal) CanStart(ctx *Context) bool {
	return ctx.Mob.Target() >= 0 && (!g.RequiresBow || ctx.Mob.UsesBow())
}

func (g *RangedAttackGoal) Start(ctx *Context) {
	g.cooldown = 0
	g.strafeDir = 1
	if rand.Intn(2) == 0 {
		g.strafeDir = -1
	}
	g.strafeTick = 0
}

func (g *RangedAttackGoal) Tick(ctx *Context) {
	targetID := ctx.Mob.Target()
	if targetID < 0 {
		return
	}

	var targetPos entity.Vec3
	found := false
	for _, p := range ctx.Players {
		if p.EntityID == targetID {
			targetPos = p.Pos
			found = true
			break
		}
	}
	if !found {
		ctx.Mob.SetTarget(-1)
		return
	}

	pos := ctx.Mob.Position()
	dx := targetPos.X - pos.X
	dy := targetPos.Y - pos.Y
	dz := targetPos.Z - pos.Z
	dist := math.Sqrt(dx*dx + dy*dy + dz*dz)

	// Face target
	yaw := float32(math.Atan2(-dx, dz) * 180.0 / math.Pi)
	pitch := float32(-math.Atan2(dy, math.Sqrt(dx*dx+dz*dz)) * 180.0 / math.Pi)
	ctx.Mob.SetRotation(entity.Vec2{X: yaw, Y: pitch})

	atkDist := g.AttackDist
	if atkDist <= 0 {
		atkDist = 15.0
	}

	speed := g.Speed
	if speed <= 0 {
		speed = 0.25
	}
	speed = movementSpeed(ctx.Mob, speed)

	// Move toward target if too far
	vel := ctx.Mob.Entity.Velocity()
	if dist > atkDist {
		vel.X = dx / dist * speed
		vel.Z = dz / dist * speed
	} else if dist > 4.0 {
		g.strafeTick++
		if g.strafeTick > 40 {
			g.strafeDir = -g.strafeDir
			g.strafeTick = 0
		}
		strafeSpeed := g.StrafeSpeed
		if strafeSpeed <= 0 {
			strafeSpeed = speed * 0.5
		} else {
			strafeSpeed = movementSpeed(ctx.Mob, strafeSpeed)
		}
		vel.X = -dz / dist * float64(g.strafeDir) * strafeSpeed
		vel.Z = dx / dist * float64(g.strafeDir) * strafeSpeed
	}
	ctx.Mob.Entity.SetVelocity(vel)

	if g.cooldown > 0 {
		g.cooldown--
	}
	if actions, ok := ctx.Blocks.(MobActions); ok && g.ShouldShoot(ctx) {
		actions.AttackMobTarget(ctx.Mob, ctx.Mob.Target(), true)
	}
}

func (g *RangedAttackGoal) CanContinue(ctx *Context) bool {
	return g.CanStart(ctx)
}

func (g *RangedAttackGoal) Stop(ctx *Context) {}

// ShouldShoot returns true if the mob should fire a projectile this tick.
func (g *RangedAttackGoal) ShouldShoot(ctx *Context) bool {
	targetID := ctx.Mob.Target()
	if targetID < 0 || g.cooldown > 0 {
		return false
	}
	for _, p := range ctx.Players {
		if p.EntityID == targetID {
			atkDist := g.AttackDist
			if atkDist <= 0 {
				atkDist = 15.0
			}
			if ctx.Mob.DistanceTo(p.Pos) <= atkDist && p.GameMode != 1 && p.GameMode != 3 && ctx.CanSee(p.Pos) {
				cd := g.Cooldown
				if cd <= 0 {
					cd = 30
				}
				g.cooldown = cd
				return true
			}
		}
	}
	return false
}

// --- AvoidSunlightGoal ---

// AvoidSunlightGoal makes a mob seek shade during daytime.
type AvoidSunlightGoal struct {
	Speed   float64
	targetX float64
	targetZ float64
	seeking bool
}

func (g *AvoidSunlightGoal) CanStart(ctx *Context) bool {
	environment, ok := ctx.Blocks.(MobEnvironment)
	return ok && ctx.Mob.Equipment(entity.EquipmentHead).Empty() && environment.MobSunlit(ctx.Mob.Position())
}

func (g *AvoidSunlightGoal) Start(ctx *Context) {
	pos := ctx.Mob.Position()
	// Move in a random direction to find shade
	angle := rand.Float64() * 2 * math.Pi
	dist := 6.0 + rand.Float64()*6.0
	g.targetX = pos.X + math.Cos(angle)*dist
	g.targetZ = pos.Z + math.Sin(angle)*dist
	g.seeking = true
}

func (g *AvoidSunlightGoal) Tick(ctx *Context) {
	pos := ctx.Mob.Position()
	dx := g.targetX - pos.X
	dz := g.targetZ - pos.Z
	distSq := dx*dx + dz*dz
	if distSq < 1.0 {
		g.seeking = false
		return
	}
	dist := math.Sqrt(distSq)
	speed := g.Speed
	if speed <= 0 {
		speed = 0.3
	}
	speed = movementSpeed(ctx.Mob, speed)
	nx := dx / dist * speed
	nz := dz / dist * speed
	yaw := float32(math.Atan2(-nx, nz) * 180.0 / math.Pi)
	ctx.Mob.SetRotation(entity.Vec2{X: yaw, Y: 0})
	vel := ctx.Mob.Entity.Velocity()
	vel.X = nx
	vel.Z = nz
	ctx.Mob.Entity.SetVelocity(vel)
}

func (g *AvoidSunlightGoal) CanContinue(ctx *Context) bool {
	return g.seeking && g.CanStart(ctx)
}

func (g *AvoidSunlightGoal) Stop(ctx *Context) {
	g.seeking = false
}

// --- RandomTeleportGoal (Enderman) ---

// RandomTeleportGoal makes an enderman-like mob teleport randomly when hurt or rained on.
type RandomTeleportGoal struct {
	Range    float64
	Cooldown int
	cooldown int
}

func (g *RandomTeleportGoal) CanStart(ctx *Context) bool {
	if g.cooldown > 0 {
		g.cooldown--
		return false
	}
	// Teleport when hurt
	return ctx.Mob.HurtTime() > 0
}

func (g *RandomTeleportGoal) Start(ctx *Context) {
	pos := ctx.Mob.Position()
	r := g.Range
	if r <= 0 {
		r = 32.0
	}
	newX := pos.X + (rand.Float64()*2-1)*r
	newZ := pos.Z + (rand.Float64()*2-1)*r
	// Keep same Y for simplicity; a full impl would scan for ground
	ctx.Mob.SetPosition(entity.Vec3{X: newX, Y: pos.Y, Z: newZ})

	cd := g.Cooldown
	if cd <= 0 {
		cd = 40
	}
	g.cooldown = cd
}

func (g *RandomTeleportGoal) Tick(ctx *Context) {}

func (g *RandomTeleportGoal) CanContinue(ctx *Context) bool {
	return false // instant action
}

func (g *RandomTeleportGoal) Stop(ctx *Context) {}

// --- TemptGoal (passive mob following food) ---

// TemptGoal makes a passive mob follow a player holding a tempt item.
type TemptGoal struct {
	Speed   float64
	MaxDist float64
	target  *PlayerInfo
}

func (g *TemptGoal) CanStart(ctx *Context) bool {
	maxDist := g.MaxDist
	if maxDist <= 0 {
		maxDist = 10.0
	}
	p := ctx.NearestPlayer(maxDist)
	if p == nil {
		return false
	}
	if !temptItem(ctx.Mob.TypeName(), p.HeldItem) && !temptItem(ctx.Mob.TypeName(), p.OffhandItem) {
		return false
	}
	g.target = p
	return true
}

func (g *TemptGoal) Start(ctx *Context) {}

func (g *TemptGoal) Tick(ctx *Context) {
	if g.target == nil {
		return
	}
	pos := ctx.Mob.Position()
	dx := g.target.Pos.X - pos.X
	dz := g.target.Pos.Z - pos.Z
	dist := math.Sqrt(dx*dx + dz*dz)
	if dist < 2.0 {
		return // close enough
	}
	speed := g.Speed
	if speed <= 0 {
		speed = 0.2
	}
	speed = movementSpeed(ctx.Mob, speed)
	nx := dx / dist * speed
	nz := dz / dist * speed
	yaw := float32(math.Atan2(-nx, nz) * 180.0 / math.Pi)
	ctx.Mob.SetRotation(entity.Vec2{X: yaw, Y: 0})
	vel := ctx.Mob.Entity.Velocity()
	vel.X = nx
	vel.Z = nz
	ctx.Mob.Entity.SetVelocity(vel)
}

func (g *TemptGoal) CanContinue(ctx *Context) bool {
	if g.target == nil {
		return false
	}
	maxDist := g.MaxDist
	if maxDist <= 0 {
		maxDist = 10.0
	}
	for i := range ctx.Players {
		p := &ctx.Players[i]
		if p.EntityID == g.target.EntityID {
			g.target = p
			return ctx.Mob.DistanceToSq(p.Pos) <= maxDist*maxDist && (temptItem(ctx.Mob.TypeName(), p.HeldItem) || temptItem(ctx.Mob.TypeName(), p.OffhandItem))
		}
	}
	return false
}

// temptItem validates food held by a player for the currently supported species.
func temptItem(mob, item string) bool {
	switch mob {
	case "cow", "sheep":
		return item == "minecraft:wheat"
	case "pig":
		return item == "minecraft:carrot" || item == "minecraft:potato" || item == "minecraft:beetroot"
	case "chicken":
		return item == "minecraft:wheat_seeds" || item == "minecraft:melon_seeds" || item == "minecraft:pumpkin_seeds" || item == "minecraft:beetroot_seeds" || item == "minecraft:torchflower_seeds" || item == "minecraft:pitcher_pod"
	default:
		return false
	}
}

func (g *TemptGoal) Stop(ctx *Context) {
	g.target = nil
}

// --- SpiderNeutralGoal ---

// SpiderNeutralGoal makes spiders neutral during daytime and hostile at night.
type SpiderNeutralGoal struct {
	MaxDist float64
	timer   int
}

func (g *SpiderNeutralGoal) CanStart(ctx *Context) bool {
	g.timer--
	if g.timer > 0 {
		return false
	}
	g.timer = 10

	environment, available := ctx.Blocks.(MobEnvironment)
	if available && environment.MobBrightness(ctx.Mob.Position()) >= 8 {
		// Daytime: clear target if exists
		if ctx.Mob.Target() >= 0 {
			ctx.Mob.SetTarget(-1)
		}
		return false
	}
	// Nighttime: acquire target
	p := ctx.NearestPlayer(g.MaxDist)
	return p != nil
}

func (g *SpiderNeutralGoal) Start(ctx *Context) {
	p := ctx.NearestPlayer(g.MaxDist)
	if p != nil {
		ctx.Mob.SetTarget(p.EntityID)
	}
}

func (g *SpiderNeutralGoal) Tick(ctx *Context) {
	targetID := ctx.Mob.Target()
	if targetID < 0 {
		return
	}
	found := false
	for _, p := range ctx.Players {
		if p.EntityID == targetID {
			if ctx.Mob.DistanceToSq(p.Pos) <= g.MaxDist*g.MaxDist {
				found = true
			}
			break
		}
	}
	if !found {
		ctx.Mob.SetTarget(-1)
	}
}

func (g *SpiderNeutralGoal) CanContinue(ctx *Context) bool {
	dayTick := ctx.Tick % 24000
	isNight := dayTick >= 13000 && dayTick <= 23000
	return isNight && ctx.Mob.Target() >= 0
}

func (g *SpiderNeutralGoal) Stop(ctx *Context) {
	ctx.Mob.SetTarget(-1)
}
