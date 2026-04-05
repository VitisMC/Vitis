package food

import "sync"

// EatingState tracks an active eating action for a player.
type EatingState struct {
	ItemName     string
	ItemID       int32
	Hand         int32
	TotalTicks   int32
	ElapsedTicks int32
	Nutrition    int32
	Saturation   float32
}

// Done returns true when the eating animation is complete.
func (e *EatingState) Done() bool {
	return e.ElapsedTicks >= e.TotalTicks
}

// EatingTracker manages per-player eating state.
type EatingTracker struct {
	mu    sync.Mutex
	state map[int32]*EatingState
}

// NewEatingTracker creates a new eating tracker.
func NewEatingTracker() *EatingTracker {
	return &EatingTracker{
		state: make(map[int32]*EatingState),
	}
}

// Start begins tracking an eating action for a player entity.
func (t *EatingTracker) Start(entityID int32, itemName string, itemID, hand int32, props *Properties) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state[entityID] = &EatingState{
		ItemName:   itemName,
		ItemID:     itemID,
		Hand:       hand,
		TotalTicks: props.EatDuration,
		Nutrition:  props.Nutrition,
		Saturation: props.Saturation,
	}
}

// Cancel removes eating state for a player, returning the state if it existed.
func (t *EatingTracker) Cancel(entityID int32) *EatingState {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := t.state[entityID]
	delete(t.state, entityID)
	return s
}

// Get returns the current eating state for a player, or nil.
func (t *EatingTracker) Get(entityID int32) *EatingState {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state[entityID]
}

// Tick advances all eating states by one tick.
// Returns entity IDs of players who finished eating this tick.
// Finished states are removed automatically.
func (t *EatingTracker) Tick() []int32 {
	t.mu.Lock()
	defer t.mu.Unlock()

	var finished []int32
	for eid, s := range t.state {
		s.ElapsedTicks++
		if s.Done() {
			finished = append(finished, eid)
			delete(t.state, eid)
		}
	}
	return finished
}
