package mining

import "sync"

// DigState tracks the progress of a player's block-breaking action.
type DigState struct {
	X, Y, Z       int
	StateID        int32
	TotalTicks     int
	ElapsedTicks   int
	LastStage      int8
	CanHarvest     bool
}

// Stage returns the current destroy stage (0-9) based on elapsed progress.
func (d *DigState) Stage() int8 {
	if d.TotalTicks <= 0 {
		return 9
	}
	progress := float64(d.ElapsedTicks) / float64(d.TotalTicks)
	stage := int8(progress * 10.0)
	if stage > 9 {
		stage = 9
	}
	return stage
}

// Done returns true if the block should be broken.
func (d *DigState) Done() bool {
	return d.ElapsedTicks >= d.TotalTicks
}

// Tracker manages active digging sessions per player entity ID.
type Tracker struct {
	mu       sync.Mutex
	sessions map[int32]*DigState
}

// NewTracker creates a new mining progress tracker.
func NewTracker() *Tracker {
	return &Tracker{
		sessions: make(map[int32]*DigState),
	}
}

// Start begins tracking a dig action for a player.
func (t *Tracker) Start(entityID int32, x, y, z int, stateID int32, totalTicks int, canHarvest bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sessions[entityID] = &DigState{
		X: x, Y: y, Z: z,
		StateID:    stateID,
		TotalTicks: totalTicks,
		CanHarvest: canHarvest,
		LastStage:  -1,
	}
}

// Cancel removes the dig session for a player.
func (t *Tracker) Cancel(entityID int32) *DigState {
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.sessions[entityID]
	delete(t.sessions, entityID)
	return state
}

// Get returns the current dig state for a player, or nil.
func (t *Tracker) Get(entityID int32) *DigState {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.sessions[entityID]
}

// Tick advances all active dig sessions by one tick.
// Returns a slice of entity IDs whose destroy stage changed this tick.
func (t *Tracker) Tick() []int32 {
	t.mu.Lock()
	defer t.mu.Unlock()

	var changed []int32
	for eid, state := range t.sessions {
		state.ElapsedTicks++
		newStage := state.Stage()
		if newStage != state.LastStage {
			state.LastStage = newStage
			changed = append(changed, eid)
		}
	}
	return changed
}
