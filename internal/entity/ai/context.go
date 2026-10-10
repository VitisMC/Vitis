package ai

import "github.com/vitismc/vitis/internal/entity"

// PlayerInfo holds position data for a nearby player visible to AI.
type PlayerInfo struct {
	EntityID    int32
	Pos         entity.Vec3
	GameMode    int32
	HeldItem    string
	OffhandItem string
}

// BlockAccess provides block state lookup for AI navigation.
type BlockAccess interface {
	GetBlockStateAt(x, y, z int) int32
}

// MobActions applies validated combat actions on the authoritative world tick.
type MobActions interface {
	AttackMobTarget(mob *entity.MobEntity, target int32, ranged bool) bool
	ExplodeMob(mob *entity.MobEntity, power float64) bool
	MobCanSee(from, to entity.Vec3) bool
}

// MobEnvironment exposes the live environment needed by noncombat goals.
type MobEnvironment interface {
	MobSunlit(pos entity.Vec3) bool
	MobBrightness(pos entity.Vec3) uint8
}

// CanSee reports whether resident world geometry permits line of sight.
func (c *Context) CanSee(pos entity.Vec3) bool {
	if actions, ok := c.Blocks.(MobActions); ok {
		origin := c.Mob.Position()
		origin.Y += c.Mob.TypeDef().Height * .85
		pos.Y += 1.62
		return actions.MobCanSee(origin, pos)
	}
	return c.Blocks == nil
}

// Context provides world information to AI goals each tick.
type Context struct {
	Mob     *entity.MobEntity
	Players []PlayerInfo
	Tick    uint64
	Blocks  BlockAccess
}

// NearestPlayer returns the closest visible survival/adventure player, or nil.
func (c *Context) NearestPlayer(maxDist float64) *PlayerInfo {
	return c.nearestPlayer(maxDist, 0)
}

// nearestPlayer optionally restricts vertical separation for species-specific targeting.
func (c *Context) nearestPlayer(maxDist, maxVertical float64) *PlayerInfo {
	if len(c.Players) == 0 {
		return nil
	}
	maxDistSq := maxDist * maxDist
	var best *PlayerInfo
	bestDist := maxDistSq + 1
	pos := c.Mob.Position()
	for i := range c.Players {
		p := &c.Players[i]
		if p.GameMode == 1 || p.GameMode == 3 {
			continue
		}
		dx := pos.X - p.Pos.X
		dy := pos.Y - p.Pos.Y
		if maxVertical > 0 && (dy < -maxVertical || dy > maxVertical) {
			continue
		}
		dz := pos.Z - p.Pos.Z
		distSq := dx*dx + dy*dy + dz*dz
		if distSq < bestDist && distSq <= maxDistSq && c.CanSee(p.Pos) {
			bestDist = distSq
			best = p
		}
	}
	if best != nil && bestDist <= maxDistSq {
		return best
	}
	return nil
}
