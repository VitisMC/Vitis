package event

// Handler is a function that processes an event.
type Handler func(Event)

// Subscription binds a handler to an event type with a priority.
type Subscription struct {
	EventType Type
	Priority  Priority
	Handler   Handler
	id        uint64
}
