package ai

import "github.com/vitismc/vitis/internal/entity"

// PlayerInfo holds position data for a nearby player visible to AI.
type PlayerInfo struct {
	EntityID int32
	Pos      entity.Vec3
	GameMode int32
}

// BlockAccess provides block state lookup for AI navigation.
type BlockAccess interface {
	GetBlockStateAt(x, y, z int) int32
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
		dz := pos.Z - p.Pos.Z
		distSq := dx*dx + dy*dy + dz*dz
		if distSq < bestDist {
			bestDist = distSq
			best = p
		}
	}
	if best != nil && bestDist <= maxDistSq {
		return best
	}
	return nil
}
