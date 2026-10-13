package chunk

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type testGenerator struct{}

func (g testGenerator) Generate(x, z int32) (*Chunk, error) {
	return New(x, z), nil
}

// TestLoaderContinuesWithoutAnotherPump keeps a small worker queue busy between ticks.
func TestLoaderContinuesWithoutAnotherPump(t *testing.T) {
	loader := NewLoader(LoaderConfig{Generator: testGenerator{}, WorkerCount: 2, WorkerQueueCapacity: 1, RequestQueueSize: 32, ResultQueueSize: 32, PumpBatch: 32})
	defer func() {
		if err := loader.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	for x := int32(0); x < 32; x++ {
		if !loader.Request(x, -x) {
			t.Fatal("request queue rejected work")
		}
	}
	if got := loader.Pump(32); got != 32 {
		t.Fatalf("dispatched %d, want 32", got)
	}
	deadline := time.Now().Add(time.Second)
	for loader.Completed() != 32 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if loader.Completed() != 32 {
		t.Fatalf("workers waited for another tick: completed %d", loader.Completed())
	}
	if got := len(loader.DrainCompletions(nil, 32)); got != 32 {
		t.Fatalf("received %d completions", got)
	}
	if loader.Failed() != 0 {
		t.Fatal("queue backpressure counted as generation failure")
	}
}

// TestLoaderCloseCancelsSaturatedCompletionQueue prevents dispatcher and worker leaks.
func TestLoaderCloseCancelsSaturatedCompletionQueue(t *testing.T) {
	loader := NewLoader(LoaderConfig{Generator: testGenerator{}, WorkerCount: 1, WorkerQueueCapacity: 1, RequestQueueSize: 32, ResultQueueSize: 1, PumpBatch: 32})
	for x := int32(0); x < 32; x++ {
		loader.Request(x, 0)
	}
	loader.Pump(32)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := loader.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("saturated close returned %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for loader.Completed() != 32 && time.Now().Before(deadline) {
		loader.DrainCompletions(nil, 32)
		time.Sleep(time.Millisecond)
		select {
		case <-loader.dispatched:
			goto drained
		default:
		}
	}
drained:
	select {
	case <-loader.dispatched:
	case <-time.After(time.Second):
		t.Fatal("dispatcher leaked after cancellation")
	}
	if err := loader.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if loader.Request(100, 100) || loader.Pump(1) != 0 {
		t.Fatal("closed loader accepted work")
	}
}

// blockingTestGenerator controls worker saturation without timing assumptions.
type blockingTestGenerator struct {
	started atomic.Int32
	release chan struct{}
}

// Generate waits for the test to release all dispatched generation work.
func (g *blockingTestGenerator) Generate(x, z int32) (*Chunk, error) {
	g.started.Add(1)
	<-g.release
	return New(x, z), nil
}

// TestLoaderCloseDrainsAcceptedRequests verifies graceful shutdown of dispatch and workers.
func TestLoaderCloseDrainsAcceptedRequests(t *testing.T) {
	generator := &blockingTestGenerator{release: make(chan struct{})}
	loader := NewLoader(LoaderConfig{Generator: generator, WorkerCount: 1, WorkerQueueCapacity: 1, RequestQueueSize: 8, ResultQueueSize: 8, PumpBatch: 8})
	for x := int32(0); x < 8; x++ {
		loader.Request(x, 0)
	}
	if got := loader.Pump(8); got != 8 {
		t.Fatalf("dispatched %d requests", got)
	}
	closed := make(chan error, 1)
	go func() { closed <- loader.Close(context.Background()) }()
	close(generator.release)
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("close did not finish")
	}
	if got := loader.Completed(); got != 8 {
		t.Fatalf("close lost accepted requests: %d", got)
	}
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

func TestLoaderBuffersCompletionWhenResultQueueIsFull(t *testing.T) {
	loader := NewLoader(LoaderConfig{
		Generator:           testGenerator{},
		WorkerCount:         1,
		WorkerQueueCapacity: 2,
		RequestQueueSize:    2,
		ResultQueueSize:     1,
		PumpBatch:           2,
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = loader.Close(ctx)
	}()

	if !loader.Request(1, 1) || !loader.Request(2, 2) {
		t.Fatal("enqueue load requests")
	}
	if submitted := loader.Pump(2); submitted != 2 {
		t.Fatalf("submitted = %d, want 2", submitted)
	}

	deadline := time.Now().Add(time.Second)
	for loader.Completed() != 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if loader.Completed() != 2 {
		t.Fatal("worker blocked instead of buffering completion")
	}
	if pending := loader.PendingCompletions(); pending != 2 {
		t.Fatalf("pending completions = %d, want 2", pending)
	}
	if results := loader.DrainCompletions(nil, 2); len(results) != 2 {
		t.Fatalf("drained results = %d, want 2", len(results))
	}
}

func TestSortLoadRequestsByNearestPriorityCenter(t *testing.T) {
	requests := []LoadRequest{
		{X: 100, Z: 100},
		{X: 2, Z: 0},
		{X: 48, Z: 50},
		{X: 0, Z: 0},
	}

	sortLoadRequestsByCenters(requests, []LoadPriorityCenter{
		{X: 0, Z: 0},
		{X: 50, Z: 50},
	})

	want := []LoadRequest{
		{X: 0, Z: 0},
		{X: 2, Z: 0},
		{X: 48, Z: 50},
		{X: 100, Z: 100},
	}
	for index := range want {
		if requests[index] != want[index] {
			t.Fatalf("request[%d] = %+v, want %+v", index, requests[index], want[index])
		}
	}
}

func TestSortLoadRequestsHandlesCoordinateExtremes(t *testing.T) {
	requests := []LoadRequest{
		{X: 0, Z: 0},
		{X: -1 << 31, Z: -1 << 31},
		{X: 1<<31 - 1, Z: 1<<31 - 1},
	}

	sortLoadRequestsByCenters(requests, []LoadPriorityCenter{{X: -1 << 31, Z: -1 << 31}})

	if requests[0].X != -1<<31 || requests[0].Z != -1<<31 {
		t.Fatalf("nearest extreme request = %+v", requests[0])
	}
}
