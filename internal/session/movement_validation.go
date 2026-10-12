package session

import (
	"fmt"
	"math"

	entitypose "github.com/vitismc/vitis/internal/data/generated/entity_pose"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/physics"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// movementGeometry provides resident collision data and the authoritative tick.
type movementGeometry interface {
	physics.BlockAccess
	MovementTick() uint64
	HasMovementGeometry() bool
}

// movementState tracks cumulative displacement within one server tick.
type movementState struct {
	tick          uint64
	initialized   bool
	start         entity.Vec3
	packets       int
	floatingTicks int
	descending    bool
	glidingTicks  int
}

// MovementTick returns the tick used to bound cumulative client movement.
func (a *DefaultWorldAccessor) MovementTick() uint64 {
	if a.CurrentTick != nil {
		return a.CurrentTick()
	}
	return 0
}

// HasMovementGeometry reports whether the accessor owns collision data.
func (a *DefaultWorldAccessor) HasMovementGeometry() bool { return a != nil && a.Chunks != nil }

// GetBlockStateAt reads resident collision states without initiating chunk loads.
func (a *DefaultWorldAccessor) GetBlockStateAt(x, y, z int) int32 {
	if a == nil || a.Chunks == nil {
		return -1
	}
	c, ok := a.Chunks.PeekLoaded(int32(x>>4), int32(z>>4))
	if !ok || c.Data() == nil {
		return -1
	}
	if y < c.Data().MinY() || y > c.Data().MaxY() {
		return 0
	}
	return c.GetBlock(x, y, z)
}

// movementDimensions resolves the player's current collision dimensions.
func movementDimensions(online *OnlinePlayer) physics.EntityDimensions {
	d := physics.PlayerDimensions()
	if online != nil {
		switch online.Pose {
		case entitypose.PoseCrouching:
			d.Height = 1.5
		case entitypose.PoseSwimming, entitypose.PoseFallFlying, entitypose.PoseSpinAttack:
			d.Height = .6
		}
	}
	return d
}

// movementOnGround derives support from server collision shapes.
func movementOnGround(world physics.BlockAccess, player *entity.Player, online *OnlinePlayer) bool {
	pos := player.Position()
	return physics.CheckOnGround(world, movementDimensions(online).MakeBoundingBox(pos.X, pos.Y, pos.Z))
}

// validateAuthoritativeMovement rejects excessive or obstructed movement before effects.
func (m *movementCommand) validateAuthoritativeMovement(online *OnlinePlayer) (bool, error) {
	geometry, ok := m.world.(movementGeometry)
	player, concrete := m.session.Player().(*entity.Player)
	if !ok || !geometry.HasMovementGeometry() || !concrete || online == nil {
		return validateMovement(m.session, m.players, m.x, m.y, m.z, m.yaw, m.pitch, m.hasRotation)
	}
	if !validPlayerPosition(m.x, m.y, m.z) || (m.hasRotation && (!finiteFloat32(m.yaw) || !finiteFloat32(m.pitch))) {
		return false, fmt.Errorf("invalid player movement values")
	}
	if ds, ok := m.session.(*DefaultSession); ok && ds.hasPendingTeleport() {
		return false, nil
	}
	updateMovementPose(m.session, m.players, online, player, geometry)
	pos := player.Position()
	state := &online.movement
	tick := geometry.MovementTick()
	if !state.initialized || state.tick != tick {
		*state = movementState{tick: tick, initialized: true, start: pos, floatingTicks: state.floatingTicks, glidingTicks: state.glidingTicks}
	}
	state.packets++
	count := state.packets
	if count > 5 {
		count = 1
	}
	dx, dy, dz := m.x-state.start.X, m.y-state.start.Y, m.z-state.start.Z
	vel := player.Velocity()
	limit := 100.0
	if online.Pose == entitypose.PoseFallFlying {
		limit = 300
	}
	if dx*dx+dy*dy+dz*dz-(vel.X*vel.X+vel.Y*vel.Y+vel.Z*vel.Z) > limit*float64(count) {
		return false, correctMovement(m.session, player, online)
	}
	if player.Living().GameMode() == 3 {
		if geometry.GetBlockStateAt(int(math.Floor(m.x)), int(math.Floor(m.y)), int(math.Floor(m.z))) < 0 {
			return false, correctMovement(m.session, player, online)
		}
		m.onGround = false
		return true, nil
	}
	box := movementDimensions(online).MakeBoundingBox(pos.X, pos.Y, pos.Z)
	dx, dy, dz = m.x-pos.X, m.y-pos.Y, m.z-pos.Z
	result := physics.MoveWithStepUp(geometry, box, dx, dy, dz, .6, physics.CheckOnGround(geometry, box))
	rx, ry, rz := dx-result.Dx, dy-result.Dy, dz-result.Dz
	if rx*rx+ry*ry+rz*rz > .0625 {
		return false, correctMovement(m.session, player, online)
	}
	destination := movementDimensions(online).MakeBoundingBox(m.x, m.y, m.z)
	if len(physics.CollectBlockCollisions(geometry, destination.Contract(1e-7))) != 0 && len(physics.CollectBlockCollisions(geometry, box.Contract(1e-7))) == 0 {
		return false, correctMovement(m.session, player, online)
	}
	m.onGround = physics.CheckOnGround(geometry, destination)
	state.descending = m.y-state.start.Y < -.03125
	return true, nil
}

// correctMovement restores the last accepted position using teleport acknowledgement.
func correctMovement(s Session, player *entity.Player, online *OnlinePlayer) error {
	id := nextTeleportID()
	if ds, ok := s.(*DefaultSession); ok {
		ds.setPendingTeleport(id)
	}
	online.movement = movementState{}
	pos, rot := player.Position(), player.Rotation()
	return s.Send(&playpacket.SyncPlayerPosition{TeleportID: id, X: pos.X, Y: pos.Y, Z: pos.Z, Yaw: rot.X, Pitch: rot.Y})
}
