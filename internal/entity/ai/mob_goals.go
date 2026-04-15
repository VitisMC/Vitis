package ai

import (
	"math"
	"math/rand"

	"github.com/vitismc/vitis/internal/entity"
)

// --- BurnInSunlightGoal ---

// BurnInSunlightGoal sets the mob on fire during daytime if exposed to sky.
// Uses a simplified check: mob Y > 60 and world tick in daytime range.
type BurnInSunlightGoal struct {
	burnTicks int
}

func (g *BurnInSunlightGoal) CanStart(ctx *Context) bool {
	// Daytime: ticks 0-12000
	dayTick := ctx.Tick % 24000
	if dayTick > 12000 {
		return false
	}
	pos := ctx.Mob.Position()
	// Simplified sky exposure check: above Y=60
	if pos.Y < 60 {
		return false
	}
	// Check if wearing helmet (would prevent burning) - skip for now
	return true
}

func (g *BurnInSunlightGoal) Start(ctx *Context) {
	g.burnTicks = 0
}

func (g *BurnInSunlightGoal) Tick(ctx *Context) {
	g.burnTicks++
	// Apply 1 fire damage every 20 ticks (1 second)
	if g.burnTicks%20 == 0 {
		ctx.Mob.Damage(1, "on_fire")
	}
}

func (g *BurnInSunlightGoal) CanContinue(ctx *Context) bool {
	dayTick := ctx.Tick % 24000
	return dayTick <= 12000 && ctx.Mob.Position().Y >= 60 && !ctx.Mob.IsDead()
}

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
}

func (g *CreeperSwellGoal) Tick(ctx *Context) {
	p := ctx.NearestPlayer(g.swellDist())
	if p == nil {
		g.swelling = false
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
		// Explode: kill the creeper and mark for explosion processing
		ctx.Mob.Damage(ctx.Mob.Health(), "explosion.self")
		g.swelling = false
	}
}

func (g *CreeperSwellGoal) CanContinue(ctx *Context) bool {
	return g.swelling && !ctx.Mob.IsDead()
}

func (g *CreeperSwellGoal) Stop(ctx *Context) {
	g.fuse = 0
	g.swelling = false
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
	cooldown    int
	strafeDir   int // -1 left, 1 right
	strafeTick  int
}

func (g *RangedAttackGoal) CanStart(ctx *Context) bool {
	return ctx.Mob.Target() >= 0
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

	// Move toward target if too far
	if dist > atkDist {
		nx := dx / dist * speed
		nz := dz / dist * speed
		ctx.Mob.SetPosition(entity.Vec3{X: pos.X + nx, Y: pos.Y, Z: pos.Z + nz})
	} else if dist > 4.0 {
		// Strafe when in range
		g.strafeTick++
		if g.strafeTick > 40 {
			g.strafeDir = -g.strafeDir
			g.strafeTick = 0
		}
		strafeSpeed := g.StrafeSpeed
		if strafeSpeed <= 0 {
			strafeSpeed = speed * 0.5
		}
		// Perpendicular movement
		perpX := -dz / dist * float64(g.strafeDir) * strafeSpeed
		perpZ := dx / dist * float64(g.strafeDir) * strafeSpeed
		ctx.Mob.SetPosition(entity.Vec3{X: pos.X + perpX, Y: pos.Y, Z: pos.Z + perpZ})
	}

	if g.cooldown > 0 {
		g.cooldown--
	}
}

func (g *RangedAttackGoal) CanContinue(ctx *Context) bool {
	return ctx.Mob.Target() >= 0
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
			if ctx.Mob.DistanceTo(p.Pos) <= atkDist {
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
	dayTick := ctx.Tick % 24000
	if dayTick > 12000 {
		return false
	}
	return ctx.Mob.Position().Y >= 60
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
	nx := dx / dist * speed
	nz := dz / dist * speed
	yaw := float32(math.Atan2(-nx, nz) * 180.0 / math.Pi)
	ctx.Mob.SetRotation(entity.Vec2{X: yaw, Y: 0})
	ctx.Mob.SetPosition(entity.Vec3{X: pos.X + nx, Y: pos.Y, Z: pos.Z + nz})
}

func (g *AvoidSunlightGoal) CanContinue(ctx *Context) bool {
	return g.seeking
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
	// Simplified: always follow nearest player in survival (full impl would check held item)
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
	nx := dx / dist * speed
	nz := dz / dist * speed
	yaw := float32(math.Atan2(-nx, nz) * 180.0 / math.Pi)
	ctx.Mob.SetRotation(entity.Vec2{X: yaw, Y: 0})
	ctx.Mob.SetPosition(entity.Vec3{X: pos.X + nx, Y: pos.Y, Z: pos.Z + nz})
}

func (g *TemptGoal) CanContinue(ctx *Context) bool {
	if g.target == nil {
		return false
	}
	maxDist := g.MaxDist
	if maxDist <= 0 {
		maxDist = 10.0
	}
	return ctx.Mob.DistanceToSq(g.target.Pos) <= maxDist*maxDist
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

	dayTick := ctx.Tick % 24000
	isNight := dayTick >= 13000 && dayTick <= 23000
	if !isNight {
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
