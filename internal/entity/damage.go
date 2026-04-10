package entity

import (
	"math"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/equipment"
)

// DamageType classifies the source of damage.
type DamageType int32

const (
	DamageTypeGeneric        DamageType = 0
	DamageTypePlayerAttack   DamageType = 1
	DamageTypeMobAttack      DamageType = 2
	DamageTypeProjectile     DamageType = 3
	DamageTypeFall           DamageType = 4
	DamageTypeFire           DamageType = 5
	DamageTypeLava           DamageType = 6
	DamageTypeDrowning       DamageType = 7
	DamageTypeSuffocation    DamageType = 8
	DamageTypeVoid           DamageType = 9
	DamageTypeStarvation     DamageType = 10
	DamageTypeExplosion      DamageType = 11
	DamageTypeMagic          DamageType = 12
	DamageTypeFireTick       DamageType = 13
	DamageTypeFreezing       DamageType = 14
	DamageTypeLightning      DamageType = 15
	DamageTypeSweep          DamageType = 16
	DamageTypeThorns         DamageType = 17
	DamageTypeCactus         DamageType = 18
	DamageTypeFlyIntoWall    DamageType = 19
	DamageTypeWitherEffect   DamageType = 20
	DamageTypeSweetBerryBush DamageType = 21
)

// BypassesArmor returns true if this damage type ignores armor protection.
func (t DamageType) BypassesArmor() bool {
	switch t {
	case DamageTypeFall, DamageTypeDrowning, DamageTypeSuffocation,
		DamageTypeVoid, DamageTypeStarvation, DamageTypeMagic,
		DamageTypeFireTick, DamageTypeFreezing, DamageTypeWitherEffect:
		return true
	}
	return false
}

// BypassesInvulnerability returns true if this damage ignores the hurt cooldown.
func (t DamageType) BypassesInvulnerability() bool {
	return t == DamageTypeVoid || t == DamageTypeStarvation
}

// IsFire returns true if this damage counts as fire damage.
func (t DamageType) IsFire() bool {
	return t == DamageTypeFire || t == DamageTypeLava || t == DamageTypeFireTick
}

// IsProjectile returns true if this damage comes from a projectile.
func (t DamageType) IsProjectile() bool {
	return t == DamageTypeProjectile
}

// IsExplosion returns true if this damage comes from an explosion.
func (t DamageType) IsExplosion() bool {
	return t == DamageTypeExplosion
}

// DeathMessage returns a format string for the death message.
func (t DamageType) DeathMessage() string {
	switch t {
	case DamageTypePlayerAttack:
		return "%s was slain by %s"
	case DamageTypeMobAttack:
		return "%s was slain by %s"
	case DamageTypeProjectile:
		return "%s was shot by %s"
	case DamageTypeFall:
		return "%s hit the ground too hard"
	case DamageTypeFire, DamageTypeFireTick:
		return "%s burned to death"
	case DamageTypeLava:
		return "%s tried to swim in lava"
	case DamageTypeDrowning:
		return "%s drowned"
	case DamageTypeSuffocation:
		return "%s suffocated in a wall"
	case DamageTypeVoid:
		return "%s fell out of the world"
	case DamageTypeStarvation:
		return "%s starved to death"
	case DamageTypeExplosion:
		return "%s was blown up"
	case DamageTypeMagic:
		return "%s was killed by magic"
	case DamageTypeFreezing:
		return "%s froze to death"
	case DamageTypeLightning:
		return "%s was struck by lightning"
	case DamageTypeCactus:
		return "%s was pricked to death"
	case DamageTypeFlyIntoWall:
		return "%s experienced kinetic energy"
	case DamageTypeSweetBerryBush:
		return "%s was poked to death by a sweet berry bush"
	default:
		return "%s died"
	}
}

// SourceName returns a short identifier for the damage type.
func (t DamageType) SourceName() string {
	switch t {
	case DamageTypePlayerAttack:
		return "player"
	case DamageTypeMobAttack:
		return "mob"
	case DamageTypeProjectile:
		return "projectile"
	case DamageTypeFall:
		return "fall"
	case DamageTypeFire:
		return "fire"
	case DamageTypeLava:
		return "lava"
	case DamageTypeDrowning:
		return "drowning"
	case DamageTypeSuffocation:
		return "suffocation"
	case DamageTypeVoid:
		return "void"
	case DamageTypeStarvation:
		return "starvation"
	case DamageTypeExplosion:
		return "explosion"
	case DamageTypeMagic:
		return "magic"
	case DamageTypeFireTick:
		return "fire_tick"
	case DamageTypeFreezing:
		return "freezing"
	case DamageTypeLightning:
		return "lightning"
	case DamageTypeSweep:
		return "player_sweep"
	case DamageTypeCactus:
		return "cactus"
	default:
		return "generic"
	}
}

// DamageSource describes a single damage event.
type DamageSource struct {
	Type       DamageType
	Amount     float32
	AttackerID int32
	DirectID   int32
}

// NewDamageSource creates a simple damage source.
func NewDamageSource(dtype DamageType, amount float32) *DamageSource {
	return &DamageSource{Type: dtype, Amount: amount, AttackerID: -1, DirectID: -1}
}

// NewAttackDamageSource creates a damage source from an entity attack.
func NewAttackDamageSource(dtype DamageType, amount float32, attackerID int32) *DamageSource {
	return &DamageSource{Type: dtype, Amount: amount, AttackerID: attackerID, DirectID: attackerID}
}

