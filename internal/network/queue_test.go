package network

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBoundedMPSCQueueEnqueueFull(t *testing.T) {
	q := NewBoundedMPSCQueue[int](1)

	if err := q.Enqueue(1); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	if err := q.Enqueue(2); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got: %v", err)
	}

	item, ok, err := q.TryDequeue()
	if err != nil {
		t.Fatalf("unexpected dequeue error: %v", err)
	}
	if !ok || item != 1 {
		t.Fatalf("unexpected dequeue result ok=%v item=%d", ok, item)
	}
}

func TestBoundedMPSCQueueEnqueueWaitClose(t *testing.T) {
	q := NewBoundedMPSCQueue[int](1)
	if err := q.Enqueue(1); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	result := make(chan error, 1)
	go func() {
		result <- q.EnqueueWait(context.Background(), 2)
	}()

	time.Sleep(20 * time.Millisecond)
	q.Close()

	select {
	case err := <-result:
		if !errors.Is(err, ErrQueueClosed) {
			t.Fatalf("expected ErrQueueClosed, got: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("enqueue wait did not unblock on close")
	}
}

func TestBoundedMPSCQueueDequeueWaitCancel(t *testing.T) {
	q := NewBoundedMPSCQueue[int](1)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	_, err := q.DequeueWait(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got: %v", err)
	}
}
