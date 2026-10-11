package world

import (
	"testing"
	"time"

	"github.com/vitismc/vitis/internal/block"
	genfluid "github.com/vitismc/vitis/internal/data/generated/fluid"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/persistence"
	"github.com/vitismc/vitis/internal/world/tick"
)

// TestScheduledTicksSurviveAsyncUnloadAndReload exercises persistence through the world lifecycle.
func TestScheduledTicksSurviveAsyncUnloadAndReload(t *testing.T) {
	store := persistence.NewChunkStore(t.TempDir())
	storage := chunk.NewStorage(1)
	c := chunk.New(-1, 2)
	storage.Set(c)
	manager := chunk.NewManager(chunk.ManagerConfig{Storage: storage, OnUnload: makeChunkSaveFunc(store)})
	w, err := New(Config{Name: "paused-ticks", ChunkManager: manager, ChunkStore: store, AutosaveIntervalTicks: 1200})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close(nil) })
	w.updateActivity()
	w.tickLighting()
	w.tickScheduler.Schedule(-1, 64, 32, w.CurrentTick(), 0, tick.PriorityHigh, tick.TickTypeBlock, block.DefaultStateID("minecraft:stone"))
	w.tickScheduler.Schedule(-2, 64, 32, w.CurrentTick(), 0, tick.PriorityLow, tick.TickTypeFluid, genfluid.FluidWater)
	// Simulate an earlier autosave with no subsequent block or queue mutations.
	c.ClearDirty()
	if !manager.MarkForUnload(-1, 2) {
		t.Fatal("unload rejected")
	}
	manager.ProcessUnloads(1)
	if _, ok := manager.PeekLoaded(-1, 2); !ok || w.tickScheduler.Pending() != 2 {
		t.Fatal("unsaved tick state disappeared")
	}
	deadline := time.Now().Add(2 * time.Second)
	for manager.IsTracked(-1, 2) && time.Now().Before(deadline) {
		w.tick.Add(1)
		w.autosave.tick(w)
		manager.ProcessUnloads(1)
		time.Sleep(time.Millisecond)
	}
	if manager.IsTracked(-1, 2) || w.tickScheduler.Pending() != 0 {
		t.Fatal("durable unload retained runtime ticks")
	}
	data, err := store.ReadChunkNBT(-1, 2)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := chunk.DecodeChunkNBT(data, -1, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	storage.Set(reloaded)
	w.updateActivity()
	w.processScheduledTicks()
	if w.tickScheduler.Pending() != 2 {
		t.Fatal("inactive reloaded ticks were lost or executed")
	}
	activateTestChunk(t, w, storage, -1, 2)
	w.processScheduledTicks()
	if w.tickScheduler.Pending() != 0 {
		t.Fatal("reactivated ticks did not execute")
	}
}

// TestShutdownRefreshesUnchangedTickDelays preserves elapsed time after an earlier autosave.
func TestShutdownRefreshesUnchangedTickDelays(t *testing.T) {
	path := t.TempDir()
	store := persistence.NewChunkStore(path)
	storage := chunk.NewStorage(1)
	c := chunk.New(0, 0)
	storage.Set(c)
	w, err := New(Config{Name: "tick-delay", ChunkStore: store, ChunkManager: chunk.NewManager(chunk.ManagerConfig{Storage: storage, OnUnload: makeChunkSaveFunc(store)})})
	if err != nil {
		t.Fatal(err)
	}
	w.updateActivity()
	w.tickScheduler.Schedule(8, 64, 8, 0, 100, tick.PriorityNormal, tick.TickTypeBlock, block.DefaultStateID("minecraft:stone"))
	if _, err := w.SaveAllDirty(); err != nil {
		t.Fatal(err)
	}
	w.tick.Store(75)
	if err := w.Close(nil); err != nil {
		t.Fatal(err)
	}
	reopened := persistence.NewChunkStore(path)
	t.Cleanup(func() { _ = reopened.Close() })
	data, err := reopened.ReadChunkNBT(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := chunk.DecodeChunkNBT(data, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	scheduler := tick.NewScheduler()
	if err := loaded.BindScheduledTicks(scheduler, func() uint64 { return 0 }); err != nil {
		t.Fatal(err)
	}
	pending := scheduler.SnapshotChunk(0, 0)
	if len(pending) != 1 || pending[0].TargetTick != 25 {
		t.Fatalf("saved remaining delay: %+v", pending)
	}
}
