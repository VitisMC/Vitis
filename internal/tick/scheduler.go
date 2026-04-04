package tick

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/vitismc/vitis/internal/network"
)

var (
	ErrSchedulerClosed    = errors.New("scheduler closed")
	ErrSchedulerQueueFull = errors.New("scheduler queue full")
	ErrInvalidTask        = errors.New("invalid task")
	ErrInvalidInterval    = errors.New("invalid interval")
)

const (
	defaultIngressCapacity   = 4096
	defaultWheelSize         = 4096
	defaultAsyncQueueFactor  = 128
	minimumSchedulerCapacity = 64
)

// Scheduler schedules sync and async tasks for execution from the tick loop.
type Scheduler interface {
	RunTask(fn func()) (Task, error)
	RunTaskLater(fn func(), delayTicks uint64) (Task, error)
	RunRepeatingTask(fn func(), intervalTicks uint64) (Task, error)
	RunAsyncTask(fn func()) (Task, error)
	RunAsyncTaskLater(fn func(), delayTicks uint64) (Task, error)
	RunAsyncRepeatingTask(fn func(), intervalTicks uint64) (Task, error)
	CurrentTick() uint64
	PendingTasks() int64
}

// SchedulerConfig configures scheduler queue sizes and async worker execution.
type SchedulerConfig struct {
	IngressCapacity     int
	WheelSize           int
	AsyncWorkerPool     *network.WorkerPool
	AsyncWorkerPoolSize int
	AsyncQueueCapacity  int
	PanicHandler        func(any, []byte)
}

type scheduleRequest struct {
	ref      *taskRef
	fn       func()
	delay    uint64
	runAt    uint64
	interval uint64
	async    bool
	repeat   bool
}

type taskBucket struct {
	head *taskEntry
}

// DefaultScheduler is a bounded, timing-wheel-based scheduler implementation.
type DefaultScheduler struct {
	ingress chan scheduleRequest

	closed   atomic.Uint32
	closeMux sync.Once

	currentTick atomic.Uint64
	nextTaskID  atomic.Uint64
	pending     atomic.Int64

	wheel     []taskBucket
	wheelSize uint64

	dueHead *taskEntry
	dueTail *taskEntry

	taskPool sync.Pool

	asyncWorkerPool *network.WorkerPool
	ownsWorkerPool  bool
}

// NewScheduler creates a scheduler optimized for single-consumer tick execution.
func NewScheduler(cfg SchedulerConfig) (*DefaultScheduler, error) {
	if cfg.IngressCapacity < minimumSchedulerCapacity {
		cfg.IngressCapacity = defaultIngressCapacity
	}
	if cfg.WheelSize < minimumSchedulerCapacity {
		cfg.WheelSize = defaultWheelSize
	}

	workerPool := cfg.AsyncWorkerPool
	ownsPool := false
	if workerPool == nil {
		workerCount := cfg.AsyncWorkerPoolSize
		if workerCount <= 0 {
			workerCount = runtime.GOMAXPROCS(0)
			if workerCount < 2 {
				workerCount = 2
			}
		}

		queueCapacity := cfg.AsyncQueueCapacity
		if queueCapacity <= 0 {
			queueCapacity = workerCount * defaultAsyncQueueFactor
		}

		workerPool = network.NewWorkerPool(network.WorkerPoolConfig{
			Size:          workerCount,
			QueueCapacity: queueCapacity,
			PanicHandler:  cfg.PanicHandler,
		})
		ownsPool = true
	}

	s := &DefaultScheduler{
		ingress:         make(chan scheduleRequest, cfg.IngressCapacity),
		wheel:           make([]taskBucket, cfg.WheelSize),
		wheelSize:       uint64(cfg.WheelSize),
		asyncWorkerPool: workerPool,
		ownsWorkerPool:  ownsPool,
	}
	s.taskPool.New = func() any {
		return &taskEntry{}
	}
	return s, nil
}

// RunTask schedules a sync task for the current tick.
func (s *DefaultScheduler) RunTask(fn func()) (Task, error) {
	return s.schedule(fn, 0, 0, false, false)
}

// RunTaskLater schedules a sync task after delayTicks.
func (s *DefaultScheduler) RunTaskLater(fn func(), delayTicks uint64) (Task, error) {
	return s.schedule(fn, delayTicks, 0, false, false)
}

