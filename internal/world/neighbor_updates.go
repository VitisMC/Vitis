package world

import (
	"sort"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/block/behavior"
	"github.com/vitismc/vitis/internal/block/fluid"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/portal"
	"github.com/vitismc/vitis/internal/world/tick"
)

const neighborQueueCapacity = 32768
const neighborWorkPerTick = 100000
const neighborBorderWorkPerTick = 4096

// neighborUpdate preserves source coordinates and the stable notification order.
type neighborUpdate struct {
	x, y, z                   int
	sourceX, sourceY, sourceZ int
}

// NotifyNeighbors queues resident neighbors without recursively mutating blocks.
// All calls and processing belong to the common world tick goroutine.
func (w *World) NotifyNeighbors(x, y, z int) {
	if w == nil {
		return
	}
	for _, offset := range [6][3]int{{-1, 0, 0}, {1, 0, 0}, {0, -1, 0}, {0, 1, 0}, {0, 0, -1}, {0, 0, 1}} {
		nx, ny, nz := x+offset[0], y+offset[1], z+offset[2]
		state, available := w.LookupBlockState(nx, ny, nz)
		if !available || block.IsAir(state) {
			continue
		}
		b := behavior.GetByState(state)
		if b.ScheduleOnNeighborUpdate() && b.TickDelay() > 0 {
			w.ScheduleBlockTick(nx, ny, nz, b.TickDelay(), tick.PriorityNormal)
		}
		if fluid.IsFluid(state) {
			if config := fluid.GetFluidConfig(state); config != nil {
				w.scheduleFluidTick(nx, ny, nz, config.FlowSpeed)
			}
		}
		w.enqueueNeighbor(neighborUpdate{nx, ny, nz, x, y, z})
	}
}

// enqueueNeighbor spills saturation into the existing durable block-tick queue.
func (w *World) enqueueNeighbor(update neighborUpdate) {
	if w.neighborCount == neighborQueueCapacity {
		w.ScheduleBlockTick(update.x, update.y, update.z, 1, tick.PriorityHigh)
		return
	}
	if w.neighborUpdates == nil {
		w.neighborUpdates = make([]neighborUpdate, neighborQueueCapacity)
	}
	w.neighborUpdates[(w.neighborCursor+w.neighborCount)%neighborQueueCapacity] = update
	w.neighborCount++
}

// processNeighborUpdates bounds chained work while retaining overflow for later ticks.
func (w *World) processNeighborUpdates() {
	for w.neighborCount > 0 {
		update := w.neighborUpdates[w.neighborCursor]
		w.neighborCursor = (w.neighborCursor + 1) % neighborQueueCapacity
		w.neighborCount--
		state, available := w.LookupBlockState(update.x, update.y, update.z)
		if !available || block.IsAir(state) {
			continue
		}
		if w.neighborWork >= neighborWorkPerTick {
			w.ScheduleBlockTick(update.x, update.y, update.z, 1, tick.PriorityHigh)
			continue
		}
		w.neighborWork++
		behavior.GetByState(state).OnNeighborUpdate(&behavior.Context{X: update.x, Y: update.y, Z: update.z, StateID: state, SourceX: update.sourceX, SourceY: update.sourceY, SourceZ: update.sourceZ})
		if current, available := w.LookupBlockState(update.x, update.y, update.z); available && !block.IsAir(current) {
			w.checkNeighborShape(update, current, false)
		}
	}
	w.neighborCursor = 0
}

// checkNeighborShape applies implemented shape changes using current resident data.
func (w *World) checkNeighborShape(update neighborUpdate, state int32, scheduled bool) {
	if block.NameFromState(state) == "minecraft:nether_portal" {
		w.checkPortalNeighbor(update, state, scheduled)
		return
	}
	b, ok := behavior.GetByState(state).(behavior.NeighborBehavior)
	if !ok {
		return
	}
	result := b.OnNeighborCheck(&behavior.NeighborContext{
		X: update.x, Y: update.y, Z: update.z, StateID: state, Light: w.RawBrightness(update.x, update.y, update.z),
		SourceX: update.sourceX, SourceY: update.sourceY, SourceZ: update.sourceZ, Scheduled: scheduled, GetBlock: w.lookupNeighborBlock,
	})
	if result == nil {
		return
	}
	if result.Delay > 0 {
		w.ScheduleBlockTick(update.x, update.y, update.z, result.Delay, tick.PriorityNormal)
	}
	if result.Changed && result.StateID >= 0 && result.StateID != state {
		w.setBlockAt(update.x, update.y, update.z, result.StateID)
		w.broadcastBlockUpdate(update.x, update.y, update.z, result.StateID)
		if result.Drop && w.GameRule("doTileDrops") == "true" {
			for _, drop := range behavior.GetByState(state).OnBreak(&behavior.Context{X: update.x, Y: update.y, Z: update.z, StateID: state}) {
				if drop.Count > 0 {
					w.SpawnItemEntity(entity.NewItemEntity(w.entities.AllocateID(), protocol.GenerateUUID(), entity.Vec3{X: float64(update.x) + .5, Y: float64(update.y) + .5, Z: float64(update.z) + .5}, inventory.NewSlot(drop.ItemID, drop.Count)))
				}
			}
		}
	}
}

