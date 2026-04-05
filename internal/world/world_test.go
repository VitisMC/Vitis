package world

import (
	"context"
	"testing"
	"time"
)

func TestWorldTickAppliesAsyncChunkCompletions(t *testing.T) {
	instance, err := New(Config{Name: "test_world"})
	if err != nil {
		t.Fatalf("create world failed: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = instance.ChunkManager().Close(ctx)
	}()

	instance.LoadChunk(12, 34)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		instance.Tick()
		loaded, ok := instance.GetChunk(12, 34)
		if !ok || loaded == nil {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		if loaded.X() != 12 || loaded.Z() != 34 {
			t.Fatalf("unexpected loaded chunk coordinates x=%d z=%d", loaded.X(), loaded.Z())
		}
		return
	}

	t.Fatalf("timed out waiting for chunk load completion")
}

func TestWorldUnloadChunkRemovesData(t *testing.T) {
	instance, err := New(Config{Name: "unload_world"})
	if err != nil {
		t.Fatalf("create world failed: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = instance.ChunkManager().Close(ctx)
	}()

	instance.LoadChunk(2, 2)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		instance.Tick()
		if _, ok := instance.GetChunk(2, 2); ok {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if _, ok := instance.GetChunk(2, 2); !ok {
		t.Fatalf("expected loaded chunk before unload")
	}

	instance.UnloadChunk(2, 2)
	instance.Tick()

	if _, ok := instance.GetChunk(2, 2); ok {
		t.Fatalf("expected chunk to be removed after unload")
	}
}