// RunRepeatingTask schedules a sync task repeating every intervalTicks.
func (s *DefaultScheduler) RunRepeatingTask(fn func(), intervalTicks uint64) (Task, error) {
	if intervalTicks == 0 {
		return nil, ErrInvalidInterval
	}
	return s.schedule(fn, intervalTicks, intervalTicks, false, true)
}

// RunAsyncTask schedules an async task for worker-pool execution.
func (s *DefaultScheduler) RunAsyncTask(fn func()) (Task, error) {
	return s.schedule(fn, 0, 0, true, false)
}

// RunAsyncTaskLater schedules an async task after delayTicks.
func (s *DefaultScheduler) RunAsyncTaskLater(fn func(), delayTicks uint64) (Task, error) {
	return s.schedule(fn, delayTicks, 0, true, false)
}

// RunAsyncRepeatingTask schedules an async repeating task.
func (s *DefaultScheduler) RunAsyncRepeatingTask(fn func(), intervalTicks uint64) (Task, error) {
	if intervalTicks == 0 {
		return nil, ErrInvalidInterval
	}
	return s.schedule(fn, intervalTicks, intervalTicks, true, true)
}

// CurrentTick returns the current tick observed by the scheduler.
func (s *DefaultScheduler) CurrentTick() uint64 {
	if s == nil {
		return 0
	}
	return s.currentTick.Load()
}

// PendingTasks returns the number of active scheduled task handles.
func (s *DefaultScheduler) PendingTasks() int64 {
	if s == nil {
		return 0
	}
	return s.pending.Load()
}

// Tick advances scheduler state and executes due tasks for tickNumber.
func (s *DefaultScheduler) Tick(tickNumber uint64) {
	if s == nil {
		return
	}
	s.currentTick.Store(tickNumber)
	s.drainIngress()
	s.collectDueForTick(tickNumber)
	s.executeDue(tickNumber)
}

// Close rejects new scheduling requests.
func (s *DefaultScheduler) Close() {
	if s == nil {
		return
	}
	s.closeMux.Do(func() {
		s.closed.Store(1)
	})
}

