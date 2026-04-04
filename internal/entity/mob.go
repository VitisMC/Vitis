package entity

import "github.com/vitismc/vitis/internal/protocol"

// Mob extends Entity with mob-specific state.
type Mob struct {
	*Entity
	mobType int32
}

// NewMob creates a mob entity with the given type identifier.
func NewMob(id int32, uuid protocol.UUID, mobType int32, pos Vec3, rot Vec2) *Mob {
	return &Mob{
		Entity:  NewEntity(id, uuid, EntityTypeMob, pos, rot),
		mobType: mobType,
	}
}

// MobType returns the mob's protocol type identifier.
func (m *Mob) MobType() int32 { return m.mobType }

// Tick advances mob logic for one tick. Currently a no-op stub.
func (m *Mob) Tick() {}
