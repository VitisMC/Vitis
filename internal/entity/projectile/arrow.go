package projectile

import (
	genentity "github.com/vitismc/vitis/internal/data/generated/entity"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/protocol"
)

const (
	ArrowPickupAllowed    int32 = 0
	ArrowPickupCreative   int32 = 1
	ArrowPickupDisallowed int32 = 2

	ArrowBaseDamage = 2.0
)

// Arrow represents an arrow entity in the world.
type Arrow struct {
	*Base
	damage      float64
	piercing    int32
	pickup      int32
	critical    bool
	knockback   int32
	potionColor int32
}

// NewArrow creates an arrow shot from the given position/velocity.
func NewArrow(id int32, uuid protocol.UUID, pos, vel entity.Vec3, ownerID int32, ownerUUID protocol.UUID) *Arrow {
	b := NewBase(id, uuid, entity.EntityType(genentity.EntityArrow), pos, vel, ownerID, ownerUUID, physics.GravityArrow, physics.DragArrow)
	b.Entity.SetProtocolInfo(genentity.EntityArrow, ownerID)
	return &Arrow{
		Base:   b,
		damage: ArrowBaseDamage,
		pickup: ArrowPickupAllowed,
	}
}

// Damage returns the base damage this arrow deals on hit.
func (a *Arrow) Damage() float64 { return a.damage }

// SetDamage sets the arrow's base damage.
func (a *Arrow) SetDamage(d float64) { a.damage = d }

// Piercing returns the number of entities this arrow can pierce through.
func (a *Arrow) Piercing() int32 { return a.piercing }

// SetPiercing sets the piercing level.
func (a *Arrow) SetPiercing(p int32) { a.piercing = p }

// Pickup returns the pickup mode for this arrow.
func (a *Arrow) Pickup() int32 { return a.pickup }

// SetPickup sets the pickup mode.
func (a *Arrow) SetPickup(p int32) { a.pickup = p }

// Critical returns whether this arrow is a critical hit.
func (a *Arrow) Critical() bool { return a.critical }

// SetCritical sets the critical flag.
func (a *Arrow) SetCritical(c bool) { a.critical = c }

// Knockback returns extra knockback level.
func (a *Arrow) Knockback() int32 { return a.knockback }

// SetKnockback sets extra knockback.
func (a *Arrow) SetKnockback(k int32) { a.knockback = k }

// PotionColor returns the potion color for tipped arrows, or 0 for normal arrows.
func (a *Arrow) PotionColor() int32 { return a.potionColor }

// SetPotionColor sets the tipped arrow potion color.
func (a *Arrow) SetPotionColor(c int32) { a.potionColor = c }

// ProtocolType returns the protocol entity type ID.
func (a *Arrow) ProtocolType() int32 {
	return genentity.EntityArrow
}

// SpawnData returns the owner entity ID + 1 for the SpawnEntity packet data field.
func (a *Arrow) SpawnData() int32 {
	if a.OwnerID() > 0 {
		return a.OwnerID() + 1
	}
	return 0
}

// Tick advances the arrow by one tick.
func (a *Arrow) Tick(world physics.BlockAccess, finder EntityFinder) {
	dims := physics.ArrowDimensions()
	a.Base.Tick(world, finder, dims)
}

// ComputeHitDamage returns the damage to deal, accounting for velocity and critical.
func (a *Arrow) ComputeHitDamage() float64 {
	vel := a.Entity.Velocity()
	speed := vel.X*vel.X + vel.Y*vel.Y + vel.Z*vel.Z
	if speed < 0.01 {
		speed = 0.01
	}
	dmg := a.damage * speed
	if a.critical {
		dmg += dmg / 2
	}
	return dmg
}
