package physics

import (
	"math"

	gencollision "github.com/vitismc/vitis/internal/data/generated/collision"
)

// BlockAccess provides read access to block state IDs in the world.
type BlockAccess interface {
	GetBlockStateAt(x, y, z int) int32
}

// CollectBlockCollisions gathers all block AABBs that intersect the given AABB.
// The blockAABBs are in world coordinates (offset by block position).
func CollectBlockCollisions(world BlockAccess, box AABB) []AABB {
	minBX := int(math.Floor(box.MinX))
	minBY := int(math.Floor(box.MinY))
	minBZ := int(math.Floor(box.MinZ))
	maxBX := int(math.Floor(box.MaxX))
	maxBY := int(math.Floor(box.MaxY))
	maxBZ := int(math.Floor(box.MaxZ))

	var result []AABB
	for bx := minBX; bx <= maxBX; bx++ {
		for by := minBY; by <= maxBY; by++ {
			for bz := minBZ; bz <= maxBZ; bz++ {
				stateID := world.GetBlockStateAt(bx, by, bz)
				shapes := gencollision.ShapesForStateID(stateID)
				for _, s := range shapes {
					worldBox := AABB{
						MinX: float64(bx) + s.MinX,
						MinY: float64(by) + s.MinY,
						MinZ: float64(bz) + s.MinZ,
						MaxX: float64(bx) + s.MaxX,
						MaxY: float64(by) + s.MaxY,
						MaxZ: float64(bz) + s.MaxZ,
					}
					result = append(result, worldBox)
				}
			}
		}
	}
	return result
}

// MoveResult holds the outcome of a collision-checked movement.
type MoveResult struct {
	Dx, Dy, Dz float64
	OnGround    bool
	CollidedX   bool
	CollidedY   bool
	CollidedZ   bool
}

// MoveWithCollision resolves entity movement against world block collisions.
// Takes the entity bounding box and desired movement vector.
// Returns the actual movement after collision clipping.
func MoveWithCollision(world BlockAccess, entityBox AABB, dx, dy, dz float64) MoveResult {
	origDx, origDy, origDz := dx, dy, dz

	sweepBox := entityBox.Expand(dx, dy, dz)
	colliders := CollectBlockCollisions(world, sweepBox)

	if len(colliders) == 0 {
		return MoveResult{
			Dx: dx, Dy: dy, Dz: dz,
			OnGround: false,
		}
	}

	for _, c := range colliders {
		dy = entityBox.ClipYCollide(c, dy)
	}
	entityBox = entityBox.Offset(0, dy, 0)

	for _, c := range colliders {
		dx = entityBox.ClipXCollide(c, dx)
	}
	entityBox = entityBox.Offset(dx, 0, 0)

	for _, c := range colliders {
		dz = entityBox.ClipZCollide(c, dz)
	}

	return MoveResult{
		Dx:        dx,
		Dy:        dy,
		Dz:        dz,
		OnGround:  origDy < 0 && dy > origDy,
		CollidedX: origDx != dx,
		CollidedY: origDy != dy,
		CollidedZ: origDz != dz,
	}
}

// MoveWithStepUp attempts movement with collision, and if horizontally blocked
// while on ground, tries stepping up by stepHeight.
func MoveWithStepUp(world BlockAccess, entityBox AABB, dx, dy, dz, stepHeight float64, onGround bool) MoveResult {
	result := MoveWithCollision(world, entityBox, dx, dy, dz)

	if !onGround || (!result.CollidedX && !result.CollidedZ) {
		return result
	}

	stepResult := MoveWithCollision(world, entityBox, 0, stepHeight, 0)
	steppedBox := entityBox.Offset(0, stepResult.Dy, 0)

	horizResult := MoveWithCollision(world, steppedBox, dx, 0, dz)
	steppedBox = steppedBox.Offset(horizResult.Dx, 0, horizResult.Dz)

	downResult := MoveWithCollision(world, steppedBox, 0, -stepHeight, 0)

	totalDx := horizResult.Dx
	totalDy := stepResult.Dy + downResult.Dy + dy
	totalDz := horizResult.Dz

	horizDistSq := totalDx*totalDx + totalDz*totalDz
	origHorizDistSq := result.Dx*result.Dx + result.Dz*result.Dz

	if horizDistSq <= origHorizDistSq {
		return result
	}

	return MoveResult{
		Dx:        totalDx,
		Dy:        totalDy,
		Dz:        totalDz,
		OnGround:  true,
		CollidedX: dx != totalDx,
		CollidedY: dy != totalDy,
		CollidedZ: dz != totalDz,
	}
}

// CheckOnGround tests whether the entity is resting on a solid surface.
func CheckOnGround(world BlockAccess, entityBox AABB) bool {
	testBox := entityBox.Offset(0, -0.04, 0)
	testBox.MaxY = entityBox.MinY
	colliders := CollectBlockCollisions(world, testBox)
	return len(colliders) > 0
}

// ApplyGravityAndDrag applies vertical gravity and horizontal drag to velocity components.
func ApplyGravityAndDrag(velX, velY, velZ, gravity, drag, terminal float64) (float64, float64, float64) {
	velY -= gravity
	if velY < terminal {
		velY = terminal
	}
	velX *= drag
	velZ *= drag
	return velX, velY, velZ
}

// ApplyGroundDrag applies drag when the entity is on the ground.
// Vanilla uses different drag on ground vs in air.
func ApplyGroundDrag(velX, velY, velZ, friction float64) (float64, float64, float64) {
	factor := 0.91 * friction
	velX *= factor
	velZ *= factor
	velY *= 0.98
	return velX, velY, velZ
}

// ApplyAirDrag applies drag when the entity is airborne.
func ApplyAirDrag(velX, velY, velZ float64) (float64, float64, float64) {
	velX *= 0.91
	velZ *= 0.91
	velY *= 0.98
	return velX, velY, velZ
}

// IsCollidingAt checks if an AABB at the given position would collide with any block.
func IsCollidingAt(world BlockAccess, box AABB) bool {
	colliders := CollectBlockCollisions(world, box)
	for _, c := range colliders {
		if box.Intersects(c) {
			return true
		}
	}
	return false
}
