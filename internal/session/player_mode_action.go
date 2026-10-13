package session

import (
	pose "github.com/vitismc/vitis/internal/data/generated/entity_pose"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/metadata"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// playerModeCommand orders movement-affecting pose and sprint actions on the world tick.
type playerModeCommand struct {
	session Session
	players *PlayerManager
	world   WorldAccessor
	request play.EntityAction
}

// OwnerSession prevents queued mode changes from crossing a world binding change.
func (c *playerModeCommand) OwnerSession() Session { return c.session }

// Execute validates ownership and publishes authoritative player flags and pose.
func (c *playerModeCommand) Execute() {
	if c.session == nil || c.session.Context().Err() != nil || c.players == nil {
		return
	}
	online := c.players.GetBySession(c.session)
	player, ok := c.session.Player().(*entity.Player)
	if online == nil || !ok || player == nil || player.IsDead() || c.request.EntityID != player.ID() {
		return
	}
	switch c.request.ActionID {
	case 0:
		online.Flags |= 0x02
	case 1:
		online.Flags &^= 0x02
	case 2:
		(&wakeBedCommand{session: c.session, players: c.players, world: c.world}).Execute()
		return
	case 3:
		online.Flags |= 0x08
	case 4:
		online.Flags &^= 0x08
	case 8:
		geometry, ok := c.world.(movementGeometry)
		if !ok || !geometry.HasMovementGeometry() || !canGlide(c.session, online, player, geometry) {
			return
		}
		online.Flags |= 0x80
	default:
		return
	}
	if geometry, ok := c.world.(movementGeometry); ok && geometry.HasMovementGeometry() {
		updateMovementPose(c.session, c.players, online, player, geometry)
	} else if online.Flags&0x02 != 0 {
		online.Pose = pose.PoseCrouching
	} else {
		online.Pose = pose.PoseStanding
	}
	data := metadata.New()
	data.SetByte(metadata.IndexBase, int8(online.Flags))
	data.SetPose(metadata.IndexPose, online.Pose)
	c.players.BroadcastExcept(online.UUID, &play.SetEntityMetadata{EntityID: online.EntityID, Metadata: data.Encode()})
}