// checkPortalNeighbor retains incomplete knowledge and collapses invalid resident frames.
func (w *World) checkPortalNeighbor(update neighborUpdate, state int32, scheduled bool) {
	axis := block.PropertiesFromState(state)["axis"]
	if !scheduled {
		if (axis == "x" && update.sourceZ != update.z) || (axis == "z" && update.sourceX != update.x) || block.NameFromState(w.getBlockAt(update.sourceX, update.sourceY, update.sourceZ)) == "minecraft:nether_portal" {
			return
		}
	}
	available := true
	lookup := func(x, y, z int) int32 {
		value := w.getBlockAt(x, y, z)
		available = available && value >= 0
		return value
	}
	frame, valid := portal.FindFrameAxis(update.x, update.y, update.z, axis, lookup)
	if valid {
		for _, pos := range frame.Interior {
			if lookup(pos.X, pos.Y, pos.Z) != state {
				valid = false
			}
		}
	}
	if !valid && available {
		w.setBlockAt(update.x, update.y, update.z, 0)
		w.broadcastBlockUpdate(update.x, update.y, update.z, 0)
	}
}

// lookupNeighborBlock treats build-height boundaries as air only in resident columns.
func (w *World) lookupNeighborBlock(x, y, z int) (int32, bool) {
	c, available := w.chunks.PeekLoaded(int32(x>>4), int32(z>>4))
	if !available {
		return 0, false
	}
	return c.GetBlock(x, y, z), true
}

// queueChunkNeighborChecks retains bounded scan cursors for resident chunk borders.
func (w *World) queueChunkNeighborChecks(c *chunk.Chunk) {
	if c == nil || c.IsPregenerationOnly() {
		return
	}
	if w.neighborBorders == nil {
		w.neighborBorders = make(map[int64]int)
	}
	if _, queued := w.neighborBorders[c.Key()]; !queued {
		w.neighborBorders[c.Key()] = 0
	}
}

// processChunkNeighborChecks spreads border reconciliation across world ticks.
func (w *World) processChunkNeighborChecks() {
	keys := make([]int64, 0, len(w.neighborBorders))
	for key := range w.neighborBorders {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	start := sort.Search(len(keys), func(i int) bool { return keys[i] > w.neighborBorderCursor })
	budget := neighborBorderWorkPerTick
	for step := range keys {
		key := keys[(start+step)%len(keys)]
		w.neighborBorderCursor = key
		c, loaded := w.chunks.PeekLoaded(int32(key>>32), int32(key))
		if !loaded || c.IsPregenerationOnly() {
			delete(w.neighborBorders, key)
			continue
		}
		cursor := w.neighborBorders[key]
		end := (c.Data().MaxY() - c.Data().MinY() + 1) * 128
		for cursor < end && budget > 0 {
			y, along, side := c.Data().MinY()+cursor/128, cursor/8%16, cursor%8
			local := [8][2]int{{0, along}, {15, along}, {along, 0}, {along, 15}, {-1, along}, {16, along}, {along, -1}, {along, 16}}[side]
			x, z := int(c.X())*16+local[0], int(c.Z())*16+local[1]
			state, available := w.LookupBlockState(x, y, z)
			if available && hasNeighborShape(state) {
				w.ScheduleBlockTick(x, y, z, 1, tick.PriorityHigh)
			}
			cursor++
			budget--
		}
		if cursor == end {
			delete(w.neighborBorders, key)
		} else {
			w.neighborBorders[key] = cursor
		}
		if budget == 0 {
			return
		}
	}
}

// hasNeighborShape identifies implemented support checks for load reconciliation.
func hasNeighborShape(state int32) bool {
	_, supported := behavior.GetByState(state).(behavior.NeighborBehavior)
	return supported || block.NameFromState(state) == "minecraft:nether_portal"
}
