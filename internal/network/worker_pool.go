package network

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
)

var (
	ErrWorkerPoolClosed = errors.New("worker pool closed")
	ErrWorkerPoolFull   = errors.New("worker pool queue full")
)

// WorkerPool runs packet-processing tasks on a fixed number of workers.
type WorkerPool struct {
	tasks   chan func()
	panicFn func(any, []byte)

	closing chan struct{}
	closed  atomic.Uint32

	wg      sync.WaitGroup
	stopMux sync.Mutex
}

// WorkerPoolConfig defines worker pool sizing and panic behavior.
type WorkerPoolConfig struct {
	Size          int
	QueueCapacity int
	PanicHandler  func(any, []byte)
}

// NewWorkerPool creates and starts a fixed-size worker pool.
func NewWorkerPool(cfg WorkerPoolConfig) *WorkerPool {
	if cfg.Size <= 0 {
		cfg.Size = 1
	}
	if cfg.QueueCapacity <= 0 {
		cfg.QueueCapacity = cfg.Size * 128
	}

	pool := &WorkerPool{
		tasks:   make(chan func(), cfg.QueueCapacity),
		panicFn: cfg.PanicHandler,
		closing: make(chan struct{}),
	}

	pool.wg.Add(cfg.Size)
	for i := 0; i < cfg.Size; i++ {
		go pool.workerLoop()
	}

	return pool
}

// Submit enqueues a task without blocking and returns ErrWorkerPoolFull when saturated.
func (p *WorkerPool) Submit(task func()) error {
	if task == nil {
		return fmt.Errorf("submit task: nil task")
	}
	if p.closed.Load() == 1 {
		return ErrWorkerPoolClosed
	}

	select {
	case <-p.closing:
		return ErrWorkerPoolClosed
	case p.tasks <- task:
		return nil
	default:
		return ErrWorkerPoolFull
	}
}

// SubmitWait enqueues a task and blocks until queue space is available or ctx is cancelled.
func (p *WorkerPool) SubmitWait(ctx context.Context, task func()) error {
	if task == nil {
		return fmt.Errorf("submit task: nil task")
	}
	if p.closed.Load() == 1 {
		return ErrWorkerPoolClosed
	}

	for {
		select {
		case <-p.closing:
			return ErrWorkerPoolClosed
		case <-ctx.Done():
			return ctx.Err()
		case p.tasks <- task:
			return nil
		}
	}
}

// Stop gracefully stops the pool after currently queued tasks complete.
func (p *WorkerPool) Stop(ctx context.Context) error {
	p.stopMux.Lock()
	if !p.closed.CompareAndSwap(0, 1) {
		p.stopMux.Unlock()
		return nil
	}
	close(p.closing)
	p.stopMux.Unlock()

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *WorkerPool) workerLoop() {
	defer p.wg.Done()

	for {
		select {
		case task := <-p.tasks:
			if task != nil {
				p.safeRun(task)
			}
		case <-p.closing:
			for {
				select {
				case task := <-p.tasks:
					if task != nil {
						p.safeRun(task)
					}
				default:
					return
				}
			}
		}
	}
}

func (p *WorkerPool) safeRun(task func()) {
	defer func() {
		if r := recover(); r != nil {
			if p.panicFn != nil {
				p.panicFn(r, debug.Stack())
			}
		}
	}()
	task()
}
