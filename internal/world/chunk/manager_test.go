package chunk

import (
	"context"
	"testing"
	"time"
)

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
