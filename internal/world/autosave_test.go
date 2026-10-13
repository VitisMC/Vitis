package world

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	genblock "github.com/vitismc/vitis/internal/data/generated/block"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/persistence"
)

// TestAutosaveBatchShutdownDrainsEverySnapshot verifies bounded batch ownership and acknowledgements.
func TestAutosaveBatchShutdownDrainsEverySnapshot(t *testing.T) {
	store := persistence.NewChunkStore(t.TempDir())
	defer store.Close()
	autosave := newChunkAutosave(store, 1200, 1, 32)
	chunks := make([]*chunk.Chunk, 32)
	for index := range chunks {
		c := chunk.New(int32(index-16), -1)
		c.SetBlock(3, 64, 4, genblock.StoneDefaultState)
		queued, err := autosave.enqueue(c)
		if err != nil || !queued {
			t.Fatalf("enqueue %d: %v %v", index, queued, err)
		}
		c.SetBlock(3, 64, 4, genblock.DirtDefaultState)
		chunks[index] = c
	}
	autosave.close()
	for index, c := range chunks {
		if c.SavePending() || !c.Dirty() {
			t.Fatalf("snapshot %d acknowledged a newer revision", index)
		}
		data, err := store.ReadChunkNBT(int(c.X()), int(c.Z()))
		if err != nil {
			t.Fatal(err)
		}
		reloaded, err := chunk.DecodeChunkNBT(data, c.X(), c.Z(), 0)
		if err != nil {
			t.Fatal(err)
		}
		if reloaded.Data().GetBlock(3, 64, 4) != genblock.StoneDefaultState {
			t.Fatal("saved a live mutation instead of frozen snapshot")
		}
	}
}

// TestAutosavePersistsDirtyChunk verifies that disk IO completes outside the tick.
func TestAutosavePersistsDirtyChunk(t *testing.T) {
	store := persistence.NewChunkStore(filepath.Join(t.TempDir(), "world"))
	storage := chunk.NewStorage(1)
	c := chunk.New(3, -2)
	c.MarkDirty()
	storage.Set(c)
	manager := chunk.NewManager(chunk.ManagerConfig{Storage: storage, OnUnload: makeChunkSaveFunc(store)})
	w, err := New(Config{
		Name: "autosave", ChunkManager: manager, ChunkStore: store,
		AutosaveIntervalTicks: 1, AutosaveBatchPerTick: 1, AutosaveQueueCapacity: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := w.Close(context.Background()); err != nil {
			t.Errorf("close world: %v", err)
		}
	}()

	w.Tick()
	if !c.SavePending() {
		t.Fatal("autosave did not reserve the chunk")
	}
	deadline := time.Now().Add(2 * time.Second)
	for c.Dirty() && time.Now().Before(deadline) {
		w.Tick()
		time.Sleep(time.Millisecond)
	}
	if c.Dirty() || c.SavePending() {
		t.Fatal("autosave did not acknowledge the persisted revision")
	}
	data, err := store.ReadChunkNBT(3, -2)
	if err != nil || len(data) == 0 {
		t.Fatalf("read autosaved chunk: bytes=%d err=%v", len(data), err)
	}
}

// TestAutosaveEncoderReadsFrozenRevision separates background encoding from live mutations.
func TestAutosaveEncoderReadsFrozenRevision(t *testing.T) {
	store := persistence.NewChunkStore(t.TempDir())
	defer store.Close()
	autosave := newChunkAutosave(store, 1200, 1, 1)
	defer autosave.close()
	c := chunk.New(-1, 2)
	c.SetBlock(3, 64, 4, genblock.StoneDefaultState)
	queued, err := autosave.enqueue(c)
	if err != nil || !queued {
		t.Fatalf("enqueue: queued=%v err=%v", queued, err)
	}
	for range 200 {
		c.SetBlock(3, 64, 4, genblock.DirtDefaultState)
		c.MarkDirty()
	}
	select {
	case result := <-autosave.results:
		if result.err != nil {
			t.Fatal(result.err)
		}
		autosave.acknowledge(result)
	case <-time.After(2 * time.Second):
		t.Fatal("encoder did not finish")
	}
	if !c.Dirty() || c.SavePending() {
		t.Fatal("old snapshot acknowledged newer changes")
	}
	encoded, err := store.ReadChunkNBT(-1, 2)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := chunk.DecodeChunkNBT(encoded, -1, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Data().GetBlock(3, 64, 4); got != genblock.StoneDefaultState {
		t.Fatalf("saved live mutation instead of snapshot: %d", got)
	}
}

// TestAutosaveAcknowledgementPreservesNewerChanges rejects stale saves.
func TestAutosaveAcknowledgementPreservesNewerChanges(t *testing.T) {
	c := chunk.New(0, 0)
	c.MarkDirty()
	if !c.BeginSave() {
		t.Fatal("could not reserve chunk")
	}
	oldRevision := c.Revision()
	c.MarkDirty()
	a := &chunkAutosave{}
	a.acknowledge(chunkSaveResult{snapshot: chunkSaveSnapshot{chunk: c, revision: oldRevision}})
	if !c.Dirty() || c.SavePending() {
		t.Fatal("stale autosave cleared a newer mutation")
	}
}

// TestAutosaveWriteFailureKeepsChunkDirty allows the next cycle or shutdown to retry.
func TestAutosaveWriteFailureKeepsChunkDirty(t *testing.T) {
	c := chunk.New(0, 0)
	c.MarkDirty()
	if !c.BeginSave() {
		t.Fatal("could not reserve chunk")
	}
	a := &chunkAutosave{}
	a.acknowledge(chunkSaveResult{
		snapshot: chunkSaveSnapshot{chunk: c, revision: c.Revision()},
		err:      errors.New("disk failure"),
	})
	if !c.Dirty() || c.SavePending() {
		t.Fatal("failed write discarded unsaved chunk state")
	}
}

// TestWorldCloseDrainsAutosave verifies that shutdown waits for queued disk writes.
func TestWorldCloseDrainsAutosave(t *testing.T) {
	worldDir := filepath.Join(t.TempDir(), "world")
	store := persistence.NewChunkStore(worldDir)
	storage := chunk.NewStorage(1)
	c := chunk.New(-1, 5)
	c.MarkDirty()
	storage.Set(c)
	manager := chunk.NewManager(chunk.ManagerConfig{Storage: storage, OnUnload: makeChunkSaveFunc(store)})
	w, err := New(Config{
		Name: "autosave-close", ChunkManager: manager, ChunkStore: store,
		AutosaveIntervalTicks: 1, AutosaveBatchPerTick: 1, AutosaveQueueCapacity: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	w.Tick()
	if err := w.Close(context.Background()); err != nil {
		t.Fatalf("close world: %v", err)
	}
	if c.Dirty() || c.SavePending() {
		t.Fatal("shutdown did not acknowledge the queued chunk write")
	}
	reopened := persistence.NewChunkStore(worldDir)
	defer reopened.Close()
	data, err := reopened.ReadChunkNBT(-1, 5)
	if err != nil || len(data) == 0 {
		t.Fatalf("read saved chunk after shutdown: bytes=%d err=%v", len(data), err)
	}
}
