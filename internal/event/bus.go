package event

import (
	"sort"
	"sync"
	"sync/atomic"
)

// Bus dispatches events to registered handlers sorted by priority.
type Bus struct {
	mu     sync.RWMutex
	subs   map[Type][]Subscription
	nextID atomic.Uint64
}

// NewBus creates a new event bus.
func NewBus() *Bus {
	return &Bus{
		subs: make(map[Type][]Subscription),
	}
}

// Subscribe registers a handler for an event type with the given priority.
// Returns a subscription ID that can be used to unsubscribe.
func (b *Bus) Subscribe(eventType Type, priority Priority, handler Handler) uint64 {
	id := b.nextID.Add(1)
	sub := Subscription{
		EventType: eventType,
		Priority:  priority,
		Handler:   handler,
		id:        id,
	}

	b.mu.Lock()
	b.subs[eventType] = append(b.subs[eventType], sub)
	sort.Slice(b.subs[eventType], func(i, j int) bool {
		return b.subs[eventType][i].Priority < b.subs[eventType][j].Priority
	})
	b.mu.Unlock()

	return id
}

// Unsubscribe removes a handler by its subscription ID.
func (b *Bus) Unsubscribe(id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for eventType, subs := range b.subs {
		for i, s := range subs {
			if s.id == id {
				b.subs[eventType] = append(subs[:i], subs[i+1:]...)
				return
			}
		}
	}
}

// Fire dispatches an event to all registered handlers in priority order.
// For Cancellable events, handlers after cancellation still run (monitor pattern).
func (b *Bus) Fire(event Event) {
	b.mu.RLock()
	subs := b.subs[event.Type()]
	b.mu.RUnlock()

	for _, sub := range subs {
		sub.Handler(event)
	}
}

// FireCancellable dispatches a cancellable event and returns whether it was cancelled.
func (b *Bus) FireCancellable(event Cancellable) bool {
	b.Fire(event)
	return event.IsCancelled()
}

// Count returns the number of subscriptions for a given event type.
func (b *Bus) Count(eventType Type) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs[eventType])
}
