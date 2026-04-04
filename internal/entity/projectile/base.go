package projectile

import (
	"math"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/protocol"
)

// HitResult describes what a projectile hit.
type HitResult struct {
	HitBlock    bool
	HitEntity   bool
	BlockX      int
	BlockY      int
	BlockZ      int
	EntityID    int32
	HitX, HitY, HitZ float64
}

// EntityFinder is used by projectiles to detect entity hits.
type EntityFinder interface {
	FindEntitiesInAABB(box physics.AABB, exclude int32) []EntityHit
}

// EntityHit holds info about an entity within a projectile's sweep box.
type EntityHit struct {
	ID       int32
	Position entity.Vec3
	Width    float64
	Height   float64
}

// Base holds common projectile state shared by arrows, thrown items, etc.
type Base struct {
	*entity.Entity
	ownerID   int32
	ownerUUID protocol.UUID
	gravity   float64
	drag      float64
	inGround  bool
	life      int32
	hitResult *HitResult
}

// NewBase creates a new projectile base.
func NewBase(id int32, uuid protocol.UUID, entityType entity.EntityType, pos entity.Vec3, vel entity.Vec3, ownerID int32, ownerUUID protocol.UUID, gravity, drag float64) *Base {
	e := entity.NewEntity(id, uuid, entityType, pos, entity.Vec2{
		X: float32(math.Atan2(vel.X, vel.Z) * 180 / math.Pi),
		Y: float32(math.Atan2(vel.Y, math.Sqrt(vel.X*vel.X+vel.Z*vel.Z)) * 180 / math.Pi),
	})
	e.SetVelocity(vel)
	return &Base{
		Entity:    e,
		ownerID:   ownerID,
		ownerUUID: ownerUUID,
		gravity:   gravity,
		drag:      drag,
	}
}

// OwnerID returns the entity ID of the shooter.
func (b *Base) OwnerID() int32 { return b.ownerID }

// OwnerUUID returns the UUID of the shooter.
func (b *Base) OwnerUUID() protocol.UUID { return b.ownerUUID }

// InGround returns whether the projectile is stuck in a block.
func (b *Base) InGround() bool { return b.inGround }

// Life returns the number of ticks this projectile has existed.
func (b *Base) Life() int32 { return b.life }

// LastHitResult returns the last hit result, or nil.
func (b *Base) LastHitResult() *HitResult { return b.hitResult }

// ClearHitResult clears the hit result after processing.
func (b *Base) ClearHitResult() { b.hitResult = nil }

// Tick advances the projectile by one tick with physics and hit detection.
func (b *Base) Tick(world physics.BlockAccess, entityFinder EntityFinder, dims physics.EntityDimensions) {
	if b == nil || b.Entity.Removed() {
		return
	}

	b.life++
	b.hitResult = nil

	if b.inGround {
		return
	}

	vel := b.Entity.Velocity()
	pos := b.Entity.Position()

	vel.Y -= b.gravity

	newPos := entity.Vec3{
		X: pos.X + vel.X,
		Y: pos.Y + vel.Y,
		Z: pos.Z + vel.Z,
	}

	blockHit := b.checkBlockHit(world, pos, newPos)
	if blockHit != nil {
		b.hitResult = blockHit
		b.inGround = true
		newPos = entity.Vec3{X: blockHit.HitX, Y: blockHit.HitY, Z: blockHit.HitZ}
		vel = entity.Vec3{}
	} else if entityFinder != nil {
		entityHit := b.checkEntityHit(entityFinder, pos, newPos, dims)
		if entityHit != nil {
			b.hitResult = entityHit
			newPos = entity.Vec3{X: entityHit.HitX, Y: entityHit.HitY, Z: entityHit.HitZ}
		}
	}

	b.Entity.SetPosition(newPos)

	vel.X *= b.drag
	vel.Z *= b.drag
	vel.Y *= b.drag

	b.Entity.SetVelocity(vel)

	if newPos.Y < -128 {
		b.Entity.Remove()
	}

	if b.life > 1200 {
		b.Entity.Remove()
	}
}

func (b *Base) checkBlockHit(world physics.BlockAccess, from, to entity.Vec3) *HitResult {
	steps := int(math.Ceil(math.Max(math.Max(math.Abs(to.X-from.X), math.Abs(to.Y-from.Y)), math.Abs(to.Z-from.Z)) * 4))
	if steps <= 0 {
		steps = 1
	}

	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x := from.X + (to.X-from.X)*t
		y := from.Y + (to.Y-from.Y)*t
		z := from.Z + (to.Z-from.Z)*t

		bx := int(math.Floor(x))
		by := int(math.Floor(y))
		bz := int(math.Floor(z))

		stateID := world.GetBlockStateAt(bx, by, bz)
		if stateID <= 0 {
			continue
		}

		testBox := physics.AABB{
			MinX: x - 0.01, MinY: y - 0.01, MinZ: z - 0.01,
			MaxX: x + 0.01, MaxY: y + 0.01, MaxZ: z + 0.01,
		}

		collisions := physics.CollectBlockCollisions(world, testBox)
		for _, c := range collisions {
			if testBox.Intersects(c) {
				return &HitResult{
					HitBlock: true,
					BlockX:   bx,
					BlockY:   by,
					BlockZ:   bz,
					HitX:     x,
					HitY:     y,
					HitZ:     z,
				}
			}
		}
	}
	return nil
}

func (b *Base) checkEntityHit(finder EntityFinder, from, to entity.Vec3, dims physics.EntityDimensions) *HitResult {
	halfW := dims.Width / 2
	sweepMinX := math.Min(from.X, to.X) - halfW - 1
	sweepMinY := math.Min(from.Y, to.Y) - 0.5
	sweepMinZ := math.Min(from.Z, to.Z) - halfW - 1
	sweepMaxX := math.Max(from.X, to.X) + halfW + 1
	sweepMaxY := math.Max(from.Y, to.Y) + dims.Height + 0.5
	sweepMaxZ := math.Max(from.Z, to.Z) + halfW + 1

	sweepBox := physics.AABB{
		MinX: sweepMinX, MinY: sweepMinY, MinZ: sweepMinZ,
		MaxX: sweepMaxX, MaxY: sweepMaxY, MaxZ: sweepMaxZ,
	}

	hits := finder.FindEntitiesInAABB(sweepBox, b.ownerID)

	var closest *HitResult
	closestDist := math.MaxFloat64

	for _, hit := range hits {
		ehW := hit.Width / 2
		entityBox := physics.AABB{
			MinX: hit.Position.X - ehW,
			MinY: hit.Position.Y,
			MinZ: hit.Position.Z - ehW,
			MaxX: hit.Position.X + ehW,
			MaxY: hit.Position.Y + hit.Height,
			MaxZ: hit.Position.Z + ehW,
		}
		entityBox = entityBox.Grow(0.3)

		if !entityBox.Contains(from.X, from.Y, from.Z) && !entityBox.Intersects(sweepBox) {
			dx := hit.Position.X - from.X
			dy := hit.Position.Y - from.Y
			dz := hit.Position.Z - from.Z
			dist := dx*dx + dy*dy + dz*dz
			if dist < closestDist {
				closestDist = dist
				closest = &HitResult{
					HitEntity: true,
					EntityID:  hit.ID,
					HitX:      hit.Position.X,
					HitY:      hit.Position.Y,
					HitZ:      hit.Position.Z,
				}
			}
		}
	}

	return closest
}
