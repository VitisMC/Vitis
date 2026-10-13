package chunk

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestManagerReleaseSavedChunk protects runtime state from stale save acknowledgements.
func TestManagerReleaseSavedChunk(t *testing.T) {
	for _, scenario := range []string{"clean", "dirty", "revision", "pending", "access", "entities", "loading", "replacement", "missing", "runtime_get", "runtime_touch", "runtime_request"} {
		t.Run(scenario, func(t *testing.T) {
			manager := &Manager{storage: NewStorage(1)}
			manager.SetTick(10)
			c := New(-4, 7)
			c.pregenerationOnly.Store(true)
			c.Touch(10)
			manager.storage.Set(c)
			revision := c.Revision()
			switch scenario {
			case "dirty":
				c.MarkDirty()
			case "revision":
				c.MarkDirty()
				c.ClearDirty()
			case "pending":
				c.BeginSave()
			case "access":
				c.Touch(11)
			case "entities":
				c.SetEntities([]uint64{1})
			case "loading":
				c.SetState(StateLoading)
			case "replacement":
				manager.storage.Set(New(c.X(), c.Z()))
			case "missing":
				manager.storage.Delete(c.X(), c.Z())
			case "runtime_get":
				manager.GetChunk(c.X(), c.Z())
			case "runtime_touch":
				manager.Touch(c.X(), c.Z())
			case "runtime_request":
				manager.RequestLoad(c.X(), c.Z())
			}
			if released := manager.ReleaseSavedChunk(c, revision, 10); released != (scenario == "clean") {
				t.Fatalf("release = %v", released)
			}
			if scenario != "clean" && scenario != "missing" && !manager.IsTracked(c.X(), c.Z()) {
				t.Fatal("release discarded live runtime state")
			}
			if scenario == "clean" && manager.IsTracked(c.X(), c.Z()) {
				t.Fatal("durably saved chunk was retained")
			}
		})
	}
}

// TestManagerVisitLoadedPreservesUnloadScan keeps incremental eviction scans stable.
func TestManagerVisitLoadedPreservesUnloadScan(t *testing.T) {
	manager := &Manager{storage: NewStorage(3), keyScratch: []int64{ChunkKey(5, 7)}, scanCursor: 1}
	manager.storage.Set(New(1, 2))
	manager.storage.Set(New(3, 4))
	visited := 0
	manager.VisitLoaded(func(*Chunk) { visited++ })
	if visited != 2 || len(manager.keyScratch) != 1 || manager.keyScratch[0] != ChunkKey(5, 7) || manager.scanCursor != 1 {
		t.Fatal("visiting chunks overwrote the eviction scan")
	}
}

