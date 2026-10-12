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
	"github.com/vitismc/vitis/internal/world/blockentity"
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
	Submit(command WorldCommand) bool
	GetChunk(x, z int32) (*chunk.Chunk, bool)
	SetBlock(x, y, z int, stateID int32)
	GetBlock(x, y, z int) int32
	ScheduleTicksForBlock(x, y, z int, stateID int32)
	NotifyNeighbors(x, y, z int)
	SpawnItemDrop(x, y, z float64, itemID, count int32) *entity.ItemEntity
	IgniteTNT(x, y, z int, ownerID int32) bool
	ResendPlayerChunks(playerID int32, chunkX, chunkZ int32)
	BlockEntities() *blockentity.Store
}

// WorldCommand is one ordered mutation executed by the authoritative world tick.
type WorldCommand interface {
	Execute()
}

// DefaultWorldAccessor implements WorldAccessor using a chunk.Manager.
type DefaultWorldAccessor struct {
	DimensionName          string
	Chunks                 *chunk.Manager
	TickScheduler          TickScheduler
	CurrentTick            func() uint64
	NextEntityID           func() int32
	Items                  *entity.ItemEntityManager
	RegisterItem           func(*entity.ItemEntity)
	BlockEntityStore       *blockentity.Store
	SubmitTask             func(func()) bool
	ReadTimeOfDay          func() int64
	WriteTimeOfDay         func(int64)
	ResetPlayerChunks      func(playerID int32, chunkX, chunkZ int32)
	PrimeTNT               func(x, y, z int, ownerID int32) bool
	ReadSleepThreat        func(int, int, int) bool
	ReadThunder            func() bool
	ClearWeatherAfterSleep func()
	CreateBedExplosion     func(int, int, int) bool
	UpdateNeighbors        func(int, int, int)
	AttackMobEntity        func(*entity.Player, int32, *entity.DamageSource, float64) bool
}

// AttackMob applies a validated player attack to a resident mob on the world tick.
func (a *DefaultWorldAccessor) AttackMob(player *entity.Player, target int32, source *entity.DamageSource, reach float64) bool {
	return a != nil && a.AttackMobEntity != nil && a.AttackMobEntity(player, target, source, reach)
}

// ExplodeBed applies a forbidden-dimension bed explosion on the authoritative tick.
func (a *DefaultWorldAccessor) ExplodeBed(x, y, z int) bool {
	return a != nil && a.CreateBedExplosion != nil && a.CreateBedExplosion(x, y, z)
}

// IgniteTNT primes a loaded TNT block on the authoritative world tick.
func (a *DefaultWorldAccessor) IgniteTNT(x, y, z int, ownerID int32) bool {
	return a != nil && a.PrimeTNT != nil && a.PrimeTNT(x, y, z, ownerID)
}

// ResendPlayerChunks restarts chunk delivery after the client resets its level on respawn.
func (a *DefaultWorldAccessor) ResendPlayerChunks(playerID int32, chunkX, chunkZ int32) {
	if a != nil && a.ResetPlayerChunks != nil {
		a.ResetPlayerChunks(playerID, chunkX, chunkZ)
	}
}

// TimeOfDay returns the current world daytime for sleep validation.
func (a *DefaultWorldAccessor) TimeOfDay() int64 {
	if a != nil && a.ReadTimeOfDay != nil {
		return a.ReadTimeOfDay()
	}
	return 0
}

// SetTimeOfDay advances daytime after all active players sleep.
func (a *DefaultWorldAccessor) SetTimeOfDay(value int64) {
	if a != nil && a.WriteTimeOfDay != nil {
		a.WriteTimeOfDay(value)
	}
}

// Submit queues one command for ordered execution by the authoritative world tick.
func (a *DefaultWorldAccessor) Submit(command WorldCommand) bool {
	if a == nil || command == nil {
		return false
	}
	if a.SubmitTask == nil {
		return false
	}
	if owned, ok := command.(interface{ OwnerSession() Session }); ok {
		s := owned.OwnerSession()
		if s == nil || s.Context().Err() != nil || (a.DimensionName != "" && SessionDimension(s) != a.DimensionName) {
			return false
		}
		var binding *worldBinding
		if concrete, ok := s.(*DefaultSession); ok {
			binding = concrete.world.Load()
		}
		_, preserveOrder := command.(interface{ preserveDisconnectOrder() })
		return a.SubmitTask(func() {
			if s.Context().Err() != nil && !preserveOrder || (a.DimensionName != "" && SessionDimension(s) != a.DimensionName) {
				return
			}
			if concrete, ok := s.(*DefaultSession); ok && concrete.world.Load() != binding {
				return
			}
			command.Execute()
		})
	}
	return a.SubmitTask(command.Execute)
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
	if y < c.Data().MinY() || y > c.Data().MaxY() || c.GetBlock(x, y, z) == stateID {
		return
	}
	c.SetBlock(x, y, z, stateID)
	if a.UpdateNeighbors != nil {
		a.UpdateNeighbors(x, y, z)
	}
}

// GetBlock returns a resident state or -1 when chunk data is unavailable.
func (a *DefaultWorldAccessor) GetBlock(x, y, z int) int32 {
	state, available := a.LookupBlockState(x, y, z)
	if !available {
		return -1
	}
	return state
}

// LookupBlockState reports block availability without substituting unloaded data with air.
func (a *DefaultWorldAccessor) LookupBlockState(x, y, z int) (int32, bool) {
	if a == nil || a.Chunks == nil {
		return 0, false
	}
	c, ok := a.Chunks.PeekLoaded(int32(x>>4), int32(z>>4))
	if !ok || y < c.Data().MinY() || y > c.Data().MaxY() {
		return 0, false
	}
	return c.GetBlock(x, y, z), true
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

// NotifyNeighbors queues production updates or invokes the legacy adapter.
func (a *DefaultWorldAccessor) NotifyNeighbors(x, y, z int) {
	if a == nil || a.Chunks == nil {
		return
	}
	if a.UpdateNeighbors != nil {
		a.UpdateNeighbors(x, y, z)
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

// BlockEntities returns the block entity store.
func (a *DefaultWorldAccessor) BlockEntities() *blockentity.Store {
	if a == nil {
		return nil
	}
	return a.BlockEntityStore
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
	if a.RegisterItem != nil {
		a.RegisterItem(ie)
	} else {
		a.Items.Add(ie)
	}
	return ie
}

// ManagesItemTracking reports whether item spawns are owned by the runtime entity tracker.
func (a *DefaultWorldAccessor) ManagesItemTracking() bool { return a != nil && a.RegisterItem != nil }

// worldOwnsItemTracking keeps compatibility broadcasts confined to standalone accessors.
func worldOwnsItemTracking(world WorldAccessor) bool {
	owner, ok := world.(interface{ ManagesItemTracking() bool })
	return ok && owner.ManagesItemTracking()
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

// SleepThreat checks nearby monsters on the authoritative world tick.
func (a *DefaultWorldAccessor) SleepThreat(x, y, z int) bool {
	return a != nil && a.ReadSleepThreat != nil && a.ReadSleepThreat(x, y, z)
}

// Thundering reports whether daytime sleep is allowed by a thunderstorm.
func (a *DefaultWorldAccessor) Thundering() bool {
	return a != nil && a.ReadThunder != nil && a.ReadThunder()
}

// ClearSleepWeather resets weather when the weather cycle permits it.
func (a *DefaultWorldAccessor) ClearSleepWeather() {
	if a != nil && a.ClearWeatherAfterSleep != nil {
		a.ClearWeatherAfterSleep()
	}
}
