package chunk

import (
	"errors"
	"testing"
)

// TestDeferredUnloadWaitsForDurableRevision keeps state accessible across asynchronous saves.
func TestDeferredUnloadWaitsForDurableRevision(t *testing.T) {
	storage := NewStorage(1)
	c := New(0, 0)
	c.MarkDirty()
	storage.Set(c)
	m := NewManager(ManagerConfig{Storage: storage})
	var revision uint64
	var saves int
	m.SetUnloadPreparer(func(c *Chunk) (bool, error) {
		saves++
		revision = c.Revision()
		c.BeginSave()
		return false, nil
	})
	evicted := false
	m.SetEvictionHandler(func(*Chunk) { evicted = true })
	if !m.MarkForUnload(0, 0) || m.ProcessUnloads(1) != 0 {
		t.Fatal("evicted before persistence")
	}
	if _, ok := m.PeekLoaded(0, 0); !ok || evicted {
		t.Fatal("pending chunk became unavailable")
	}
	c.MarkDirty()
	c.FinishSave()
	c.ClearDirtyIfRevision(revision)
	if m.ProcessUnloads(1) != 0 || saves != 2 {
		t.Fatal("stale revision was accepted instead of retrying")
	}
	c.FinishSave()
	c.ClearDirtyIfRevision(revision)
	if m.ProcessUnloads(1) != 1 || !evicted || m.Len() != 0 {
		t.Fatal("durably saved chunk was not evicted")
	}
}

// TestDeferredUnloadFailureAndRetention preserve unsaved state and cancel reacquired evictions.
func TestDeferredUnloadFailureAndRetention(t *testing.T) {
	storage := NewStorage(1)
	c := New(0, 0)
	c.MarkDirty()
	storage.Set(c)
	m := NewManager(ManagerConfig{Storage: storage})
	m.SetUnloadPreparer(func(*Chunk) (bool, error) { return false, errors.New("write failed") })
	m.MarkForUnload(0, 0)
	if m.ProcessUnloads(1) != 0 || !c.Dirty() {
		t.Fatal("write failure discarded state")
	}
	m.SetUnloadPreparer(func(*Chunk) (bool, error) { return false, nil })
	m.MarkForUnload(0, 0)
	m.ProcessUnloads(1)
	retained := true
	m.SetRetentionPredicate(func(int32, int32) bool { return retained })
	m.ProcessUnloads(1)
	retained = false
	c.ClearDirty()
	if m.ProcessUnloads(1) != 0 || m.Len() != 1 {
		t.Fatal("reacquired chunk eviction was not cancelled")
	}
}

// TestUnloadQueueOverflowLeavesChunkAvailable rejects work without orphaning its state.
func TestUnloadQueueOverflowLeavesChunkAvailable(t *testing.T) {
	storage := NewStorage(2)
	storage.Set(New(0, 0))
	storage.Set(New(1, 0))
	m := NewManager(ManagerConfig{Storage: storage, UnloadQueueCapacity: 1})
	if !m.MarkForUnload(0, 0) || m.MarkForUnload(1, 0) {
		t.Fatal("unexpected unload queue capacity")
	}
	if _, ok := m.PeekLoaded(1, 0); !ok {
		t.Fatal("overflowed unload left unavailable chunk")
	}
}
