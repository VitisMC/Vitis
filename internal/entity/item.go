package entity

import (
	"math"
	"math/rand"

	genentity "github.com/vitismc/vitis/internal/data/generated/entity"
	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
)

const (
	EntityTypeItem EntityType = 20

	itemDespawnTicks  = 6000
	itemMergeRadius   = 0.5
	itemPickupDelay   = 10
	itemMaxStackSize  = 64
	itemDefaultHealth = 5
)

// ItemEntity represents a dropped item in the world.
type ItemEntity struct {
	*Entity
	stack       inventory.Slot
	age         int32
	pickupDelay int32
	health      float32
	thrower     protocol.UUID
	noDespawn   bool
}

// NewItemEntity creates an item entity with random initial velocity.
func NewItemEntity(id int32, uuid protocol.UUID, pos Vec3, stack inventory.Slot) *ItemEntity {
	e := NewEntity(id, uuid, EntityTypeItem, pos, Vec2{
		X: float32(rand.Float64() * 360.0),
	})
	e.SetProtocolInfo(genentity.EntityItem, 0)
	e.vel = Vec3{
		X: (rand.Float64()*0.2 - 0.1),
		Y: 0.2,
		Z: (rand.Float64()*0.2 - 0.1),
	}
	e.dirty |= DirtyVelocity
	return &ItemEntity{
		Entity:      e,
		stack:       stack,
		pickupDelay: itemPickupDelay,
		health:      itemDefaultHealth,
	}
}

// NewItemEntityWithVelocity creates an item entity with a specified velocity.
func NewItemEntityWithVelocity(id int32, uuid protocol.UUID, pos Vec3, stack inventory.Slot, vel Vec3, pickupDelay int32) *ItemEntity {
	e := NewEntity(id, uuid, EntityTypeItem, pos, Vec2{
		X: float32(rand.Float64() * 360.0),
	})
	e.SetProtocolInfo(genentity.EntityItem, 0)
	e.vel = vel
	e.dirty |= DirtyVelocity
	return &ItemEntity{
		Entity:      e,
		stack:       stack,
		pickupDelay: pickupDelay,
		health:      itemDefaultHealth,
	}
}

// Stack returns the item stack this entity represents.
func (i *ItemEntity) Stack() inventory.Slot { return i.stack }

// SetStack replaces the item stack.
func (i *ItemEntity) SetStack(s inventory.Slot) { i.stack = s }

// Age returns the entity's age in ticks.
func (i *ItemEntity) Age() int32 { return i.age }

// PickupDelay returns remaining ticks before this item can be picked up.
func (i *ItemEntity) PickupDelay() int32 { return i.pickupDelay }

// SetPickupDelay sets the pickup delay in ticks.
func (i *ItemEntity) SetPickupDelay(d int32) { i.pickupDelay = d }

// Thrower returns the UUID of the entity that dropped this item.
func (i *ItemEntity) Thrower() protocol.UUID { return i.thrower }

// SetThrower sets the dropper's UUID.
func (i *ItemEntity) SetThrower(uuid protocol.UUID) { i.thrower = uuid }

// SetNoDespawn prevents the item from despawning after the normal timer.
func (i *ItemEntity) SetNoDespawn(v bool) { i.noDespawn = v }

// ProtocolType returns the protocol entity type ID for items.
func (i *ItemEntity) ProtocolType() int32 {
	return genentity.EntityItem
}

// SpawnData returns the data field for the SpawnEntity packet (unused for items).
func (i *ItemEntity) SpawnData() int32 {
	return 0
}

// CanPickup returns whether a player can pick up this item.
func (i *ItemEntity) CanPickup() bool {
	return i.pickupDelay <= 0 && !i.removed && !i.stack.Empty()
}

// TryMerge attempts to merge another item entity's stack into this one.
// Returns true if the merge was successful and other should be removed.
func (i *ItemEntity) TryMerge(other *ItemEntity) bool {
	if i == nil || other == nil || i.removed || other.removed {
		return false
	}
	if i.stack.ItemID != other.stack.ItemID {
		return false
	}
	total := i.stack.ItemCount + other.stack.ItemCount
	if total > itemMaxStackSize {
		return false
	}
	i.stack.ItemCount = total
	i.age = min32(i.age, other.age)
	return true
}

