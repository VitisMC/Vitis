package ai

import (
	"math"
	"math/rand"

	"github.com/vitismc/vitis/internal/entity"
)

// --- LookAtPlayerGoal ---

// LookAtPlayerGoal makes a mob face the nearest player within range.
type LookAtPlayerGoal struct {
	MaxDist  float64
	target   *PlayerInfo
	lookTime int
}

func (g *LookAtPlayerGoal) CanStart(ctx *Context) bool {
	p := ctx.NearestPlayer(g.MaxDist)
	if p == nil {
		return false
	}
	g.target = p
	return true
}

func (g *LookAtPlayerGoal) Start(ctx *Context) {
	g.lookTime = 40 + rand.Intn(40)
}

func (g *LookAtPlayerGoal) Tick(ctx *Context) {
	if g.target == nil {
		return
	}
	pos := ctx.Mob.Position()
	dx := g.target.Pos.X - pos.X
	dz := g.target.Pos.Z - pos.Z
	yaw := float32(math.Atan2(-dx, dz) * 180.0 / math.Pi)
	dy := g.target.Pos.Y - pos.Y
	dist := math.Sqrt(dx*dx + dz*dz)
	pitch := float32(-math.Atan2(dy, dist) * 180.0 / math.Pi)
	ctx.Mob.SetRotation(entity.Vec2{X: yaw, Y: pitch})
	g.lookTime--
}

func (g *LookAtPlayerGoal) CanContinue(ctx *Context) bool {
	return g.lookTime > 0 && g.target != nil
}

func (g *LookAtPlayerGoal) Stop(ctx *Context) {
	g.target = nil
}

// --- WanderGoal ---

// WanderGoal makes a mob wander randomly when idle.
type WanderGoal struct {
	Speed    float64
	Interval int
	targetX  float64
	targetZ  float64
	timer    int
	walking  bool
}

func (g *WanderGoal) CanStart(ctx *Context) bool {
	g.timer--
	if g.timer > 0 {
		return false
	}
	interval := g.Interval
	if interval <= 0 {
		interval = 1
	}
	g.timer = interval + rand.Intn(interval)
	return true
}

func (g *WanderGoal) Start(ctx *Context) {
	pos := ctx.Mob.Position()
	angle := rand.Float64() * 2 * math.Pi
	dist := 4.0 + rand.Float64()*6.0
	g.targetX = pos.X + math.Cos(angle)*dist
	g.targetZ = pos.Z + math.Sin(angle)*dist
	g.walking = true
}

func (g *WanderGoal) Tick(ctx *Context) {
	pos := ctx.Mob.Position()
	dx := g.targetX - pos.X
	dz := g.targetZ - pos.Z
	distSq := dx*dx + dz*dz

	if distSq < 0.5 {
		g.walking = false
		return
	}

	dist := math.Sqrt(distSq)
	speed := g.Speed
	if speed <= 0 {
		speed = 0.15
	}
	nx := dx / dist * speed
	nz := dz / dist * speed

	yaw := float32(math.Atan2(-nx, nz) * 180.0 / math.Pi)
	ctx.Mob.SetRotation(entity.Vec2{X: yaw, Y: 0})
	ctx.Mob.SetPosition(entity.Vec3{X: pos.X + nx, Y: pos.Y, Z: pos.Z + nz})
}

func (g *WanderGoal) CanContinue(ctx *Context) bool {
	return g.walking
}

func (g *WanderGoal) Stop(ctx *Context) {
	g.walking = false
}

// --- MeleeAttackGoal ---

// MeleeAttackGoal makes a hostile mob move toward and attack its target.
type MeleeAttackGoal struct {
	Speed      float64
	AttackDist float64
	Cooldown   int
	cooldown   int
	nav        *Navigator
	replanTick int
}

func (g *MeleeAttackGoal) CanStart(ctx *Context) bool {
	return ctx.Mob.Target() >= 0
}

func (g *MeleeAttackGoal) Start(ctx *Context) {
	g.cooldown = 0
	g.nav = NewNavigator(g.Speed)
	g.replanTick = 0
}

func (g *MeleeAttackGoal) Tick(ctx *Context) {
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

	yaw := float32(math.Atan2(-dx, dz) * 180.0 / math.Pi)
	pitch := float32(-math.Atan2(dy, math.Sqrt(dx*dx+dz*dz)) * 180.0 / math.Pi)
	ctx.Mob.SetRotation(entity.Vec2{X: yaw, Y: pitch})

	atkDist := g.AttackDist
	if atkDist <= 0 {
		atkDist = 2.0
	}

	if dist > atkDist {
		g.replanTick--
		if ctx.Blocks != nil && (!g.nav.HasPath() || g.replanTick <= 0) {
			g.nav.NavigateTo(ctx.Mob, targetPos, ctx.Blocks)
			g.replanTick = 20
		}
		if g.nav.HasPath() {
			g.nav.FollowPath(ctx.Mob)
		} else {
			speed := g.Speed
			if speed <= 0 {
				speed = 0.2
			}
			nx := dx / dist * speed
			nz := dz / dist * speed
			ctx.Mob.SetPosition(entity.Vec3{X: pos.X + nx, Y: pos.Y, Z: pos.Z + nz})
		}
	}

	if g.cooldown > 0 {
		g.cooldown--
	}
}

func (g *MeleeAttackGoal) CanContinue(ctx *Context) bool {
	return ctx.Mob.Target() >= 0
}

func (g *MeleeAttackGoal) Stop(ctx *Context) {
	if g.nav != nil {
		g.nav.ClearPath()
	}
}

