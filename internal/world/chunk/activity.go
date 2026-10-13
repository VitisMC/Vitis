package chunk

import (
	"fmt"
	"sort"
)

// Activity describes which runtime operations a chunk's tickets permit.
type Activity uint8

const (
	ActivityUnloaded Activity = iota
	ActivityLoaded
	ActivityBlockTicking
	ActivityEntityTicking
)

// TicketKind identifies the owner of a chunk's residency requirement.
type TicketKind uint8

const (
	TicketView TicketKind = iota
	TicketPlayer
	TicketSpawn
	TicketPortal
	TicketForced
)

// Ticket pins a square of chunks; ticking tickets include two dependency rings.
// ExpiresAt is an absolute world tick; zero means no expiry.
type Ticket struct {
	Kind      TicketKind
	Owner     uint64
	X, Z      int32
	Radius    int32
	ExpiresAt uint64
}

// ticketKey separates independent residency reasons for the same owner.
type ticketKey struct {
	kind  TicketKind
	owner uint64
}

// ActivityTracker owns residency requirements on the world tick goroutine.
type ActivityTracker struct {
	tickets  map[ticketKey]Ticket
	activity map[int64]Activity
	dirty    bool
	required []int64
	cursor   int
}

// NewActivityTracker creates an empty ticket collection.
func NewActivityTracker() *ActivityTracker {
	return &ActivityTracker{tickets: make(map[ticketKey]Ticket), activity: make(map[int64]Activity)}
}

// Set inserts or updates one bounded residency requirement.
func (a *ActivityTracker) Set(t Ticket) error {
	if a == nil || t.Kind > TicketForced || t.Radius < 0 || t.Radius > 96 || t.X < -1875000 || t.X > 1874999 || t.Z < -1875000 || t.Z > 1874999 {
		return fmt.Errorf("invalid chunk ticket")
	}
	k := ticketKey{t.Kind, t.Owner}
	if previous, ok := a.tickets[k]; !ok || previous != t {
		a.tickets[k] = t
		a.dirty = true
	}
	return nil
}

// Remove releases only the specified owner's residency reason.
func (a *ActivityTracker) Remove(kind TicketKind, owner uint64) {
	if a == nil {
		return
	}
	k := ticketKey{kind, owner}
	if _, ok := a.tickets[k]; ok {
		delete(a.tickets, k)
		a.dirty = true
	}
}

// Update expires timed tickets and rebuilds membership only when it changes.
func (a *ActivityTracker) Update(now uint64) {
	if a == nil {
		return
	}
	for k, t := range a.tickets {
		if t.ExpiresAt != 0 && t.ExpiresAt <= now {
			delete(a.tickets, k)
			a.dirty = true
		}
	}
	if !a.dirty {
		return
	}
	clear(a.activity)
	for _, t := range a.tickets {
		radius := t.Radius
		if t.Kind != TicketView {
			radius += 2
		}
		for dz := -radius; dz <= radius; dz++ {
			for dx := -radius; dx <= radius; dx++ {
				state := ActivityLoaded
				if t.Kind != TicketView {
					distance := max(absTicketOffset(dx), absTicketOffset(dz))
					if distance <= t.Radius {
						state = ActivityEntityTicking
					} else if distance == t.Radius+1 {
						state = ActivityBlockTicking
					}
				}
				key := ChunkKey(t.X+dx, t.Z+dz)
				if state > a.activity[key] {
					a.activity[key] = state
				}
			}
		}
	}
	a.dirty = false
	a.required = a.required[:0]
	for key := range a.activity {
		a.required = append(a.required, key)
	}
	sort.Slice(a.required, func(i, j int) bool {
		if a.activity[a.required[i]] != a.activity[a.required[j]] {
			return a.activity[a.required[i]] > a.activity[a.required[j]]
		}
		return a.required[i] < a.required[j]
	})
	a.cursor = 0
}

// VisitRequired traverses residency requests in bounded batches with a persistent cursor.
func (a *ActivityTracker) VisitRequired(limit int, visit func(int32, int32)) {
	if a == nil || visit == nil || limit <= 0 || len(a.required) == 0 {
		return
	}
	for i := 0; i < min(limit, len(a.required)); i++ {
		key := a.required[a.cursor]
		visit(int32(uint64(key)>>32), int32(key))
		a.cursor = (a.cursor + 1) % len(a.required)
	}
}

// At returns ticket activity independently of whether disk loading has completed.
func (a *ActivityTracker) At(x, z int32) Activity {
	if a == nil {
		return ActivityUnloaded
	}
	return a.activity[ChunkKey(x, z)]
}

// absTicketOffset returns an absolute bounded chunk offset.
func absTicketOffset(value int32) int32 {
	if value < 0 {
		return -value
	}
	return value
}
