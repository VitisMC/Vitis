package projectile

import (
	genentity "github.com/vitismc/vitis/internal/data/generated/entity"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/protocol"
)

const (
	TridentBaseDamage = 8.0
	tridentGravity    = 0.05
	tridentDrag       = 0.99
)

// Trident represents a thrown trident entity in the world.
type Trident struct {
	*Base
	damage      float64
	loyalty     int32
	channeling  bool
	returnTimer int32
}

// NewTrident creates a trident projectile shot from the given position/velocity.
func NewTrident(id int32, uuid protocol.UUID, pos, vel entity.Vec3, ownerID int32, ownerUUID protocol.UUID) *Trident {
	b := NewBase(id, uuid, entity.EntityType(genentity.EntityTrident), pos, vel, ownerID, ownerUUID, tridentGravity, tridentDrag)
	b.Entity.SetProtocolInfo(genentity.EntityTrident, ownerID)
	return &Trident{
		Base:   b,
		damage: TridentBaseDamage,
	}
}

// Damage returns the base damage this trident deals on hit.
func (t *Trident) Damage() float64 { return t.damage }

// SetDamage sets the trident's base damage.
func (t *Trident) SetDamage(d float64) { t.damage = d }

// Loyalty returns the loyalty enchantment level (0 = none).
func (t *Trident) Loyalty() int32 { return t.loyalty }

// SetLoyalty sets the loyalty level, causing the trident to return to the owner.
func (t *Trident) SetLoyalty(level int32) { t.loyalty = level }

// Channeling returns whether the trident has the channeling enchantment.
func (t *Trident) Channeling() bool { return t.channeling }

// SetChanneling sets the channeling flag.
func (t *Trident) SetChanneling(c bool) { t.channeling = c }

// ProtocolType returns the protocol entity type ID.
func (t *Trident) ProtocolType() int32 {
	return genentity.EntityTrident
}

// SpawnData returns the owner entity ID + 1 for the SpawnEntity packet data field.
func (t *Trident) SpawnData() int32 {
	if t.OwnerID() > 0 {
		return t.OwnerID() + 1
	}
	return 0
}

// Tick advances the trident by one tick.
func (t *Trident) Tick(world physics.BlockAccess, finder EntityFinder) {
	dims := physics.EntityDimensions{Width: 0.5, Height: 0.5}
	t.Base.Tick(world, finder, dims)

	// Loyalty: return to owner after hitting ground
	if t.loyalty > 0 && t.InGround() {
		t.returnTimer++
		returnDelay := int32((4 - t.loyalty) * 20)
		if returnDelay < 20 {
			returnDelay = 20
		}
		if t.returnTimer > returnDelay {
			t.Entity.Remove()
		}
	}
}

// ComputeHitDamage returns the damage to deal on entity hit.
func (t *Trident) ComputeHitDamage() float64 {
	return t.damage
}
