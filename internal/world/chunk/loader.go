package chunk

import (
	"context"
	"runtime"
	"sync/atomic"

	"github.com/vitismc/vitis/internal/network"
)

const (
	defaultLoaderWorkerQueueFactor = 64
	defaultLoaderPumpBatch         = 256
)

// LoaderConfig configures asynchronous chunk loading behavior.
type LoaderConfig struct {
	Generator           Generator
	WorkerPool          *network.WorkerPool
	WorkerCount         int
	WorkerQueueCapacity int
	RequestQueue        *LoadRequestQueue
	ResultQueue         *LoadResultQueue
	RequestQueueSize    int
	ResultQueueSize     int
	PumpBatch           int
}

// Loader executes chunk generation/loading off the world thread.
type Loader struct {
	generator Generator

	requests *LoadRequestQueue
	results  *LoadResultQueue

	workerPool *network.WorkerPool
	ownsPool   bool

	pumpBatch int

	requestScratch []LoadRequest

	submitted atomic.Uint64
	completed atomic.Uint64
	failed    atomic.Uint64
}

// NewLoader creates a bounded asynchronous chunk loader.
func NewLoader(config LoaderConfig) *Loader {
	generator := config.Generator
	if generator == nil {
		generator = NewStubGenerator()
	}

	requests := config.RequestQueue
	if requests == nil {
		requests = NewLoadRequestQueue(config.RequestQueueSize)
	}

	results := config.ResultQueue
	if results == nil {
		results = NewLoadResultQueue(config.ResultQueueSize)
	}

	pool := config.WorkerPool
	ownsPool := false
	if pool == nil {
		workerCount := config.WorkerCount
		if workerCount <= 0 {
			workerCount = runtime.GOMAXPROCS(0)
			if workerCount < 2 {
				workerCount = 2
			}
		}

		queueCapacity := config.WorkerQueueCapacity
		if queueCapacity <= 0 {
			queueCapacity = workerCount * defaultLoaderWorkerQueueFactor
		}

		pool = network.NewWorkerPool(network.WorkerPoolConfig{
			Size:          workerCount,
			QueueCapacity: queueCapacity,
		})
		ownsPool = true
	}

	pumpBatch := config.PumpBatch
	if pumpBatch <= 0 {
		pumpBatch = defaultLoaderPumpBatch
	}

	return &Loader{
		generator:      generator,
		requests:       requests,
		results:        results,
		workerPool:     pool,
		ownsPool:       ownsPool,
		pumpBatch:      pumpBatch,
		requestScratch: make([]LoadRequest, 0, pumpBatch),
	}
}

// Request enqueues one chunk load request without blocking.
func (l *Loader) Request(x, z int32) bool {
	if l == nil {
		return false
	}
	return l.requests.Enqueue(LoadRequest{X: x, Z: z})
}

// Pump submits up to max queued requests to the worker pool.
func (l *Loader) Pump(max int) int {
	if l == nil || max <= 0 {
		return 0
	}
	if max > l.pumpBatch {
		max = l.pumpBatch
	}

	batch := l.requests.Drain(l.requestScratch[:0], max)
	submitted := 0
	for index := range batch {
		request := batch[index]
		err := l.workerPool.Submit(func() {
			generated, generationErr := l.generator.Generate(request.X, request.Z)
			l.results.Enqueue(LoadResult{
				X:     request.X,
				Z:     request.Z,
				Chunk: generated,
				Err:   generationErr,
			})
			l.completed.Add(1)
		})
		if err != nil {
			l.requests.Enqueue(request)
			for remaining := index + 1; remaining < len(batch); remaining++ {
				l.requests.Enqueue(batch[remaining])
			}
			l.failed.Add(1)
			return submitted
		}
		submitted++
		l.submitted.Add(1)
	}

	return submitted
}

// DrainCompletions drains up to max generated results into dst.
func (l *Loader) DrainCompletions(dst []LoadResult, max int) []LoadResult {
	if l == nil || max <= 0 {
		return dst
	}
	return l.results.Drain(dst, max)
}

// PendingRequests returns currently queued request count.
func (l *Loader) PendingRequests() int {
	if l == nil {
		return 0
	}
	return l.requests.Len()
}

// PendingCompletions returns currently queued completion count.
func (l *Loader) PendingCompletions() int {
	if l == nil {
		return 0
	}
	return l.results.Len()
}

// Submitted returns total worker-submitted load task count.
func (l *Loader) Submitted() uint64 {
	if l == nil {
		return 0
	}
	return l.submitted.Load()
}

// Completed returns total completed worker task count.
func (l *Loader) Completed() uint64 {
	if l == nil {
		return 0
	}
	return l.completed.Load()
}

// Failed returns total failed worker submit count.
func (l *Loader) Failed() uint64 {
	if l == nil {
		return 0
	}
	return l.failed.Load()
}

// Close stops owned worker resources.
func (l *Loader) Close(ctx context.Context) error {
	if l == nil || !l.ownsPool {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return l.workerPool.Stop(ctx)
}