func TestManagerRequestLoadDeduplicatesLoadingState(t *testing.T) {
	manager := NewManager(ManagerConfig{
		Loader: NewLoader(LoaderConfig{
			Generator:           testGenerator{},
			WorkerCount:         1,
			WorkerQueueCapacity: 8,
			RequestQueueSize:    8,
			ResultQueueSize:     8,
			PumpBatch:           8,
		}),
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = manager.Close(ctx)
	}()

	if !manager.RequestLoad(2, 3) {
		t.Fatalf("expected first load request to succeed")
	}
	if !manager.RequestLoad(2, 3) {
		t.Fatalf("expected duplicate load request to be accepted")
	}

	if manager.Len() != 1 {
		t.Fatalf("expected one placeholder chunk, got %d", manager.Len())
	}
	if manager.PendingLoadRequests() != 1 {
		t.Fatalf("expected only one queued request, got %d", manager.PendingLoadRequests())
	}

	placeholder, ok := manager.storage.Get(2, 3)
	if !ok || placeholder == nil {
		t.Fatalf("expected loading placeholder chunk")
	}
	if placeholder.State() != StateLoading {
		t.Fatalf("expected loading state, got %d", placeholder.State())
	}
}

func TestManagerFailedSaveKeepsChunkLoadedAndDirty(t *testing.T) {
	saveErr := errors.New("write failed")
	manager := NewManager(ManagerConfig{
		OnUnload: func(*Chunk) error { return saveErr },
	})
	chunk := New(4, 7)
	chunk.MarkDirty()
	manager.storage.Set(chunk)

	if !manager.MarkForUnload(4, 7) {
		t.Fatal("expected chunk to be queued for unload")
	}
	if removed := manager.ProcessUnloads(1); removed != 0 {
		t.Fatalf("removed %d chunks after failed save", removed)
	}
	loaded, ok := manager.GetChunk(4, 7)
	if !ok || loaded != chunk {
		t.Fatal("failed save removed the chunk")
	}
	if !loaded.Dirty() {
		t.Fatal("failed save cleared dirty state")
	}

	saved, err := manager.SaveAllDirty()
	if saved != 0 || !errors.Is(err, saveErr) {
		t.Fatalf("SaveAllDirty() = (%d, %v), want (0, %v)", saved, err, saveErr)
	}
}

// TestManagerDoesNotUnloadDuringAutosave protects against stale writes after unload.
func TestManagerDoesNotUnloadDuringAutosave(t *testing.T) {
	manager := NewManager(ManagerConfig{OnUnload: func(*Chunk) error { return nil }})
	c := New(4, 7)
	c.MarkDirty()
	manager.storage.Set(c)
	if !c.BeginSave() || !manager.MarkForUnload(4, 7) {
		t.Fatal("could not schedule the in-flight save and unload")
	}
	if removed := manager.ProcessUnloads(1); removed != 0 {
		t.Fatalf("unloaded %d chunks during autosave", removed)
	}
	if current, ok := manager.storage.Get(4, 7); !ok || current != c || !c.Dirty() {
		t.Fatal("autosaving chunk was lost or marked clean")
	}
	c.FinishSave()
}

func TestManagerApplyCompletionsAndRetrieveChunk(t *testing.T) {
	manager := NewManager(ManagerConfig{
		Loader: NewLoader(LoaderConfig{
			Generator:           testGenerator{},
			WorkerCount:         1,
			WorkerQueueCapacity: 8,
			RequestQueueSize:    8,
			ResultQueueSize:     8,
			PumpBatch:           8,
		}),
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = manager.Close(ctx)
	}()

	manager.SetTick(10)
	if !manager.RequestLoad(6, 7) {
		t.Fatalf("expected request enqueue")
	}
	if submitted := manager.PumpLoadRequests(); submitted != 1 {
		t.Fatalf("expected one submitted load task, got %d", submitted)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		applied := manager.ApplyLoadCompletions(8)
		if applied == 0 {
			time.Sleep(5 * time.Millisecond)
			continue
		}

		loaded, ok := manager.GetChunk(6, 7)
		if !ok || loaded == nil {
			t.Fatalf("expected loaded chunk")
		}
		if loaded.State() != StateLoaded {
			t.Fatalf("expected loaded state, got %d", loaded.State())
		}
		if loaded.LastAccessTick() != manager.CurrentTick() {
			t.Fatalf("expected access tick to match manager tick")
		}
		return
	}

	t.Fatalf("timed out waiting for load completion")
}

func TestManagerUnloadBehaviorBatched(t *testing.T) {
	manager := NewManager(ManagerConfig{})
	manager.SetTick(100)

	chunkA := New(1, 1)
	chunkA.Touch(1)
	chunkB := New(2, 2)
	chunkB.Touch(1)
	manager.storage.Set(chunkA)
	manager.storage.Set(chunkB)

	if !manager.MarkForUnload(1, 1) {
		t.Fatalf("expected chunk A marked for unload")
	}
	if !manager.MarkForUnload(2, 2) {
		t.Fatalf("expected chunk B marked for unload")
	}

	removed := manager.ProcessUnloads(1)
	if removed != 1 {
		t.Fatalf("expected one removed chunk in batch, got %d", removed)
	}
	if manager.Len() != 1 {
		t.Fatalf("expected one chunk remaining after first batch")
	}

	removed = manager.ProcessUnloads(1)
	if removed != 1 {
		t.Fatalf("expected second removed chunk, got %d", removed)
	}
	if manager.Len() != 0 {
		t.Fatalf("expected all chunks removed, got %d", manager.Len())
	}
}
