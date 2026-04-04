package entity

import (
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// PacketSender is the subset of session.Session needed by the entity system.
type PacketSender interface {
	Send(packet protocol.Packet) error
}

// Player extends LivingEntity with session binding and entity tracking state.
type Player struct {
	*LivingEntity

	session      PacketSender
	username     string
	viewDistance int32

	trackedEntities map[int32]struct{}

	visibleBuf []int32
	spawnBuf   []int32
	despawnBuf []int32
}

// NewPlayer creates a player entity bound to a session.
func NewPlayer(id int32, uuid protocol.UUID, username string, pos Vec3, rot Vec2, session PacketSender, viewDistance int32) *Player {
	le := NewLivingEntity(NewEntity(id, uuid, EntityTypePlayer, pos, rot), 20.0)
	le.Entity.SetProtocolInfo(147, 0)
	return &Player{
		LivingEntity:    le,
		session:         session,
		username:        username,
		viewDistance:    viewDistance,
		trackedEntities: make(map[int32]struct{}, 64),
	}
}

// Living returns the underlying LivingEntity for health/damage access.
func (p *Player) Living() *LivingEntity { return p.LivingEntity }

// Session returns the packet sender bound to this player.
func (p *Player) Session() PacketSender { return p.session }

// Username returns the player's username.
func (p *Player) Username() string { return p.username }

// ViewDistance returns the player's view distance in chunks.
func (p *Player) ViewDistance() int32 { return p.viewDistance }

// SetViewDistance updates the player's view distance.
func (p *Player) SetViewDistance(d int32) { p.viewDistance = d }

// SetPositionXYZ updates position from individual coordinates (satisfies session.MovablePlayer).
func (p *Player) SetPositionXYZ(x, y, z float64) {
	p.Entity.SetPosition(Vec3{X: x, Y: y, Z: z})
}

// SetRotationYP updates rotation from yaw/pitch (satisfies session.MovablePlayer).
func (p *Player) SetRotationYP(yaw, pitch float32) {
	p.Entity.SetRotation(Vec2{X: yaw, Y: pitch})
}

// TrackedEntities returns the set of entity IDs currently tracked by this player.
func (p *Player) TrackedEntities() map[int32]struct{} { return p.trackedEntities }

// VisibleChunkKeys computes chunk keys within the player's view distance.
// Results are appended to the reusable visibleBuf and the slice is returned.
func (p *Player) VisibleChunkKeys() []int64 {
	cx := p.ChunkX()
	cz := p.ChunkZ()
	d := p.viewDistance

	count := int((2*d + 1) * (2*d + 1))
	keys := make([]int64, 0, count)

	for dx := -d; dx <= d; dx++ {
		for dz := -d; dz <= d; dz++ {
			keys = append(keys, ChunkKey(cx+dx, cz+dz))
		}
	}
	return keys
}

// UpdateTracking diffs the currently visible entities against tracked entities
// and populates spawn/despawn buffers.
func (p *Player) UpdateTracking(mgr *Manager) (spawns []int32, despawns []int32) {
	p.visibleBuf = p.visibleBuf[:0]
	p.spawnBuf = p.spawnBuf[:0]
	p.despawnBuf = p.despawnBuf[:0]

	chunkKeys := p.VisibleChunkKeys()
	for _, key := range chunkKeys {
		ids := mgr.EntitiesInChunk(key)
		for _, eid := range ids {
			if eid == p.ID() {
				continue
			}
			p.visibleBuf = append(p.visibleBuf, eid)
		}
	}

	visible := make(map[int32]struct{}, len(p.visibleBuf))
	for _, eid := range p.visibleBuf {
		visible[eid] = struct{}{}
	}

	for _, eid := range p.visibleBuf {
		if _, tracked := p.trackedEntities[eid]; !tracked {
			p.spawnBuf = append(p.spawnBuf, eid)
			p.trackedEntities[eid] = struct{}{}
		}
	}

	for eid := range p.trackedEntities {
		if _, vis := visible[eid]; !vis {
			p.despawnBuf = append(p.despawnBuf, eid)
		}
	}
	for _, eid := range p.despawnBuf {
		delete(p.trackedEntities, eid)
	}

	return p.spawnBuf, p.despawnBuf
}

// SendSpawnEntity sends a SpawnEntity packet for the given entity.
func (p *Player) SendSpawnEntity(e *Entity) {
	if p.session == nil || e == nil {
		return
	}
	pos := e.Position()
	rot := e.Rotation()
	vel := e.Velocity()

	pkt := &play.SpawnEntity{
		EntityID:   e.ID(),
		EntityUUID: e.UUID(),
		Type:       e.ProtocolType(),
		X:          pos.X,
		Y:          pos.Y,
		Z:          pos.Z,
		Pitch:      AngleToByte(rot.Y),
		Yaw:        AngleToByte(rot.X),
		HeadYaw:    AngleToByte(rot.X),
		Data:       e.SpawnData(),
		VelocityX:  velocityToProtocol(vel.X),
		VelocityY:  velocityToProtocol(vel.Y),
		VelocityZ:  velocityToProtocol(vel.Z),
	}
	_ = p.session.Send(pkt)
}

// SendDespawnEntities sends a RemoveEntities packet for the given entity IDs.
func (p *Player) SendDespawnEntities(ids []int32) {
	if p.session == nil || len(ids) == 0 {
		return
	}
	pkt := &play.RemoveEntities{EntityIDs: ids}
	_ = p.session.Send(pkt)
}

// SendMovementUpdate sends the appropriate movement packet for a dirty entity.
func (p *Player) SendMovementUpdate(e *Entity) {
	if p.session == nil || e == nil {
		return
	}

	posDirty := e.Dirty()&DirtyPosition != 0
	rotDirty := e.Dirty()&DirtyRotation != 0

	if !posDirty && !rotDirty {
		return
	}

	if posDirty {
		dx, dy, dz, fits := PositionDelta(e.PrevPosition(), e.Position())
		if !fits {
			p.sendTeleport(e)
			return
		}

		if rotDirty {
			rot := e.Rotation()
			pkt := &play.UpdateEntityPositionAndRotation{
				EntityID: e.ID(),
				DeltaX:   dx,
				DeltaY:   dy,
				DeltaZ:   dz,
				Yaw:      AngleToByte(rot.X),
				Pitch:    AngleToByte(rot.Y),
				OnGround: e.OnGround(),
			}
			_ = p.session.Send(pkt)

			headPkt := &play.SetHeadRotation{
				EntityID: e.ID(),
				HeadYaw:  AngleToByte(rot.X),
			}
			_ = p.session.Send(headPkt)
			return
		}

		pkt := &play.UpdateEntityPosition{
			EntityID: e.ID(),
			DeltaX:   dx,
			DeltaY:   dy,
			DeltaZ:   dz,
			OnGround: e.OnGround(),
		}
		_ = p.session.Send(pkt)
		return
	}

	rot := e.Rotation()
	pkt := &play.UpdateEntityRotation{
		EntityID: e.ID(),
		Yaw:      AngleToByte(rot.X),
		Pitch:    AngleToByte(rot.Y),
		OnGround: e.OnGround(),
	}
	_ = p.session.Send(pkt)

	headPkt := &play.SetHeadRotation{
		EntityID: e.ID(),
		HeadYaw:  AngleToByte(rot.X),
	}
	_ = p.session.Send(headPkt)
}

func (p *Player) sendTeleport(e *Entity) {
	pos := e.Position()
	rot := e.Rotation()
	pkt := &play.TeleportEntity{
		EntityID: e.ID(),
		X:        pos.X,
		Y:        pos.Y,
		Z:        pos.Z,
		Yaw:      rot.X,
		Pitch:    rot.Y,
		OnGround: e.OnGround(),
	}
	_ = p.session.Send(pkt)
}

func velocityToProtocol(v float64) int16 {
	scaled := v * 8000.0
	if scaled > 32767 {
		return 32767
	}
	if scaled < -32768 {
		return -32768
	}
	return int16(scaled)
}
