package entity

import (
	"math"
	"github.com/vitismc/vitis/internal/protocol"
)

// EntityType identifies the kind of entity.
type EntityType uint8

const (
	EntityTypePlayer EntityType = iota
	EntityTypeMob
)

const (
	DirtyPosition uint8 = 1 << iota
	DirtyRotation
	DirtyVelocity
)

// Vec3 represents a 3D position or velocity with float64 components.
type Vec3 struct {
	X float64
	Y float64
	Z float64
}

// Vec2 represents yaw/pitch rotation with float32 components.
type Vec2 struct {
	X float32
	Y float32
}

// Entity is the base entity structure owned by the world tick goroutine.
type Entity struct {
	id         int32
	uuid       protocol.UUID
	entityType EntityType

	protocolType int32
	spawnData    int32

	pos      Vec3
	rot      Vec2
	vel      Vec3
	onGround bool

	prevPos Vec3
	prevRot Vec2

	chunkX int32
	chunkZ int32

	dirty uint8

	clientSimulated bool
	removed         bool
}

// NewEntity creates a base entity with the given parameters.
func NewEntity(id int32, uuid protocol.UUID, entityType EntityType, pos Vec3, rot Vec2) *Entity {
	cx := BlockToChunk(pos.X)
	cz := BlockToChunk(pos.Z)
	return &Entity{
		id:         id,
		uuid:       uuid,
		entityType: entityType,
		pos:        pos,
		rot:        rot,
		prevPos:    pos,
		prevRot:    rot,
		chunkX:     cx,
		chunkZ:     cz,
	}
}

// ID returns the entity's protocol-level identifier.
func (e *Entity) ID() int32 { return e.id }

// UUID returns the entity's unique identifier.
func (e *Entity) UUID() protocol.UUID { return e.uuid }

// Type returns the entity type.
func (e *Entity) Type() EntityType { return e.entityType }

// ProtocolType returns the protocol entity type ID for SpawnEntity packets.
func (e *Entity) ProtocolType() int32 { return e.protocolType }

// SpawnData returns the extra data sent in SpawnEntity packets (e.g. block state for falling blocks).
func (e *Entity) SpawnData() int32 { return e.spawnData }

// SetProtocolInfo sets the protocol type and spawn data for this entity.
func (e *Entity) SetProtocolInfo(protocolType, spawnData int32) {
	e.protocolType = protocolType
	e.spawnData = spawnData
}

// Position returns the current position.
func (e *Entity) Position() Vec3 { return e.pos }

// SetPosition updates position and marks the entity as dirty.
func (e *Entity) SetPosition(pos Vec3) {
	e.pos = pos
	e.dirty |= DirtyPosition
}

// Rotation returns the current yaw/pitch rotation.
func (e *Entity) Rotation() Vec2 { return e.rot }

// SetRotation updates rotation and marks the entity as dirty.
func (e *Entity) SetRotation(rot Vec2) {
	e.rot = rot
	e.dirty |= DirtyRotation
}

// Velocity returns the current velocity.
func (e *Entity) Velocity() Vec3 { return e.vel }

// SetVelocity updates velocity and marks the entity as dirty.
func (e *Entity) SetVelocity(vel Vec3) {
	e.vel = vel
	e.dirty |= DirtyVelocity
}

// OnGround returns whether the entity is on the ground.
func (e *Entity) OnGround() bool { return e.onGround }

// SetOnGround updates the on-ground flag.
func (e *Entity) SetOnGround(v bool) { e.onGround = v }

// PrevPosition returns the position snapshot from the previous tick.
func (e *Entity) PrevPosition() Vec3 { return e.prevPos }

// PrevRotation returns the rotation snapshot from the previous tick.
func (e *Entity) PrevRotation() Vec2 { return e.prevRot }

// ChunkX returns the cached chunk X coordinate.
func (e *Entity) ChunkX() int32 { return e.chunkX }

// ChunkZ returns the cached chunk Z coordinate.
func (e *Entity) ChunkZ() int32 { return e.chunkZ }

// Dirty returns the current dirty bitmask.
func (e *Entity) Dirty() uint8 { return e.dirty }

// ClearDirty resets the dirty bitmask to zero.
func (e *Entity) ClearDirty() { e.dirty = 0 }

// SnapshotPrev stores current position and rotation as previous-tick values.
func (e *Entity) SnapshotPrev() {
	e.prevPos = e.pos
	e.prevRot = e.rot
}

// ClientSimulated returns whether the entity's physics are simulated client-side.
func (e *Entity) ClientSimulated() bool { return e.clientSimulated }

// SetClientSimulated marks the entity as client-simulated (no server movement updates).
func (e *Entity) SetClientSimulated(v bool) { e.clientSimulated = v }

// Removed returns whether the entity has been marked for removal.
func (e *Entity) Removed() bool { return e.removed }

// Remove marks the entity for removal.
func (e *Entity) Remove() { e.removed = true }

// UpdateChunkCoords recalculates cached chunk coordinates from current position.
// Returns true if chunk membership changed.
func (e *Entity) UpdateChunkCoords() bool {
	cx := BlockToChunk(e.pos.X)
	cz := BlockToChunk(e.pos.Z)
	if cx == e.chunkX && cz == e.chunkZ {
		return false
	}
	e.chunkX = cx
	e.chunkZ = cz
	return true
}

// BlockToChunk converts a block coordinate to a chunk coordinate using floor division.
func BlockToChunk(v float64) int32 {
	return int32(math.Floor(v / 16))
}

// ChunkKey packs chunk X/Z into a single int64 key.
func ChunkKey(x, z int32) int64 {
	return int64(x)<<32 | int64(uint32(z))
}
