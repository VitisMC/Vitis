package vehicle

import (
	"math"

	genentity "github.com/vitismc/vitis/internal/data/generated/entity"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/protocol"
)

// BoatWoodType represents the wood type of a boat.
type BoatWoodType int32

const (
	BoatOak      BoatWoodType = 0
	BoatSpruce   BoatWoodType = 1
	BoatBirch    BoatWoodType = 2
	BoatJungle   BoatWoodType = 3
	BoatAcacia   BoatWoodType = 4
	BoatDarkOak  BoatWoodType = 5
	BoatMangrove BoatWoodType = 6
	BoatBamboo   BoatWoodType = 7
	BoatCherry   BoatWoodType = 8
	BoatPaleOak  BoatWoodType = 9
)

var boatEntityIDs = map[BoatWoodType]int32{
	BoatOak:      genentity.EntityOakBoat,
	BoatSpruce:   genentity.EntitySpruceBoat,
	BoatBirch:    genentity.EntityBirchBoat,
	BoatJungle:   genentity.EntityJungleBoat,
	BoatAcacia:   genentity.EntityAcaciaBoat,
	BoatDarkOak:  genentity.EntityDarkOakBoat,
	BoatMangrove: genentity.EntityMangroveBoat,
	BoatBamboo:   genentity.EntityBambooRaft,
	BoatCherry:   genentity.EntityCherryBoat,
	BoatPaleOak:  genentity.EntityPaleOakBoat,
}

// Boat represents a boat entity.
type Boat struct {
	*BaseVehicle
	woodType    BoatWoodType
	hasChest    bool
	paddleLeft  bool
	paddleRight bool
	waterLevel  float64
	inWater     bool
}

// NewBoat creates a boat entity at the given position.
func NewBoat(id int32, uuid protocol.UUID, pos entity.Vec3, woodType BoatWoodType) *Boat {
	entityTypeID := boatEntityIDs[woodType]
	e := entity.NewEntity(id, uuid, entity.EntityType(entityTypeID), pos, entity.Vec2{})
	e.SetProtocolInfo(entityTypeID, 0)
	return &Boat{
		BaseVehicle: NewBaseVehicle(e, 2),
		woodType:    woodType,
	}
}

// WoodType returns the boat's wood type.
func (b *Boat) WoodType() BoatWoodType { return b.woodType }

// HasChest returns whether this is a chest boat.
func (b *Boat) HasChest() bool { return b.hasChest }

// SetHasChest sets the chest boat flag.
func (b *Boat) SetHasChest(v bool) { b.hasChest = v }

// InWater returns whether the boat is in water.
func (b *Boat) InWater() bool { return b.inWater }

// ProtocolType returns the protocol entity type ID.
func (b *Boat) ProtocolType() int32 {
	return boatEntityIDs[b.woodType]
}

// SpawnData returns the variant data for the SpawnEntity packet.
func (b *Boat) SpawnData() int32 {
	return 0
}

// Tick advances the boat by one tick.
func (b *Boat) Tick(world physics.BlockAccess) {
	if b == nil || b.entity.Removed() {
		return
	}

	vel := b.entity.Velocity()
	pos := b.entity.Position()

	vel.Y -= physics.GravityBoat

	dims := physics.EntityDimensions{Width: 1.375, Height: 0.5625}
	box := dims.MakeBoundingBox(pos.X, pos.Y, pos.Z)
	result := physics.MoveWithCollision(world, box, vel.X, vel.Y, vel.Z)

	pos.X += result.Dx
	pos.Y += result.Dy
	pos.Z += result.Dz
	b.entity.SetPosition(pos)

	b.entity.SetOnGround(result.OnGround)

	if result.CollidedX {
		vel.X = 0
	}
	if result.CollidedY {
		vel.Y = 0
	}
	if result.CollidedZ {
		vel.Z = 0
	}

	vel.X *= 0.9
	vel.Z *= 0.9
	vel.Y *= 0.95

	if math.Abs(vel.X) < 0.003 {
		vel.X = 0
	}
	if math.Abs(vel.Z) < 0.003 {
		vel.Z = 0
	}

	b.entity.SetVelocity(vel)

	if pos.Y < -128 {
		b.entity.Remove()
	}
}

// SetPaddleState sets the paddle animation state.
func (b *Boat) SetPaddleState(left, right bool) {
	b.paddleLeft = left
	b.paddleRight = right
}

// PaddleState returns the paddle animation state.
func (b *Boat) PaddleState() (left, right bool) {
	return b.paddleLeft, b.paddleRight
}
