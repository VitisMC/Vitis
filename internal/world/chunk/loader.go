package chunk

import (
	"context"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/vitismc/vitis/internal/network"
)

const (
	defaultLoaderWorkerQueueFactor = 1
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
	dispatch   chan LoadRequest
	dispatched chan struct{}
	context    context.Context
	cancel     context.CancelFunc
	closed     atomic.Bool
	closeOnce  sync.Once

	pumpBatch int

	requestScratch  []LoadRequest
	priorityCenters []LoadPriorityCenter

	completionMu       sync.Mutex
	completionOverflow []LoadResult
	completionCapacity int

	submitted atomic.Uint64
	completed atomic.Uint64
	failed    atomic.Uint64
}

// LoadPriorityCenter identifies a player-centered source of chunk demand.
type LoadPriorityCenter struct {
	X int32
	Z int32
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

	ctx, cancel := context.WithCancel(context.Background())
	loader := &Loader{
		generator:          generator,
		requests:           requests,
		results:            results,
		workerPool:         pool,
		ownsPool:           ownsPool,
		pumpBatch:          pumpBatch,
		requestScratch:     make([]LoadRequest, 0, pumpBatch),
		completionOverflow: make([]LoadResult, 0, results.Capacity()),
		completionCapacity: results.Capacity(),
		dispatch:           make(chan LoadRequest, pumpBatch),
		dispatched:         make(chan struct{}),
		context:            ctx,
		cancel:             cancel,
	}
	go loader.dispatchRequests()
	return loader
}

// dispatchRequests keeps workers supplied between world ticks with bounded pending work.
func (l *Loader) dispatchRequests() {
	defer close(l.dispatched)
	for request := range l.dispatch {
		err := l.workerPool.SubmitWait(l.context, func() {
			generated, generationErr := l.generator.Generate(request.X, request.Z)
			l.enqueueCompletion(LoadResult{X: request.X, Z: request.Z, Chunk: generated, Err: generationErr})
			l.completed.Add(1)
		})
		if err != nil {
			l.failed.Add(1)
			l.enqueueCompletion(LoadResult{X: request.X, Z: request.Z, Err: err})
		}
	}
}

// Request enqueues one chunk load request without blocking.
func (l *Loader) Request(x, z int32) bool {
	if l == nil || l.closed.Load() {
		return false
	}
	return l.requests.Enqueue(LoadRequest{X: x, Z: z})
}

// SetPriorityCenters updates positions used to order work that has not started.
// The caller owns world-thread serialization with Pump and Request.
func (l *Loader) SetPriorityCenters(centers []LoadPriorityCenter) {
	if l == nil {
		return
	}
	l.priorityCenters = append(l.priorityCenters[:0], centers...)
}

// Pump dispatches up to max prioritized requests without blocking the world thread.
func (l *Loader) Pump(max int) int {
	if l == nil || max <= 0 || l.closed.Load() {
		return 0
	}
	if max > l.pumpBatch {
		max = l.pumpBatch
	}

	queued := l.requests.Len()
	if queued < max {
		queued = max
	}
	batch := l.requests.Drain(l.requestScratch[:0], queued)
	sortLoadRequestsByCenters(batch, l.priorityCenters)
	submitted := 0
	for index := range batch {
		if submitted >= max {
			for remaining := index; remaining < len(batch); remaining++ {
				l.requests.Enqueue(batch[remaining])
			}
			break
		}
		request := batch[index]
		select {
		case l.dispatch <- request:
			submitted++
			l.submitted.Add(1)
		default:
			l.requests.Enqueue(request)
			for remaining := index + 1; remaining < len(batch); remaining++ {
				l.requests.Enqueue(batch[remaining])
			}
			return submitted
		}
	}

	return submitted
}

func sortLoadRequestsByCenters(requests []LoadRequest, centers []LoadPriorityCenter) {
	if len(requests) < 2 || len(centers) == 0 {
		return
	}
	distance := func(request LoadRequest) uint64 {
		best := ^uint64(0)
		for _, center := range centers {
			dx := absCoordinateDelta(request.X, center.X)
			dz := absCoordinateDelta(request.Z, center.Z)
			dxSquared, dzSquared := dx*dx, dz*dz
			value := dxSquared + dzSquared
			if value < dxSquared {
				value = ^uint64(0)
			}
			if value < best {
				best = value
			}
		}
		return best
	}
	sort.SliceStable(requests, func(i, j int) bool {
		return distance(requests[i]) < distance(requests[j])
	})
}

func absCoordinateDelta(a, b int32) uint64 {
	delta := int64(a) - int64(b)
	if delta < 0 {
		delta = -delta
	}
	return uint64(delta)
}

// DrainCompletions drains up to max generated results into dst.
func (l *Loader) DrainCompletions(dst []LoadResult, max int) []LoadResult {
	if l == nil || max <= 0 {
		return dst
	}
	initialLen := len(dst)
	dst = l.results.Drain(dst, max)
	remaining := max - (len(dst) - initialLen)
	if remaining <= 0 {
		return dst
	}

	l.completionMu.Lock()
	if remaining > len(l.completionOverflow) {
		remaining = len(l.completionOverflow)
	}
	dst = append(dst, l.completionOverflow[:remaining]...)
	copy(l.completionOverflow, l.completionOverflow[remaining:])
	clear(l.completionOverflow[len(l.completionOverflow)-remaining:])
	l.completionOverflow = l.completionOverflow[:len(l.completionOverflow)-remaining]
	l.completionMu.Unlock()
	return dst
}

// PendingRequests returns currently queued request count.
func (l *Loader) PendingRequests() int {
	if l == nil {
		return 0
	}
	return l.requests.Len() + len(l.dispatch)
}

// PendingCompletions returns currently queued completion count.
func (l *Loader) PendingCompletions() int {
	if l == nil {
		return 0
	}
	l.completionMu.Lock()
	overflow := len(l.completionOverflow)
	l.completionMu.Unlock()
	return l.results.Len() + overflow
}

func (l *Loader) enqueueCompletion(result LoadResult) {
	if l.results.TryEnqueue(result) {
		return
	}

	l.completionMu.Lock()
	if len(l.completionOverflow) < l.completionCapacity {
		l.completionOverflow = append(l.completionOverflow, result)
		l.completionMu.Unlock()
		return
	}
	l.completionMu.Unlock()

	// The bounded overflow is exhausted. Blocking here preserves the completion
	// rather than dropping a chunk in StateLoading; the world tick will drain it.
	select {
	case l.results.ch <- result:
	case <-l.context.Done():
	}
}

// Submitted returns the number of load tasks accepted for worker dispatch.
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

// Close drains dispatched work and stops owned workers; Pump must already be stopped.
func (l *Loader) Close(ctx context.Context) error {
	if l == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	l.closeOnce.Do(func() {
		l.closed.Store(true)
		close(l.dispatch)
	})
	select {
	case <-l.dispatched:
	case <-ctx.Done():
		l.cancel()
		if l.ownsPool {
			_ = l.workerPool.Stop(ctx)
		}
		return ctx.Err()
	}
	if l.ownsPool {
		if err := l.workerPool.Stop(ctx); err != nil {
			l.cancel()
			return err
		}
	}
	l.cancel()
	return nil
}
