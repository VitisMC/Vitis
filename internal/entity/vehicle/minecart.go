package vehicle

import (
	"math"

	genentity "github.com/vitismc/vitis/internal/data/generated/entity"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/protocol"
)

// MinecartType represents the variant of minecart.
type MinecartType int32

const (
	MinecartRideable     MinecartType = 0
	MinecartChest        MinecartType = 1
	MinecartFurnace      MinecartType = 2
	MinecartTNT          MinecartType = 3
	MinecartSpawner      MinecartType = 4
	MinecartHopper       MinecartType = 5
	MinecartCommandBlock MinecartType = 6
)

var minecartEntityIDs = map[MinecartType]int32{
	MinecartRideable:     genentity.EntityMinecart,
	MinecartChest:        genentity.EntityChestMinecart,
	MinecartFurnace:      genentity.EntityFurnaceMinecart,
	MinecartTNT:          genentity.EntityTntMinecart,
	MinecartSpawner:      genentity.EntitySpawnerMinecart,
	MinecartHopper:       genentity.EntityHopperMinecart,
	MinecartCommandBlock: genentity.EntityCommandBlockMinecart,
}

// Minecart represents a minecart entity.
type Minecart struct {
	*BaseVehicle
	minecartType MinecartType
	onRail       bool
	speed        float64
	maxSpeed     float64
}

// NewMinecart creates a minecart at the given position.
func NewMinecart(id int32, uuid protocol.UUID, pos entity.Vec3, mcType MinecartType) *Minecart {
	entityTypeID := minecartEntityIDs[mcType]
	maxSeats := 0
	if mcType == MinecartRideable {
		maxSeats = 1
	}
	e := entity.NewEntity(id, uuid, entity.EntityType(entityTypeID), pos, entity.Vec2{})
	e.SetProtocolInfo(entityTypeID, 0)
	return &Minecart{
		BaseVehicle:  NewBaseVehicle(e, maxSeats),
		minecartType: mcType,
		maxSpeed:     0.4,
	}
}

// MinecartType returns the minecart variant.
func (m *Minecart) MinecartType() MinecartType { return m.minecartType }

// OnRail returns whether the minecart is currently on a rail.
func (m *Minecart) OnRail() bool { return m.onRail }

// Speed returns the current speed.
func (m *Minecart) Speed() float64 { return m.speed }

// ProtocolType returns the protocol entity type ID.
func (m *Minecart) ProtocolType() int32 {
	return minecartEntityIDs[m.minecartType]
}

// SpawnData returns the variant data for the SpawnEntity packet.
func (m *Minecart) SpawnData() int32 {
	return 0
}

// Tick advances the minecart by one tick.
func (m *Minecart) Tick(world physics.BlockAccess) {
	if m == nil || m.entity.Removed() {
		return
	}

	vel := m.entity.Velocity()
	pos := m.entity.Position()

	vel.Y -= 0.04

	dims := physics.EntityDimensions{Width: 0.98, Height: 0.7}
	box := dims.MakeBoundingBox(pos.X, pos.Y, pos.Z)
	result := physics.MoveWithCollision(world, box, vel.X, vel.Y, vel.Z)

	pos.X += result.Dx
	pos.Y += result.Dy
	pos.Z += result.Dz
	m.entity.SetPosition(pos)

	m.entity.SetOnGround(result.OnGround)

	if result.CollidedX {
		vel.X = 0
	}
	if result.CollidedY {
		vel.Y = 0
	}
	if result.CollidedZ {
		vel.Z = 0
	}

	vel.X *= 0.96
	vel.Y *= 0.95
	vel.Z *= 0.96

	if m.entity.OnGround() {
		vel.X *= 0.5
		vel.Z *= 0.5
	}

	if math.Abs(vel.X) < 0.003 {
		vel.X = 0
	}
	if math.Abs(vel.Z) < 0.003 {
		vel.Z = 0
	}

	m.speed = math.Sqrt(vel.X*vel.X + vel.Z*vel.Z)
	m.entity.SetVelocity(vel)

	if pos.Y < -128 {
		m.entity.Remove()
	}
}