// ShouldAttack returns true if the mob is in range and off cooldown.
// The caller is responsible for applying damage.
func (g *MeleeAttackGoal) ShouldAttack(ctx *Context) bool {
	targetID := ctx.Mob.Target()
	if targetID < 0 || g.cooldown > 0 {
		return false
	}
	for _, p := range ctx.Players {
		if p.EntityID == targetID {
			dist := ctx.Mob.DistanceTo(p.Pos)
			atkDist := g.AttackDist
			if atkDist <= 0 {
				atkDist = 2.0
			}
			if dist <= atkDist {
				cd := g.Cooldown
				if cd <= 0 {
					cd = 20
				}
				g.cooldown = cd
				return true
			}
		}
	}
	return false
}

// --- NearestAttackableTargetGoal ---

// NearestAttackableTargetGoal scans for the nearest valid target and sets it.
type NearestAttackableTargetGoal struct {
	MaxDist  float64
	Interval int
	timer    int
}

func (g *NearestAttackableTargetGoal) CanStart(ctx *Context) bool {
	g.timer--
	if g.timer > 0 {
		return false
	}
	g.timer = g.Interval
	if g.timer <= 0 {
		g.timer = 10
	}
	p := ctx.NearestPlayer(g.MaxDist)
	return p != nil
}

func (g *NearestAttackableTargetGoal) Start(ctx *Context) {
	p := ctx.NearestPlayer(g.MaxDist)
	if p != nil {
		ctx.Mob.SetTarget(p.EntityID)
	}
}

func (g *NearestAttackableTargetGoal) Tick(ctx *Context) {
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

func (g *NearestAttackableTargetGoal) CanContinue(ctx *Context) bool {
	return ctx.Mob.Target() >= 0
}

func (g *NearestAttackableTargetGoal) Stop(ctx *Context) {
	ctx.Mob.SetTarget(-1)
}

// --- FleeGoal ---

// FleeGoal makes a mob run away from the nearest player.
type FleeGoal struct {
	Speed    float64
	FleeDist float64
	timer    int
}

func (g *FleeGoal) CanStart(ctx *Context) bool {
	if ctx.Mob.Health() > ctx.Mob.MaxHealth()*0.5 {
		return false
	}
	p := ctx.NearestPlayer(g.FleeDist)
	return p != nil
}

func (g *FleeGoal) Start(ctx *Context) {
	g.timer = 60 + rand.Intn(40)
}

func (g *FleeGoal) Tick(ctx *Context) {
	g.timer--
	p := ctx.NearestPlayer(g.FleeDist * 2)
	if p == nil {
		return
	}
	pos := ctx.Mob.Position()
	dx := pos.X - p.Pos.X
	dz := pos.Z - p.Pos.Z
	dist := math.Sqrt(dx*dx + dz*dz)
	if dist < 0.01 {
		return
	}
	speed := g.Speed
	if speed <= 0 {
		speed = 0.25
	}
	nx := dx / dist * speed
	nz := dz / dist * speed
	yaw := float32(math.Atan2(-nx, nz) * 180.0 / math.Pi)
	ctx.Mob.SetRotation(entity.Vec2{X: yaw, Y: 0})
	ctx.Mob.SetPosition(entity.Vec3{X: pos.X + nx, Y: pos.Y, Z: pos.Z + nz})
}

func (g *FleeGoal) CanContinue(ctx *Context) bool {
	return g.timer > 0
}

func (g *FleeGoal) Stop(ctx *Context) {}

// --- PanicGoal ---

// PanicGoal makes a passive mob run in a random direction when hurt.
type PanicGoal struct {
	Speed   float64
	targetX float64
	targetZ float64
	timer   int
}

func (g *PanicGoal) CanStart(ctx *Context) bool {
	return ctx.Mob.HurtTime() > 0
}

func (g *PanicGoal) Start(ctx *Context) {
	pos := ctx.Mob.Position()
	angle := rand.Float64() * 2 * math.Pi
	dist := 8.0 + rand.Float64()*8.0
	g.targetX = pos.X + math.Cos(angle)*dist
	g.targetZ = pos.Z + math.Sin(angle)*dist
	g.timer = 60 + rand.Intn(40)
}

func (g *PanicGoal) Tick(ctx *Context) {
	g.timer--
	pos := ctx.Mob.Position()
	dx := g.targetX - pos.X
	dz := g.targetZ - pos.Z
	distSq := dx*dx + dz*dz
	if distSq < 1.0 {
		g.timer = 0
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

func (g *PanicGoal) CanContinue(ctx *Context) bool {
	return g.timer > 0
}

func (g *PanicGoal) Stop(ctx *Context) {}

// --- SwimGoal ---

// SwimGoal makes a mob bob up when in water (stub — checks Y < 63 as placeholder).
type SwimGoal struct{}

func (g *SwimGoal) CanStart(ctx *Context) bool {
	return ctx.Mob.Position().Y < 62
}

func (g *SwimGoal) Start(ctx *Context) {}

func (g *SwimGoal) Tick(ctx *Context) {
	pos := ctx.Mob.Position()
	ctx.Mob.SetVelocity(entity.Vec3{X: 0, Y: 0.04, Z: 0})
	ctx.Mob.SetPosition(entity.Vec3{X: pos.X, Y: pos.Y + 0.04, Z: pos.Z})
}

func (g *SwimGoal) CanContinue(ctx *Context) bool {
	return ctx.Mob.Position().Y < 62
}

func (g *SwimGoal) Stop(ctx *Context) {}
