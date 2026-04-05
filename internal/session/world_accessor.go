package session

import (
	"math"
	"math/rand"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/block/behavior"
	"github.com/vitismc/vitis/internal/block/fluid"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/tick"
)

// TickScheduler interface for scheduling block/fluid ticks.
type TickScheduler interface {
	Schedule(x, y, z int, currentTick uint64, delay int, priority tick.Priority, tickType tick.TickType, subID int32)
	IsScheduled(x, y, z int, tickType tick.TickType, subID int32) bool
}

// WorldAccessor provides block-level world access for play state handlers.
type WorldAccessor interface {
	GetChunk(x, z int32) (*chunk.Chunk, bool)
	SetBlock(x, y, z int, stateID int32)
	GetBlock(x, y, z int) int32
	ScheduleTicksForBlock(x, y, z int, stateID int32)
	NotifyNeighbors(x, y, z int)
	SpawnItemDrop(x, y, z float64, itemID, count int32) *entity.ItemEntity
}

// DefaultWorldAccessor implements WorldAccessor using a chunk.Manager.
type DefaultWorldAccessor struct {
	Chunks        *chunk.Manager
	TickScheduler TickScheduler
	CurrentTick   func() uint64
	NextEntityID  func() int32
	Items         *entity.ItemEntityManager
}

// GetChunk returns a loaded chunk from the manager.
func (a *DefaultWorldAccessor) GetChunk(x, z int32) (*chunk.Chunk, bool) {
	if a == nil || a.Chunks == nil {
		return nil, false
	}
	return a.Chunks.GetChunk(x, z)
}

// SetBlock sets a block in the chunk at the given absolute coordinates.
func (a *DefaultWorldAccessor) SetBlock(x, y, z int, stateID int32) {
	if a == nil || a.Chunks == nil {
		return
	}
	cx := int32(math.Floor(float64(x) / 16))
	cz := int32(math.Floor(float64(z) / 16))
	c, ok := a.Chunks.GetChunk(cx, cz)
	if !ok || c == nil {
		return
	}
	c.SetBlock(x, y, z, stateID)
}

// GetBlock returns the block state ID at the given absolute coordinates.
func (a *DefaultWorldAccessor) GetBlock(x, y, z int) int32 {
	if a == nil || a.Chunks == nil {
		return 0
	}
	cx := int32(math.Floor(float64(x) / 16))
	cz := int32(math.Floor(float64(z) / 16))
	c, ok := a.Chunks.GetChunk(cx, cz)
	if !ok || c == nil {
		return 0
	}
	return c.GetBlock(x, y, z)
}

// ScheduleTicksForBlock schedules block/fluid ticks for a newly placed block.
func (a *DefaultWorldAccessor) ScheduleTicksForBlock(x, y, z int, stateID int32) {
	if a == nil || a.TickScheduler == nil || a.CurrentTick == nil {
		return
	}

	currentTick := a.CurrentTick()

	b := behavior.GetByState(stateID)
	if b.ScheduleOnPlace() {
		delay := b.TickDelay()
		if delay > 0 {
			a.TickScheduler.Schedule(x, y, z, currentTick, delay, tick.PriorityNormal, tick.TickTypeBlock, stateID)
		}
	}

	if fluid.IsFluid(stateID) {
		config := fluid.GetFluidConfig(stateID)
		if config != nil {
			fluidID := fluid.GetFluidID(config.FluidType)
			if !a.TickScheduler.IsScheduled(x, y, z, tick.TickTypeFluid, fluidID) {
				a.TickScheduler.Schedule(x, y, z, currentTick, config.FlowSpeed, tick.PriorityNormal, tick.TickTypeFluid, fluidID)
			}
		}
	}
}

// NotifyNeighbors calls OnNeighborUpdate on the 6 blocks adjacent to (x, y, z).
func (a *DefaultWorldAccessor) NotifyNeighbors(x, y, z int) {
	if a == nil || a.Chunks == nil {
		return
	}
	offsets := [6][3]int{{-1, 0, 0}, {1, 0, 0}, {0, -1, 0}, {0, 1, 0}, {0, 0, -1}, {0, 0, 1}}
	for _, off := range offsets {
		nx, ny, nz := x+off[0], y+off[1], z+off[2]
		state := a.GetBlock(nx, ny, nz)
		if state <= 0 {
			continue
		}

		if a.TickScheduler != nil && a.CurrentTick != nil {
			currentTick := a.CurrentTick()

			b := behavior.GetByState(state)
			if b.ScheduleOnNeighborUpdate() {
				delay := b.TickDelay()
				if delay > 0 && !a.TickScheduler.IsScheduled(nx, ny, nz, tick.TickTypeBlock, state) {
					a.TickScheduler.Schedule(nx, ny, nz, currentTick, delay, tick.PriorityNormal, tick.TickTypeBlock, state)
				}
			}

			if fluid.IsFluid(state) {
				config := fluid.GetFluidConfig(state)
				if config != nil {
					fluidID := fluid.GetFluidID(config.FluidType)
					if !a.TickScheduler.IsScheduled(nx, ny, nz, tick.TickTypeFluid, fluidID) {
						a.TickScheduler.Schedule(nx, ny, nz, currentTick, config.FlowSpeed, tick.PriorityNormal, tick.TickTypeFluid, fluidID)
					}
				}
			}
		}

		ctx := &behavior.Context{X: nx, Y: ny, Z: nz, StateID: state}
		behavior.GetByState(state).OnNeighborUpdate(ctx)
	}
}

// BlockFaceOffset returns the position offset for a given block face direction.
// Face values: 0=bottom(-Y), 1=top(+Y), 2=north(-Z), 3=south(+Z), 4=west(-X), 5=east(+X).
func BlockFaceOffset(face int32) (dx, dy, dz int32) {
	switch face {
	case 0:
		return 0, -1, 0
	case 1:
		return 0, 1, 0
	case 2:
		return 0, 0, -1
	case 3:
		return 0, 0, 1
	case 4:
		return -1, 0, 0
	case 5:
		return 1, 0, 0
	default:
		return 0, 0, 0
	}
}

// SpawnItemDrop creates an item entity at the given position and registers it.
func (a *DefaultWorldAccessor) SpawnItemDrop(x, y, z float64, itemID, count int32) *entity.ItemEntity {
	if a == nil || a.NextEntityID == nil || a.Items == nil {
		return nil
	}
	if itemID <= 0 || count <= 0 {
		return nil
	}
	eid := a.NextEntityID()
	uuid := protocol.UUID{uint64(rand.Int63()), uint64(rand.Int63())}
	stack := inventory.NewSlot(itemID, count)
	pos := entity.Vec3{X: x + 0.5, Y: y + 0.5, Z: z + 0.5}
	ie := entity.NewItemEntity(eid, uuid, pos, stack)
	a.Items.Add(ie)
	return ie
}

// resolveBlockStateFromItem maps an item ID to the default block state ID.
// Returns 0 (air) if the item doesn't correspond to a placeable block.
func resolveBlockStateFromItem(itemID int32) int32 {
	itemName := item.NameByID(itemID)
	if itemName == "" {
		return 0
	}
	stateID := block.DefaultStateID(itemName)
	if stateID < 0 {
		return 0
	}
	return stateID
}