// ApplyDamage applies a structured damage event with armor and enchantment reduction.
// armorDefense and armorToughness come from the target's equipped armor.
// enchProtection is the combined enchantment protection factor (0-20).
// Returns the actual damage dealt after all reductions.
func (l *LivingEntity) ApplyDamage(src *DamageSource, armorDefense, armorToughness, enchProtection float64) float32 {
	if l.dead || src == nil || src.Amount <= 0 {
		return 0
	}
	if !src.Type.BypassesInvulnerability() && l.hurtCooldown > 0 {
		return 0
	}

	amount := src.Amount

	if !src.Type.BypassesArmor() && armorDefense > 0 {
		amount = float32(equipment.CalculateDamageReduction(float64(amount), armorDefense, armorToughness))
	}

	if enchProtection > 20 {
		enchProtection = 20
	}
	if enchProtection > 0 && !src.Type.BypassesArmor() {
		amount *= float32(1.0 - enchProtection/25.0)
	}

	if amount <= 0 {
		return 0
	}

	actual := amount
	if l.absorption > 0 {
		absorbed := l.absorption
		if absorbed > actual {
			absorbed = actual
		}
		l.absorption -= absorbed
		actual -= absorbed
	}

	l.health -= actual
	if l.health <= 0 {
		l.health = 0
		l.dead = true
		l.deathTime = 1
	}

	l.hurtTime = 10
	l.hurtCooldown = 10
	l.lastDamageSource = src.Type.SourceName()
	l.lastDamageAmount = src.Amount
	l.lastDamageType = src.Type
	l.lastAttackerID = src.AttackerID

	return actual
}

// LastDamageType returns the type of the last damage taken.
func (l *LivingEntity) LastDamageType() DamageType { return l.lastDamageType }

// LastAttackerID returns the entity ID of the last attacker (-1 if none).
func (l *LivingEntity) LastAttackerID() int32 { return l.lastAttackerID }

// AirSupply returns remaining air ticks (max 300 = 15 seconds).
func (l *LivingEntity) AirSupply() int32 { return l.airSupply }

// SetAirSupply sets the air supply value.
func (l *LivingEntity) SetAirSupply(v int32) {
	if v < 0 {
		v = 0
	}
	if v > 300 {
		v = 300
	}
	l.airSupply = v
}

// FireTicks returns the remaining fire ticks. > 0 means on fire.
func (l *LivingEntity) FireTicks() int32 { return l.fireTicks }

// SetFireTicks sets the fire tick counter.
func (l *LivingEntity) SetFireTicks(v int32) {
	if v < 0 {
		v = 0
	}
	l.fireTicks = v
}

// TickEnvironment checks for environmental damage each tick.
// blockAt returns the block state ID at absolute world coordinates.
// Returns a list of damage sources to apply (caller handles packets).
func (l *LivingEntity) TickEnvironment(blockAt func(x, y, z int) int32) []*DamageSource {
	if l.dead || l.gameMode == 1 || l.gameMode == 3 {
		l.fireTicks = 0
		l.airSupply = 300
		return nil
	}

	var damages []*DamageSource
	pos := l.Entity.Position()
	feetX := int(math.Floor(pos.X))
	feetY := int(math.Floor(pos.Y))
	feetZ := int(math.Floor(pos.Z))
	headY := int(math.Floor(pos.Y + 1.62))

	feetBlock := blockAt(feetX, feetY, feetZ)
	headBlock := blockAt(feetX, headY, feetZ)

	if pos.Y < -64 {
		damages = append(damages, NewDamageSource(DamageTypeVoid, 4.0))
	}

	if isFireBlock(feetBlock) && l.fireTicks <= 0 {
		l.fireTicks = 160
		damages = append(damages, NewDamageSource(DamageTypeFire, 1.0))
	}

	if isLavaBlock(feetBlock) {
		l.fireTicks = 300
		damages = append(damages, NewDamageSource(DamageTypeLava, 4.0))
	}

	if l.fireTicks > 0 {
		l.fireTicks--
		if l.fireTicks > 0 && l.fireTicks%20 == 0 {
			damages = append(damages, NewDamageSource(DamageTypeFireTick, 1.0))
		}
		if isWaterBlock(feetBlock) || isWaterBlock(headBlock) {
			l.fireTicks = 0
		}
	}

	inWater := isWaterBlock(headBlock)
	if inWater {
		if l.airSupply > 0 {
			l.airSupply--
		} else {
			damages = append(damages, NewDamageSource(DamageTypeDrowning, 2.0))
		}
	} else {
		if l.airSupply < 300 {
			l.airSupply += 4
			if l.airSupply > 300 {
				l.airSupply = 300
			}
		}
	}

	if isSolidBlock(headBlock) && headBlock > 0 {
		damages = append(damages, NewDamageSource(DamageTypeSuffocation, 1.0))
	}

	if isCactusBlock(feetBlock) {
		damages = append(damages, NewDamageSource(DamageTypeCactus, 1.0))
	}

	if isSweetBerryBush(feetBlock) {
		damages = append(damages, NewDamageSource(DamageTypeSweetBerryBush, 1.0))
	}

	return damages
}

// Block type detection helpers delegating to the block package.

func isFireBlock(stateID int32) bool      { return block.IsFire(stateID) }
func isLavaBlock(stateID int32) bool      { return block.IsLava(stateID) }
func isWaterBlock(stateID int32) bool     { return block.IsWater(stateID) }
func isSolidBlock(stateID int32) bool     { return block.IsSolid(stateID) }
func isCactusBlock(stateID int32) bool    { return block.IsCactus(stateID) }
func isSweetBerryBush(stateID int32) bool { return block.IsSweetBerryBush(stateID) }
