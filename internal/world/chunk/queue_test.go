package chunk

import "testing"

func TestLoadRequestQueueBoundedEnqueueAndDrain(t *testing.T) {
	queue := NewLoadRequestQueue(2)
	if !queue.Enqueue(LoadRequest{X: 1, Z: 1}) {
		t.Fatalf("expected first enqueue to succeed")
	}
	if !queue.Enqueue(LoadRequest{X: 2, Z: 2}) {
		t.Fatalf("expected second enqueue to succeed")
	}
	if queue.Enqueue(LoadRequest{X: 3, Z: 3}) {
		t.Fatalf("expected enqueue to fail for full queue")
	}

	drained := queue.Drain(nil, 2)
	if len(drained) != 2 {
		t.Fatalf("expected 2 drained requests, got %d", len(drained))
	}
	if drained[0].X != 1 || drained[1].X != 2 {
		t.Fatalf("expected fifo drain order")
	}
}

func TestLoadResultQueueDrainOrder(t *testing.T) {
	queue := NewLoadResultQueue(4)
	queue.Enqueue(LoadResult{X: 11, Z: 0})
	queue.Enqueue(LoadResult{X: 22, Z: 0})
	queue.Enqueue(LoadResult{X: 33, Z: 0})

	drained := queue.Drain(nil, 3)
	if len(drained) != 3 {
		t.Fatalf("expected 3 drained results, got %d", len(drained))
	}
	if drained[0].X != 11 || drained[1].X != 22 || drained[2].X != 33 {
		t.Fatalf("unexpected drain order")
	}
}

func TestUnloadQueueBoundedAndDrain(t *testing.T) {
	queue := NewUnloadQueue(2)
	if !queue.Enqueue(10) || !queue.Enqueue(20) {
		t.Fatalf("expected enqueue to succeed")
	}
	if queue.Enqueue(30) {
		t.Fatalf("expected enqueue to fail when full")
	}

	drained := queue.Drain(nil, 1)
	if len(drained) != 1 || drained[0] != 10 {
		t.Fatalf("unexpected first drain result")
	}

	if !queue.Enqueue(30) {
		t.Fatalf("expected enqueue to succeed after head compaction")
	}

	drained = queue.Drain(nil, 5)
	if len(drained) != 2 {
		t.Fatalf("expected 2 remaining keys, got %d", len(drained))
	}
	if drained[0] != 20 || drained[1] != 30 {
		t.Fatalf("unexpected drain order for remaining keys")
	}
}
