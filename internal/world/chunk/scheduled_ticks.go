package chunk

import (
	"fmt"
	"math"

	"github.com/vitismc/vitis/internal/block"
	genfluid "github.com/vitismc/vitis/internal/data/generated/fluid"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/tick"
)

// HasScheduledTicks reports runtime tick state that needs a fresh remaining-delay snapshot.
func (c *Chunk) HasScheduledTicks() bool {
	return c != nil && c.tickScheduler != nil && c.tickScheduler.PendingInChunk(c.X(), c.Z()) != 0
}

// BindScheduledTicks connects persistence to the authoritative world's scheduler.
// It must be called on the world tick after a chunk is published.
func (c *Chunk) BindScheduledTicks(scheduler *tick.Scheduler, clock func() uint64) error {
	if c == nil || c.Data() == nil || scheduler == nil || clock == nil {
		return fmt.Errorf("bind scheduled ticks: invalid chunk or scheduler")
	}
	if c.tickScheduler != nil {
		return nil
	}
	root := c.Data().PersistentNBT()
	now := clock()
	var restored []tick.ScheduledTick
	if root != nil {
		for kind, name := range []string{"block_ticks", "fluid_ticks"} {
			list, ok := root.GetList(name)
			if !ok {
				if _, _, present := root.Get(name); present {
					return fmt.Errorf("%s: invalid tick list", name)
				}
				continue
			}
			if list.ElementType() != nbt.TagCompound && !(list.Len() == 0 && list.ElementType() == nbt.TagEnd) {
				return fmt.Errorf("%s: invalid tick element type", name)
			}
			for _, entry := range list.Elements() {
				value, ok := entry.(*nbt.Compound)
				if !ok {
					return fmt.Errorf("%s: invalid tick entry", name)
				}
				x, xOK := value.GetInt("x")
				y, yOK := value.GetInt("y")
				z, zOK := value.GetInt("z")
				id, idOK := value.GetString("i")
				delay, delayOK := value.GetInt("t")
				priority, priorityOK := value.GetInt("p")
				if !xOK || !yOK || !zOK || !idOK || !delayOK || !priorityOK || x>>4 != c.X() || z>>4 != c.Z() || int(y) < c.Data().MinY() || int(y) > c.Data().MaxY() || priority < -3 || priority > 3 || uint64(max(delay, 0)) > math.MaxUint64-now {
					return fmt.Errorf("%s: invalid tick position or fields", name)
				}
				subID := block.DefaultStateID(id)
				if kind == int(tick.TickTypeFluid) {
					subID = genfluid.FluidIDByName(id)
				}
				if subID < 0 {
					return fmt.Errorf("%s: unknown tick type %q", name, id)
				}
				// Negative remaining delays are already due and run after activation.
				restored = append(restored, tick.ScheduledTick{X: int(x), Y: int(y), Z: int(z), TargetTick: uint64(max(delay, 0)), Priority: tick.Priority(priority), Type: tick.TickType(kind), SubID: subID})
			}
		}
	}
	for _, pending := range restored {
		scheduler.Schedule(pending.X, pending.Y, pending.Z, now, int(pending.TargetTick), pending.Priority, pending.Type, pending.SubID)
	}
	c.tickScheduler, c.tickClock = scheduler, clock
	return nil
}

// encodeScheduledTicksNBT refreshes tick lists only for world-bound chunks.
func encodeScheduledTicksNBT(root *nbt.Compound, c *Chunk) error {
	if c.tickScheduler == nil || c.tickClock == nil {
		return nil
	}
	lists := [2]*nbt.List{nbt.NewList(nbt.TagCompound), nbt.NewList(nbt.TagCompound)}
	now := c.tickClock()
	for _, scheduled := range c.tickScheduler.SnapshotChunk(c.X(), c.Z()) {
		id := block.NameFromState(scheduled.SubID)
		if scheduled.Type == tick.TickTypeFluid {
			id = genfluid.FluidNameByID(scheduled.SubID)
		}
		if id == "" {
			return fmt.Errorf("encode scheduled tick: unknown type %d", scheduled.SubID)
		}
		delay := int32(0)
		if scheduled.TargetTick > now {
			remaining := scheduled.TargetTick - now
			if remaining > math.MaxInt32 {
				return fmt.Errorf("encode scheduled tick: delay exceeds int32")
			}
			delay = int32(remaining)
		}
		entry := nbt.NewCompound().PutString("i", id).PutInt("x", int32(scheduled.X)).PutInt("y", int32(scheduled.Y)).PutInt("z", int32(scheduled.Z)).PutInt("t", delay).PutInt("p", int32(scheduled.Priority))
		lists[scheduled.Type].Add(entry)
	}
	root.PutList("block_ticks", lists[0]).PutList("fluid_ticks", lists[1])
	return nil
}
