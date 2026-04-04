package event

import (
	"testing"
)

type testEvent struct {
	BaseEvent
	Value string
}

type testCancellableEvent struct {
	CancellableEvent
	Value string
}

func TestFireEvent(t *testing.T) {
	bus := NewBus()
	var received string
	bus.Subscribe(PlayerJoin, PriorityNormal, func(e Event) {
		received = e.(*testEvent).Value
	})

	bus.Fire(&testEvent{BaseEvent: BaseEvent{EventType: PlayerJoin}, Value: "hello"})
	if received != "hello" {
		t.Fatalf("expected 'hello', got %q", received)
	}
}

func TestPriorityOrder(t *testing.T) {
	bus := NewBus()
	var order []int

	bus.Subscribe(PlayerChat, PriorityHigh, func(e Event) {
		order = append(order, 2)
	})
	bus.Subscribe(PlayerChat, PriorityLow, func(e Event) {
		order = append(order, 1)
	})
	bus.Subscribe(PlayerChat, PriorityHighest, func(e Event) {
		order = append(order, 3)
	})

	bus.Fire(&testEvent{BaseEvent: BaseEvent{EventType: PlayerChat}})

	if len(order) != 3 {
		t.Fatalf("expected 3 handlers, got %d", len(order))
	}
	if order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Fatalf("expected [1,2,3], got %v", order)
	}
}

func TestCancellable(t *testing.T) {
	bus := NewBus()
	bus.Subscribe(BlockBreak, PriorityNormal, func(e Event) {
		if c, ok := e.(Cancellable); ok {
			c.SetCancelled(true)
		}
	})

	evt := &testCancellableEvent{
		CancellableEvent: CancellableEvent{BaseEvent: BaseEvent{EventType: BlockBreak}},
		Value:            "stone",
	}
	cancelled := bus.FireCancellable(evt)
	if !cancelled {
		t.Fatal("expected event to be cancelled")
	}
}

func TestUnsubscribe(t *testing.T) {
	bus := NewBus()
	called := 0
	id := bus.Subscribe(PlayerQuit, PriorityNormal, func(e Event) {
		called++
	})

	bus.Fire(&testEvent{BaseEvent: BaseEvent{EventType: PlayerQuit}})
	if called != 1 {
		t.Fatalf("expected 1, got %d", called)
	}

	bus.Unsubscribe(id)
	bus.Fire(&testEvent{BaseEvent: BaseEvent{EventType: PlayerQuit}})
	if called != 1 {
		t.Fatalf("expected still 1 after unsub, got %d", called)
	}
}

func TestCount(t *testing.T) {
	bus := NewBus()
	if bus.Count(PlayerJoin) != 0 {
		t.Fatal("expected 0")
	}
	bus.Subscribe(PlayerJoin, PriorityNormal, func(e Event) {})
	bus.Subscribe(PlayerJoin, PriorityHigh, func(e Event) {})
	if bus.Count(PlayerJoin) != 2 {
		t.Fatalf("expected 2, got %d", bus.Count(PlayerJoin))
	}
}

func TestNoHandlers(t *testing.T) {
	bus := NewBus()
	bus.Fire(&testEvent{BaseEvent: BaseEvent{EventType: PlayerMove}})
}

func BenchmarkFire(b *testing.B) {
	bus := NewBus()
	bus.Subscribe(PlayerMove, PriorityNormal, func(e Event) {})
	bus.Subscribe(PlayerMove, PriorityHigh, func(e Event) {})
	evt := &testEvent{BaseEvent: BaseEvent{EventType: PlayerMove}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Fire(evt)
	}
}
