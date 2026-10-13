package chunk

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	genfluid "github.com/vitismc/vitis/internal/data/generated/fluid"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/tick"
)

// TestScheduledTickSnapshotRoundTrip preserves timing, negative coordinates, and immutable snapshots.
func TestScheduledTickSnapshotRoundTrip(t *testing.T) {
	c := New(-1, 2)
	scheduler := tick.NewScheduler()
	now := uint64(100)
	if err := c.BindScheduledTicks(scheduler, func() uint64 { return now }); err != nil {
		t.Fatal(err)
	}
	scheduler.Schedule(-1, 64, 32, now, 20, tick.PriorityLow, tick.TickTypeBlock, block.DefaultStateID("minecraft:stone"))
	scheduler.Schedule(-2, 64, 32, now, 5, tick.PriorityHigh, tick.TickTypeFluid, genfluid.FluidWater)
	snapshot, err := SnapshotForSave(c)
	if err != nil {
		t.Fatal(err)
	}
	now = 200
	scheduler.RemoveChunk(-1, 2)
	data, err := EncodeChunkNBT(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := DecodeChunkNBT(data, -1, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	restored := tick.NewScheduler()
	if err := loaded.BindScheduledTicks(restored, func() uint64 { return 300 }); err != nil {
		t.Fatal(err)
	}
	if got := restored.Drain(304, 10); len(got) != 0 {
		t.Fatal("tick fired early")
	}
	if got := restored.Drain(305, 10); len(got) != 1 || got[0].Type != tick.TickTypeFluid || got[0].Priority != tick.PriorityHigh {
		t.Fatalf("wrong fluid tick: %+v", got)
	}
	if got := restored.Drain(320, 10); len(got) != 1 || got[0].Type != tick.TickTypeBlock || got[0].Priority != tick.PriorityLow {
		t.Fatalf("wrong block tick: %+v", got)
	}
}

// TestScheduledTickRestoreIsAtomic rejects malformed data before scheduling any entry.
func TestScheduledTickRestoreIsAtomic(t *testing.T) {
	c := New(0, 0)
	list := nbt.NewList(nbt.TagCompound)
	for _, x := range []int32{1, 16} {
		list.Add(nbt.NewCompound().PutString("i", "minecraft:stone").PutInt("x", x).PutInt("y", 64).PutInt("z", 1).PutInt("t", 1).PutInt("p", 0))
	}
	c.Data().SetPersistentNBT(nbt.NewCompound().PutList("block_ticks", list))
	s := tick.NewScheduler()
	if c.BindScheduledTicks(s, func() uint64 { return 0 }) == nil || s.Pending() != 0 {
		t.Fatal("invalid chunk partially restored")
	}
}
