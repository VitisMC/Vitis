package network

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrQueueClosed = errors.New("queue closed")
	ErrQueueFull   = errors.New("queue full")
)

// BoundedMPSCQueue is a bounded multi-producer, single-consumer ring queue.
type BoundedMPSCQueue[T any] struct {
	mu       sync.Mutex
	buf      []T
	head     int
	tail     int
	size     int
	closed   bool
	closeCh  chan struct{}
	hasItems chan struct{}
	hasSpace chan struct{}
}

// NewBoundedMPSCQueue creates a queue with a fixed capacity.
func NewBoundedMPSCQueue[T any](capacity int) *BoundedMPSCQueue[T] {
	if capacity <= 0 {
		capacity = 128
	}

	return &BoundedMPSCQueue[T]{
		buf:      make([]T, capacity),
		closeCh:  make(chan struct{}),
		hasItems: make(chan struct{}, 1),
		hasSpace: make(chan struct{}, 1),
	}
}

// Enqueue inserts an item and returns ErrQueueFull when capacity is exhausted.
func (q *BoundedMPSCQueue[T]) Enqueue(item T) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return ErrQueueClosed
	}
	if q.size == len(q.buf) {
		return ErrQueueFull
	}

	wasEmpty := q.size == 0
	q.buf[q.tail] = item
	q.tail = (q.tail + 1) % len(q.buf)
	q.size++

	if wasEmpty {
		q.notify(q.hasItems)
	}

	return nil
}

// EnqueueWait inserts an item and blocks until space is available or ctx is cancelled.
func (q *BoundedMPSCQueue[T]) EnqueueWait(ctx context.Context, item T) error {
	for {
		q.mu.Lock()
		if q.closed {
			q.mu.Unlock()
			return ErrQueueClosed
		}

		if q.size < len(q.buf) {
			wasEmpty := q.size == 0
			q.buf[q.tail] = item
			q.tail = (q.tail + 1) % len(q.buf)
			q.size++
			if wasEmpty {
				q.notify(q.hasItems)
			}
			q.mu.Unlock()
			return nil
		}
		q.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-q.closeCh:
			return ErrQueueClosed
		case <-q.hasSpace:
		}
	}
}

// DequeueWait pops an item and blocks until data is available or ctx is cancelled.
func (q *BoundedMPSCQueue[T]) DequeueWait(ctx context.Context) (T, error) {
	var zero T

	for {
		q.mu.Lock()
		if q.size > 0 {
			item := q.buf[q.head]
			q.buf[q.head] = zero
			q.head = (q.head + 1) % len(q.buf)
			wasFull := q.size == len(q.buf)
			q.size--
			if wasFull {
				q.notify(q.hasSpace)
			}
			q.mu.Unlock()
			return item, nil
		}

		if q.closed {
			q.mu.Unlock()
			return zero, ErrQueueClosed
		}
		q.mu.Unlock()

		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-q.closeCh:
			return zero, ErrQueueClosed
		case <-q.hasItems:
		}
	}
}

// TryDequeue pops an item immediately when available.
func (q *BoundedMPSCQueue[T]) TryDequeue() (item T, ok bool, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.size == 0 {
		if q.closed {
			return item, false, ErrQueueClosed
		}
		return item, false, nil
	}

	item = q.buf[q.head]
	var zero T
	q.buf[q.head] = zero
	q.head = (q.head + 1) % len(q.buf)
	wasFull := q.size == len(q.buf)
	q.size--
	if wasFull {
		q.notify(q.hasSpace)
	}

	return item, true, nil
}

// Close marks the queue as closed and wakes blocked producers/consumer.
func (q *BoundedMPSCQueue[T]) Close() {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.closed = true
	close(q.closeCh)
	q.mu.Unlock()

	q.notify(q.hasItems)
	q.notify(q.hasSpace)
}

// Len returns the current queue length.
func (q *BoundedMPSCQueue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.size
}

// Capacity returns the queue capacity.
func (q *BoundedMPSCQueue[T]) Capacity() int {
	return len(q.buf)
}

func (q *BoundedMPSCQueue[T]) notify(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}
