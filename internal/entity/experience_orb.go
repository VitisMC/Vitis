package entity

import (
	"math"

	genentity "github.com/vitismc/vitis/internal/data/generated/entity"
	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/protocol"
)

const (
	EntityTypeXPOrb EntityType = 21

	xpOrbDespawnTicks   = 6000
	xpOrbPickupDistance = 1.5
	xpOrbFlySpeed       = 0.025
	xpOrbMergeRadius    = 0.5
)

// XPOrb represents an experience orb entity in the world.
type XPOrb struct {
	*Entity
	xpValue      int32
	age          int32
	targetPlayer int32
}

// NewXPOrb creates an experience orb at the given position with the specified XP value.
func NewXPOrb(id int32, uuid protocol.UUID, pos Vec3, xpValue int32) *XPOrb {
	e := NewEntity(id, uuid, EntityTypeXPOrb, pos, Vec2{})
	e.SetProtocolInfo(genentity.EntityExperienceOrb, xpValue)
	e.vel = Vec3{
		X: (math.Float64frombits(uint64(id*1103515245+12345)&0x7FFFFFFFFFFFFFFF)/math.MaxFloat64*0.2 - 0.1),
		Y: 0.2,
		Z: (math.Float64frombits(uint64(id*1103515245+54321)&0x7FFFFFFFFFFFFFFF)/math.MaxFloat64*0.2 - 0.1),
	}
	e.dirty |= DirtyVelocity
	return &XPOrb{
		Entity:       e,
		xpValue:      xpValue,
		targetPlayer: -1,
	}
}

// XPValue returns the experience points this orb awards.
func (x *XPOrb) XPValue() int32 { return x.xpValue }

// Age returns the orb's age in ticks.
func (x *XPOrb) Age() int32 { return x.age }

// TargetPlayer returns the ID of the player this orb is flying toward, or -1.
func (x *XPOrb) TargetPlayer() int32 { return x.targetPlayer }

// SetTargetPlayer sets the player this orb should fly toward.
func (x *XPOrb) SetTargetPlayer(id int32) { x.targetPlayer = id }

// ProtocolType returns the protocol entity type ID for experience orbs.
func (x *XPOrb) ProtocolType() int32 {
	return genentity.EntityExperienceOrb
}

// SpawnData returns the XP value as spawn data for the protocol.
func (x *XPOrb) SpawnData() int32 {
	return x.xpValue
}

// TryMerge attempts to merge another orb into this one.
// Returns true if successful and other should be removed.
func (x *XPOrb) TryMerge(other *XPOrb) bool {
	if x == nil || other == nil || x.removed || other.removed {
		return false
	}
	x.xpValue += other.xpValue
	x.age = min32(x.age, other.age)
	return true
}

// Tick advances the experience orb by one tick.
func (x *XPOrb) Tick(world physics.BlockAccess, findNearestPlayer func(pos Vec3, radius float64) (int32, Vec3, bool)) {
	if x == nil || x.removed {
		return
	}

	x.age++
	if x.age >= xpOrbDespawnTicks {
		x.removed = true
		return
	}

	x.vel.Y -= physics.GravityXPOrb
	if x.vel.Y < physics.TerminalVelocity {
		x.vel.Y = physics.TerminalVelocity
	}

	if findNearestPlayer != nil {
		playerID, playerPos, found := findNearestPlayer(x.pos, 8.0)
		if found {
			x.targetPlayer = playerID
			dx := playerPos.X - x.pos.X
			dy := playerPos.Y + 0.5 - x.pos.Y
			dz := playerPos.Z - x.pos.Z
			dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
			if dist > 0.1 {
				factor := xpOrbFlySpeed / dist
				x.vel.X += dx * factor
				x.vel.Y += dy * factor
				x.vel.Z += dz * factor
			}
		} else {
			x.targetPlayer = -1
		}
	}

	dims := physics.XPOrbDimensions()
	box := dims.MakeBoundingBox(x.pos.X, x.pos.Y, x.pos.Z)
	result := physics.MoveWithCollision(world, box, x.vel.X, x.vel.Y, x.vel.Z)

	x.pos.X += result.Dx
	x.pos.Y += result.Dy
	x.pos.Z += result.Dz
	x.dirty |= DirtyPosition

	x.onGround = result.OnGround

	if result.CollidedX {
		x.vel.X = 0
	}
	if result.CollidedY {
		x.vel.Y = 0
	}
	if result.CollidedZ {
		x.vel.Z = 0
	}

	x.vel.X *= physics.DragXPOrb
	x.vel.Z *= physics.DragXPOrb

	if x.onGround {
		x.vel.Y *= -0.5
		if math.Abs(x.vel.Y) < 0.01 {
			x.vel.Y = 0
		}
	}

	if x.pos.Y < -128 {
		x.removed = true
	}
}

// IsInPickupRange returns whether the orb is within pickup distance of a position.
func (x *XPOrb) IsInPickupRange(pos Vec3) bool {
	dx := x.pos.X - pos.X
	dy := x.pos.Y - pos.Y
	dz := x.pos.Z - pos.Z
	return dx*dx+dy*dy+dz*dz <= xpOrbPickupDistance*xpOrbPickupDistance
}

// XPOrbManager manages all experience orb entities in a world.
type XPOrbManager struct {
	orbs map[int32]*XPOrb
}

// NewXPOrbManager creates a new experience orb manager.
func NewXPOrbManager() *XPOrbManager {
	return &XPOrbManager{
		orbs: make(map[int32]*XPOrb, 32),
	}
}

// Add registers an XP orb.
func (m *XPOrbManager) Add(orb *XPOrb) {
	if m == nil || orb == nil {
		return
	}
	m.orbs[orb.ID()] = orb
}

// Remove removes an XP orb by ID.
func (m *XPOrbManager) Remove(id int32) {
	if m == nil {
		return
	}
	delete(m.orbs, id)
}

// Get returns an XP orb by ID.
func (m *XPOrbManager) Get(id int32) *XPOrb {
	if m == nil {
		return nil
	}
	return m.orbs[id]
}

// All returns all XP orbs.
func (m *XPOrbManager) All() map[int32]*XPOrb {
	if m == nil {
		return nil
	}
	return m.orbs
}

// Count returns the number of XP orbs.
func (m *XPOrbManager) Count() int {
	if m == nil {
		return 0
	}
	return len(m.orbs)
}

// TickAll advances all orb entities and handles merging and removal.
func (m *XPOrbManager) TickAll(world physics.BlockAccess, findPlayer func(pos Vec3, radius float64) (int32, Vec3, bool)) []int32 {
	if m == nil {
		return nil
	}

	var removed []int32

	for _, orb := range m.orbs {
		orb.Tick(world, findPlayer)
	}

	for id, orb := range m.orbs {
		if orb.removed {
			removed = append(removed, id)
			continue
		}

		if orb.age%20 == 0 {
			for othID, other := range m.orbs {
				if othID == id || other.removed {
					continue
				}
				if !orb.IsInPickupRange(other.pos) {
					continue
				}
				if orb.TryMerge(other) {
					other.removed = true
					removed = append(removed, othID)
				}
			}
		}
	}

	for _, id := range removed {
		delete(m.orbs, id)
	}

	return removed
}
