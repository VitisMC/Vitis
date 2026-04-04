package event

// Type identifies an event kind.
type Type string

const (
	PlayerJoin        Type = "player_join"
	PlayerQuit        Type = "player_quit"
	PlayerChat        Type = "player_chat"
	PlayerCommand     Type = "player_command"
	BlockBreak        Type = "block_break"
	BlockPlace        Type = "block_place"
	PlayerMove        Type = "player_move"
	PlayerChangeWorld Type = "player_change_world"
)

// Event is the base interface for all events.
type Event interface {
	Type() Type
}

// Cancellable marks events that can be cancelled by handlers.
type Cancellable interface {
	Event
	IsCancelled() bool
	SetCancelled(bool)
}

// BaseEvent provides a default Type implementation.
type BaseEvent struct {
	EventType Type
}

func (e *BaseEvent) Type() Type { return e.EventType }

// CancellableEvent embeds BaseEvent and adds cancel support.
type CancellableEvent struct {
	BaseEvent
	cancelled bool
}

func (e *CancellableEvent) IsCancelled() bool   { return e.cancelled }
func (e *CancellableEvent) SetCancelled(v bool) { e.cancelled = v }
