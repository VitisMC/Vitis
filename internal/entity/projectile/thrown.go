package projectile

import (
	genentity "github.com/vitismc/vitis/internal/data/generated/entity"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/protocol"
)

// ThrownType identifies the kind of thrown projectile.
type ThrownType int32

const (
	ThrownSnowball     ThrownType = ThrownType(genentity.EntitySnowball)
	ThrownEgg          ThrownType = ThrownType(genentity.EntityEgg)
	ThrownEnderPearl   ThrownType = ThrownType(genentity.EntityEnderPearl)
	ThrownSplashPotion ThrownType = ThrownType(genentity.EntityPotion)
)

// ThrownEntity represents a thrown projectile (snowball, egg, ender pearl, splash potion).
type ThrownEntity struct {
	*Base
	thrownType ThrownType
	itemID     int32
}

// NewThrownEntity creates a thrown projectile.
func NewThrownEntity(id int32, uuid protocol.UUID, thrownType ThrownType, pos, vel entity.Vec3, ownerID int32, ownerUUID protocol.UUID) *ThrownEntity {
	b := NewBase(id, uuid, entity.EntityType(thrownType), pos, vel, ownerID, ownerUUID, physics.GravityThrown, physics.DragThrown)
	spawnData := int32(0)
	if ownerID > 0 {
		spawnData = ownerID + 1
	}
	b.Entity.SetProtocolInfo(int32(thrownType), spawnData)
	return &ThrownEntity{
		Base:       b,
		thrownType: thrownType,
	}
}

// ThrownType returns the type of thrown projectile.
func (t *ThrownEntity) ThrownType() ThrownType { return t.thrownType }

// ItemID returns the item ID associated with this thrown entity (for spawn data).
func (t *ThrownEntity) ItemID() int32 { return t.itemID }

// SetItemID sets the item ID for this thrown entity.
func (t *ThrownEntity) SetItemID(id int32) { t.itemID = id }

// ProtocolType returns the protocol entity type ID.
func (t *ThrownEntity) ProtocolType() int32 {
	return int32(t.thrownType)
}

// SpawnData returns the owner entity ID + 1 for the SpawnEntity packet data field.
func (t *ThrownEntity) SpawnData() int32 {
	if t.OwnerID() > 0 {
		return t.OwnerID() + 1
	}
	return 0
}

// Tick advances the thrown entity by one tick.
func (t *ThrownEntity) Tick(world physics.BlockAccess, finder EntityFinder) {
	dims := physics.EntityDimensions{Width: 0.25, Height: 0.25}
	t.Base.Tick(world, finder, dims)
}
