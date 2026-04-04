package tick

import "sync/atomic"

// Task is a handle for a scheduled tick task.
type Task interface {
	ID() uint64
	Cancel() bool
	Cancelled() bool
	Async() bool
	Repeating() bool
	IntervalTicks() uint64
	NextRunTick() uint64
}

type taskRef struct {
	id        uint64
	async     bool
	repeating bool
	interval  uint64

	cancelled atomic.Bool
	nextTick  atomic.Uint64
}

func newTaskRef(id uint64, async bool, repeating bool, interval uint64, nextTick uint64) *taskRef {
	ref := &taskRef{
		id:        id,
		async:     async,
		repeating: repeating,
		interval:  interval,
	}
	ref.nextTick.Store(nextTick)
	return ref
}

func (t *taskRef) ID() uint64 {
	if t == nil {
		return 0
	}
	return t.id
}

func (t *taskRef) Cancel() bool {
	if t == nil {
		return false
	}
	return t.cancelled.CompareAndSwap(false, true)
}

func (t *taskRef) Cancelled() bool {
	if t == nil {
		return true
	}
	return t.cancelled.Load()
}

func (t *taskRef) Async() bool {
	if t == nil {
		return false
	}
	return t.async
}

func (t *taskRef) Repeating() bool {
	if t == nil {
		return false
	}
	return t.repeating
}

func (t *taskRef) IntervalTicks() uint64 {
	if t == nil {
		return 0
	}
	return t.interval
}

func (t *taskRef) NextRunTick() uint64 {
	if t == nil {
		return 0
	}
	return t.nextTick.Load()
}

type taskEntry struct {
	ref      *taskRef
	fn       func()
	interval uint64
	rounds   uint64
	next     *taskEntry
}

func (e *taskEntry) reset() {
	e.ref = nil
	e.fn = nil
	e.interval = 0
	e.rounds = 0
	e.next = nil
}
