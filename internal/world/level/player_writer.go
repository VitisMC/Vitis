package level

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// PlayerWrite is an independent snapshot and its monotonic tick capture sequence.
type PlayerWrite struct {
	Data     *PlayerData
	Sequence uint64
}

// PlayerWriter serializes bounded snapshot writes and retains failures for retry.
type PlayerWriter struct {
	queue    chan PlayerWrite
	stop     chan struct{}
	done     chan struct{}
	save     func(*PlayerData) error
	mu       sync.Mutex
	closing  bool
	stopOnce sync.Once
	failure  error
	latest   map[string]uint64
	pending  map[string]PlayerWrite
}

// NewPlayerWriter starts a single writer, preserving order for each UUID.
func NewPlayerWriter(capacity int, save func(*PlayerData) error) *PlayerWriter {
	if capacity < 1 {
		capacity = 1
	}
	writer := &PlayerWriter{queue: make(chan PlayerWrite, capacity), stop: make(chan struct{}), done: make(chan struct{}), save: save, latest: make(map[string]uint64), pending: make(map[string]PlayerWrite)}
	go writer.run()
	return writer
}

// Offer queues a snapshot without blocking the authoritative tick.
func (w *PlayerWriter) Offer(snapshot PlayerWrite) bool {
	if snapshot.Data == nil || snapshot.Data.UUID == "" {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closing {
		return false
	}
	select {
	case w.queue <- snapshot:
		sequence, saved := w.latest[snapshot.Data.UUID]
		previous, waiting := w.pending[snapshot.Data.UUID]
		if (!saved || snapshot.Sequence > sequence) && (!waiting || snapshot.Sequence >= previous.Sequence) {
			w.pending[snapshot.Data.UUID] = snapshot
		}
		return true
	default:
		return false
	}
}

// Pending returns an independent copy of the latest accepted, unwritten snapshot.
// Reconnecting players can read it without waiting for background disk IO.
func (w *PlayerWriter) Pending(uuid string) *PlayerData {
	w.mu.Lock()
	snapshot := w.pending[uuid]
	w.mu.Unlock()
	return snapshot.Data.Clone()
}

// Enqueue waits outside the tick when a final snapshot encounters backpressure.
func (w *PlayerWriter) Enqueue(ctx context.Context, snapshot PlayerWrite) error {
	if snapshot.Data == nil || snapshot.Data.UUID == "" {
		return fmt.Errorf("invalid player snapshot")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if w.Offer(snapshot) {
			return nil
		}
		w.mu.Lock()
		closing := w.closing
		w.mu.Unlock()
		if closing {
			return fmt.Errorf("player writer closed")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// Close flushes accepted snapshots and cancels persistent write retries at timeout.
func (w *PlayerWriter) Close(ctx context.Context) error {
	w.mu.Lock()
	if !w.closing {
		w.closing = true
		close(w.queue)
	}
	w.mu.Unlock()
	select {
	case <-w.done:
	case <-ctx.Done():
		w.stopOnce.Do(func() { close(w.stop) })
		<-w.done
		return errors.Join(ctx.Err(), w.Failure())
	}
	return w.Failure()
}

// Failure returns the last outstanding storage error.
func (w *PlayerWriter) Failure() error { w.mu.Lock(); defer w.mu.Unlock(); return w.failure }

// run writes snapshots sequentially and retries a failed write before progressing.
func (w *PlayerWriter) run() {
	defer close(w.done)
	for snapshot := range w.queue {
		w.mu.Lock()
		sequence, saved := w.latest[snapshot.Data.UUID]
		stale := saved && snapshot.Sequence <= sequence
		w.mu.Unlock()
		if stale {
			continue
		}
		for {
			err := w.save(snapshot.Data)
			w.mu.Lock()
			w.failure = err
			if err == nil {
				w.latest[snapshot.Data.UUID] = snapshot.Sequence
				if pending, exists := w.pending[snapshot.Data.UUID]; exists && pending.Sequence == snapshot.Sequence {
					delete(w.pending, snapshot.Data.UUID)
				}
				w.mu.Unlock()
				break
			}
			w.mu.Unlock()
			timer := time.NewTimer(100 * time.Millisecond)
			select {
			case <-w.stop:
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
}
