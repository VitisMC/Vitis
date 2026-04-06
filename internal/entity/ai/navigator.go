package ai

import (
	"math"

	"github.com/vitismc/vitis/internal/entity"
)

// Navigator manages path-following for a single mob.
type Navigator struct {
	path       *Path
	speed      float64
	replanTick int
}

// NewNavigator creates a navigator with the given movement speed.
func NewNavigator(speed float64) *Navigator {
	return &Navigator{speed: speed}
}

// HasPath returns true if the navigator has an active path.
func (n *Navigator) HasPath() bool {
	return n.path != nil && !n.path.Done()
}

// SetPath assigns a new path to follow.
func (n *Navigator) SetPath(path *Path) {
	n.path = path
}

// ClearPath removes the current path.
func (n *Navigator) ClearPath() {
	n.path = nil
}

// FollowPath moves the mob along the current path for one tick.
// Returns true if the mob reached the current waypoint.
func (n *Navigator) FollowPath(mob *entity.MobEntity) bool {
	if n.path == nil || n.path.Done() {
		return false
	}

	node := n.path.Current()
	pos := mob.Position()

	tx := float64(node.X) + 0.5
	ty := float64(node.Y)
	tz := float64(node.Z) + 0.5

	dx := tx - pos.X
	dy := ty - pos.Y
	dz := tz - pos.Z
	horizDistSq := dx*dx + dz*dz

	speed := n.speed
	if speed <= 0 {
		speed = 0.15
	}

	if horizDistSq < speed*speed+0.1 && math.Abs(dy) < 1.5 {
		mob.SetPosition(entity.Vec3{X: tx, Y: ty, Z: tz})
		n.path.Advance()
		return true
	}

	horizDist := math.Sqrt(horizDistSq)
	if horizDist > 0.01 {
		nx := dx / horizDist * speed
		nz := dz / horizDist * speed

		newY := pos.Y
		if dy > 0.5 {
			newY += speed
		} else if dy < -0.5 {
			newY -= speed
		}

		yaw := float32(math.Atan2(-nx, nz) * 180.0 / math.Pi)
		pitch := float32(-math.Atan2(dy, horizDist) * 180.0 / math.Pi)
		mob.SetRotation(entity.Vec2{X: yaw, Y: pitch})
		mob.SetPosition(entity.Vec3{X: pos.X + nx, Y: newY, Z: pos.Z + nz})
	}

	return false
}

// NavigateTo computes a path to the target and begins following it.
func (n *Navigator) NavigateTo(mob *entity.MobEntity, target entity.Vec3, blocks BlockAccess) bool {
	if blocks == nil {
		return false
	}

	pos := mob.Position()
	start := PathNode{
		X: int(math.Floor(pos.X)),
		Y: int(math.Floor(pos.Y)),
		Z: int(math.Floor(pos.Z)),
	}
	goal := PathNode{
		X: int(math.Floor(target.X)),
		Y: int(math.Floor(target.Y)),
		Z: int(math.Floor(target.Z)),
	}

	if start == goal {
		return true
	}

	def := mob.TypeDef()
	path := FindPath(blocks, start, goal, def.Width, def.Height)
	if path == nil {
		return false
	}

	n.path = path
	return true
}
