package chunk

import (
	"context"
	"testing"
	"time"
)

type testGenerator struct{}

func (g testGenerator) Generate(x, z int32) (*Chunk, error) {
	return New(x, z), nil
}

func TestLoaderAsyncRequestAndCompletion(t *testing.T) {
	loader := NewLoader(LoaderConfig{
		Generator:           testGenerator{},
		WorkerCount:         1,
		WorkerQueueCapacity: 8,
		RequestQueueSize:    8,
		ResultQueueSize:     8,
		PumpBatch:           8,
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = loader.Close(ctx)
	}()

	if !loader.Request(4, 5) {
		t.Fatalf("expected request enqueue")
	}

	submitted := loader.Pump(8)
	if submitted != 1 {
		t.Fatalf("expected one submitted task, got %d", submitted)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		results := loader.DrainCompletions(nil, 4)
		if len(results) == 0 {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		if len(results) != 1 {
			t.Fatalf("expected one result, got %d", len(results))
		}
		result := results[0]
		if result.Err != nil {
			t.Fatalf("unexpected generation error: %v", result.Err)
		}
		if result.Chunk == nil {
			t.Fatalf("expected generated chunk")
		}
		if result.Chunk.X() != 4 || result.Chunk.Z() != 5 {
			t.Fatalf("unexpected chunk coordinates x=%d z=%d", result.Chunk.X(), result.Chunk.Z())
		}
		return
	}

	t.Fatalf("timed out waiting for completion")
}
