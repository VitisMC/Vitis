package session

import (
	"fmt"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol/packets/play"
)

// movementCommand applies one validated movement packet on the world tick.
type movementCommand struct {
	session     Session
	world       WorldAccessor
	players     *PlayerManager
	hasPosition bool
	hasRotation bool
	x, y, z     float64
	yaw, pitch  float32
	onGround    bool
}

// teleportAckCommand orders teleport acknowledgement with movement commands.
type teleportAckCommand struct {
	session *DefaultSession
	id      int32
	players *PlayerManager
}

type clientLoadedCommand struct{ session Session }

// Execute marks terrain loading complete on the authoritative world tick.
func (c *clientLoadedCommand) Execute() {
	if c.session == nil {
		return
	}
	if player, ok := c.session.Player().(*entity.Player); ok {
		player.SetClientLoaded(true)
	}
}

// Execute acknowledges only the teleport that is pending at this tick position.
func (c *teleportAckCommand) Execute() {
	if c.session == nil {
		return
	}
	if err := c.session.acknowledgeTeleport(c.id); err != nil {
		_ = c.session.ForceClose(err)
		return
	}
	if c.players != nil {
		if online := c.players.GetBySession(c.session); online != nil {
			online.movement = movementState{}
		}
	}
}

// Execute applies movement in the same order the session submitted packets.
func (m *movementCommand) Execute() {
	player, ok := m.session.Player().(MovablePlayer)
	if !ok || player == nil {
		return
	}
	if concrete, ok := m.session.Player().(*entity.Player); ok && (!concrete.HasClientLoaded() || concrete.IsDead()) {
		return
	}
	var online *OnlinePlayer
	if m.players != nil {
		online = m.players.GetBySession(m.session)
		if online == nil {
			return
		}
	}
	if m.hasPosition {
		accepted, err := m.validateAuthoritativeMovement(online)
		if err != nil {
			_ = m.session.ForceClose(err)
			return
		}
		if !accepted {
			return
		}
		if online != nil {
			fluid := false
			if geometry, ok := m.world.(movementGeometry); ok && geometry.HasMovementGeometry() {
				if concrete, ok := m.session.Player().(*entity.Player); ok {
					fluid = movementFluid(geometry, entity.Vec3{X: m.x, Y: m.y, Z: m.z}, movementDimensions(online).Height)
					if fluid || playerFlying(m.session, concrete) || online.Flags&0x80 != 0 {
						concrete.SetFallDistance(0)
					} else {
						checkFallDamage(m.session, online.Y, m.y, m.onGround)
					}
				}
			} else {
				checkFallDamage(m.session, online.Y, m.y, m.onGround)
			}
			if fluid {
				applySwimmingExhaustion(m.session, online.X, online.Y, online.Z, m.x, m.y, m.z)
			} else {
				applyMovementExhaustion(m.session, online, online.X, online.Y, online.Z, m.x, m.y, m.z, m.onGround)
			}
			online.X, online.Y, online.Z = m.x, m.y, m.z
		}
		player.SetPositionXYZ(m.x, m.y, m.z)
	}
	if geometry, ok := m.world.(movementGeometry); ok && geometry.HasMovementGeometry() {
		if concrete, ok := m.session.Player().(*entity.Player); ok {
			m.onGround = concrete.Living().GameMode() != 3 && movementOnGround(geometry, concrete, online)
		}
	}
	if m.hasRotation {
		player.SetRotationYP(m.yaw, m.pitch)
		if online != nil {
			online.Yaw, online.Pitch = m.yaw, m.pitch
		}
	}
	player.SetOnGround(m.onGround)
	if online == nil || (!m.hasPosition && !m.hasRotation) {
		return
	}
	if online.ManagedTracking {
		return
	}
	update := &play.SyncEntityPosition{
		EntityID: online.EntityID,
		X:        online.X, Y: online.Y, Z: online.Z,
		OnGround: m.onGround,
	}
	if m.hasRotation {
		update.Yaw, update.Pitch = m.yaw, m.pitch
	}
	m.players.BroadcastExcept(online.UUID, update)
	if m.hasRotation {
		m.players.BroadcastExcept(online.UUID, &play.SetHeadRotation{
			EntityID: online.EntityID,
			HeadYaw:  byte(int32(m.yaw*256.0/360.0) & 0xFF),
		})
	}
}

// submitMovement validates untrusted values before entering the bounded world queue.
func submitMovement(s Session, wa WorldAccessor, command movementCommand) error {
	if player, ok := s.Player().(MovablePlayer); !ok || player == nil {
		return nil
	}
	if command.hasPosition && !validPlayerPosition(command.x, command.y, command.z) {
		return fmt.Errorf("invalid player movement values")
	}
	if command.hasRotation && (!finiteFloat32(command.yaw) || !finiteFloat32(command.pitch)) {
		return fmt.Errorf("invalid player rotation")
	}
	command.session = s
	command.world = wa
	if wa == nil || !wa.Submit(&command) {
		return fmt.Errorf("world command queue is unavailable or full")
	}
	return nil
}