// Stop cancels pending tasks and stops owned async worker pool.
func (s *DefaultScheduler) Stop(ctx context.Context, cancelPending bool) error {
	if s == nil {
		return nil
	}
	s.Close()

	if cancelPending {
		s.cancelPendingTasks()
	}

	if s.ownsWorkerPool {
		if ctx == nil {
			ctx = context.Background()
		}
		if err := s.asyncWorkerPool.Stop(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (s *DefaultScheduler) schedule(fn func(), delay uint64, interval uint64, async bool, repeat bool) (Task, error) {
	if s == nil {
		return nil, ErrSchedulerClosed
	}
	if fn == nil {
		return nil, ErrInvalidTask
	}
	if s.closed.Load() == 1 {
		return nil, ErrSchedulerClosed
	}

	nowTick := s.currentTick.Load()
	nextTick := nowTick + delay
	if delay == 0 {
		nextTick = nowTick
	}

	id := s.nextTaskID.Add(1)
	ref := newTaskRef(id, async, repeat, interval, nextTick)
	request := scheduleRequest{
		ref:      ref,
		fn:       fn,
		delay:    delay,
		runAt:    nextTick,
		interval: interval,
		async:    async,
		repeat:   repeat,
	}

	select {
	case s.ingress <- request:
		s.pending.Add(1)
		return ref, nil
	default:
		return nil, ErrSchedulerQueueFull
	}
}

func (s *DefaultScheduler) drainIngress() {
	for {
		select {
		case request := <-s.ingress:
			s.acceptRequest(request)
		default:
			return
		}
	}
}

func (s *DefaultScheduler) acceptRequest(request scheduleRequest) {
	if request.ref == nil || request.fn == nil {
		s.pending.Add(-1)
		return
	}
	if request.ref.Cancelled() {
		s.completeEntryRef(request.ref)
		return
	}

	entry := s.acquireTaskEntry()
	entry.ref = request.ref
	entry.fn = request.fn
	entry.interval = request.interval

	if request.delay == 0 {
		s.enqueueDue(entry)
		return
	}

	currentTick := s.currentTick.Load()
	if request.runAt <= currentTick {
		s.enqueueDue(entry)
		return
	}

	s.enqueueDelayed(entry, request.runAt-currentTick)
}

func (s *DefaultScheduler) collectDueForTick(tickNumber uint64) {
	if len(s.wheel) == 0 {
		return
	}

	slot := tickNumber % s.wheelSize
	head := s.wheel[slot].head
	s.wheel[slot].head = nil

	for head != nil {
		next := head.next
		head.next = nil

		if head.ref == nil || head.ref.Cancelled() {
			s.completeEntry(head)
			head = next
			continue
		}

		if head.rounds > 0 {
			head.rounds--
			head.next = s.wheel[slot].head
			s.wheel[slot].head = head
			head = next
			continue
		}

		s.enqueueDue(head)
		head = next
	}
}

func (s *DefaultScheduler) executeDue(tickNumber uint64) {
	for {
		entry := s.dequeueDue()
		if entry == nil {
			return
		}

		if entry.ref == nil || entry.ref.Cancelled() {
			s.completeEntry(entry)
			continue
		}

		if entry.ref.Async() {
			err := s.asyncWorkerPool.Submit(entry.fn)
			if err != nil {
				if errors.Is(err, network.ErrWorkerPoolFull) {
					s.enqueueDelayed(entry, 1)
					continue
				}
				entry.ref.Cancel()
				s.completeEntry(entry)
				continue
			}
		} else {
			s.safeRun(entry.fn)
		}

		if entry.ref.Repeating() && !entry.ref.Cancelled() && s.closed.Load() == 0 {
			interval := entry.interval
			if interval == 0 {
				interval = 1
			}
			s.enqueueDelayed(entry, interval)
			continue
		}

		entry.ref.nextTick.Store(tickNumber)
		s.completeEntry(entry)
	}
}

func (s *DefaultScheduler) enqueueDelayed(entry *taskEntry, delay uint64) {
	if entry == nil {
		return
	}
	if delay == 0 {
		s.enqueueDue(entry)
		return
	}

	baseTick := s.currentTick.Load()
	targetTick := baseTick + delay
	if entry.ref != nil {
		entry.ref.nextTick.Store(targetTick)
	}

	entry.rounds = (delay - 1) / s.wheelSize
	slot := targetTick % s.wheelSize

	entry.next = s.wheel[slot].head
	s.wheel[slot].head = entry
}

func (s *DefaultScheduler) enqueueDue(entry *taskEntry) {
	if entry == nil {
		return
	}
	entry.next = nil
	if s.dueTail == nil {
		s.dueHead = entry
		s.dueTail = entry
		return
	}
	s.dueTail.next = entry
	s.dueTail = entry
}

func (s *DefaultScheduler) dequeueDue() *taskEntry {
	head := s.dueHead
	if head == nil {
		return nil
	}
	s.dueHead = head.next
	if s.dueHead == nil {
		s.dueTail = nil
	}
	head.next = nil
	return head
}

func (s *DefaultScheduler) acquireTaskEntry() *taskEntry {
	entry := s.taskPool.Get().(*taskEntry)
	entry.reset()
	return entry
}

func (s *DefaultScheduler) releaseTaskEntry(entry *taskEntry) {
	if entry == nil {
		return
	}
	entry.reset()
	s.taskPool.Put(entry)
}

func (s *DefaultScheduler) completeEntryRef(ref *taskRef) {
	if ref != nil {
		ref.nextTick.Store(0)
	}
	s.pending.Add(-1)
}

func (s *DefaultScheduler) completeEntry(entry *taskEntry) {
	if entry == nil {
		return
	}
	s.completeEntryRef(entry.ref)
	s.releaseTaskEntry(entry)
}

func (s *DefaultScheduler) safeRun(fn func()) {
	defer func() {
		_ = recover()
	}()
	fn()
}

func (s *DefaultScheduler) cancelPendingTasks() {
	for {
		select {
		case request := <-s.ingress:
			if request.ref != nil {
				request.ref.Cancel()
			}
			s.pending.Add(-1)
		default:
			goto doneIngress
		}
	}

doneIngress:
	for {
		entry := s.dequeueDue()
		if entry == nil {
			break
		}
		if entry.ref != nil {
			entry.ref.Cancel()
		}
		s.completeEntry(entry)
	}

	for i := 0; i < len(s.wheel); i++ {
		head := s.wheel[i].head
		s.wheel[i].head = nil
		for head != nil {
			next := head.next
			head.next = nil
			if head.ref != nil {
				head.ref.Cancel()
			}
			s.completeEntry(head)
			head = next
		}
	}

	if pending := s.pending.Load(); pending < 0 {
		s.pending.Store(0)
	}
}

func (s *DefaultScheduler) String() string {
	if s == nil {
		return "scheduler<nil>"
	}
	return fmt.Sprintf("scheduler{tick=%d,pending=%d}", s.CurrentTick(), s.PendingTasks())
}