// Tick advances the item entity by one tick.
func (i *ItemEntity) Tick(world physics.BlockAccess) {
	if i == nil || i.removed {
		return
	}

	i.age++
	if i.pickupDelay > 0 {
		i.pickupDelay--
	}

	if !i.noDespawn && i.age >= itemDespawnTicks {
		i.removed = true
		return
	}

	i.vel.Y -= physics.GravityItem
	if i.vel.Y < physics.TerminalVelocityItem {
		i.vel.Y = physics.TerminalVelocityItem
	}

	dims := physics.ItemDimensions()
	box := dims.MakeBoundingBox(i.pos.X, i.pos.Y, i.pos.Z)
	result := physics.MoveWithCollision(world, box, i.vel.X, i.vel.Y, i.vel.Z)

	i.pos.X += result.Dx
	i.pos.Y += result.Dy
	i.pos.Z += result.Dz
	i.dirty |= DirtyPosition

	i.onGround = result.OnGround

	if result.CollidedX {
		i.vel.X = 0
	}
	if result.CollidedY {
		i.vel.Y = 0
	}
	if result.CollidedZ {
		i.vel.Z = 0
	}

	if i.onGround {
		i.vel.X *= 0.7
		i.vel.Z *= 0.7
		i.vel.Y *= -0.5
		if math.Abs(i.vel.Y) < 0.01 {
			i.vel.Y = 0
		}
	} else {
		i.vel.X *= physics.DragItem
		i.vel.Z *= physics.DragItem
	}

	if i.pos.Y < -128 {
		i.removed = true
	}
}

// IsInRange returns whether pos is within mergeRadius of this item.
func (i *ItemEntity) IsInRange(pos Vec3, radius float64) bool {
	dx := i.pos.X - pos.X
	dy := i.pos.Y - pos.Y
	dz := i.pos.Z - pos.Z
	return dx*dx+dy*dy+dz*dz <= radius*radius
}

// ItemEntityManager manages all item entities in a world.
type ItemEntityManager struct {
	items map[int32]*ItemEntity
}

// NewItemEntityManager creates a new item entity manager.
func NewItemEntityManager() *ItemEntityManager {
	return &ItemEntityManager{
		items: make(map[int32]*ItemEntity, 64),
	}
}

// Add registers an item entity.
func (m *ItemEntityManager) Add(item *ItemEntity) {
	if m == nil || item == nil {
		return
	}
	m.items[item.ID()] = item
}

// Remove removes an item entity by ID.
func (m *ItemEntityManager) Remove(id int32) {
	if m == nil {
		return
	}
	delete(m.items, id)
}

// Get returns an item entity by ID.
func (m *ItemEntityManager) Get(id int32) *ItemEntity {
	if m == nil {
		return nil
	}
	return m.items[id]
}

// All returns all item entities.
func (m *ItemEntityManager) All() map[int32]*ItemEntity {
	if m == nil {
		return nil
	}
	return m.items
}

// Count returns the number of item entities.
func (m *ItemEntityManager) Count() int {
	if m == nil {
		return 0
	}
	return len(m.items)
}

// TickAll advances all item entities and handles merging and removal.
func (m *ItemEntityManager) TickAll(world physics.BlockAccess) []int32 {
	if m == nil {
		return nil
	}

	var removed []int32

	for _, item := range m.items {
		item.Tick(world)
	}

	for id, item := range m.items {
		if item.removed {
			removed = append(removed, id)
			continue
		}

		if item.age%25 == 0 && !item.stack.Empty() {
			for othID, other := range m.items {
				if othID == id || other.removed || other.stack.Empty() {
					continue
				}
				if !item.IsInRange(other.pos, itemMergeRadius) {
					continue
				}
				if item.TryMerge(other) {
					other.removed = true
					removed = append(removed, othID)
				}
			}
		}
	}

	for _, id := range removed {
		delete(m.items, id)
	}

	return removed
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}
