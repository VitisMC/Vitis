package projectile

// Manager manages all projectile entities in a world.
type Manager struct {
	arrows map[int32]*Arrow
	thrown map[int32]*ThrownEntity
}

// NewManager creates a new projectile manager.
func NewManager() *Manager {
	return &Manager{
		arrows: make(map[int32]*Arrow, 32),
		thrown: make(map[int32]*ThrownEntity, 32),
	}
}

// AddArrow registers an arrow.
func (m *Manager) AddArrow(a *Arrow) {
	if m == nil || a == nil {
		return
	}
	m.arrows[a.Entity.ID()] = a
}

// AddThrown registers a thrown entity.
func (m *Manager) AddThrown(t *ThrownEntity) {
	if m == nil || t == nil {
		return
	}
	m.thrown[t.Entity.ID()] = t
}

// RemoveArrow removes an arrow by ID.
func (m *Manager) RemoveArrow(id int32) {
	if m == nil {
		return
	}
	delete(m.arrows, id)
}

// RemoveThrown removes a thrown entity by ID.
func (m *Manager) RemoveThrown(id int32) {
	if m == nil {
		return
	}
	delete(m.thrown, id)
}

// GetArrow returns an arrow by ID.
func (m *Manager) GetArrow(id int32) *Arrow {
	if m == nil {
		return nil
	}
	return m.arrows[id]
}

// GetThrown returns a thrown entity by ID.
func (m *Manager) GetThrown(id int32) *ThrownEntity {
	if m == nil {
		return nil
	}
	return m.thrown[id]
}

// Arrows returns all arrows.
func (m *Manager) Arrows() map[int32]*Arrow {
	if m == nil {
		return nil
	}
	return m.arrows
}

// Thrown returns all thrown entities.
func (m *Manager) Thrown() map[int32]*ThrownEntity {
	if m == nil {
		return nil
	}
	return m.thrown
}

// ArrowCount returns the number of arrows.
func (m *Manager) ArrowCount() int {
	if m == nil {
		return 0
	}
	return len(m.arrows)
}

// ThrownCount returns the number of thrown entities.
func (m *Manager) ThrownCount() int {
	if m == nil {
		return 0
	}
	return len(m.thrown)
}

// TickResult holds the IDs of projectiles that hit something or were removed.
type TickResult struct {
	ArrowHits   []ArrowHitEvent
	ThrownHits  []ThrownHitEvent
	RemovedIDs  []int32
}

// ArrowHitEvent describes an arrow that hit a block or entity.
type ArrowHitEvent struct {
	ArrowID   int32
	HitResult *HitResult
}

// ThrownHitEvent describes a thrown entity that hit a block or entity.
type ThrownHitEvent struct {
	ThrownID   int32
	ThrownType ThrownType
	HitResult  *HitResult
}

// TickAll advances all projectiles and returns hit events and removed IDs.
func (m *Manager) TickAll(world BlockAccessProvider, finder EntityFinder) TickResult {
	var result TickResult

	for id, arrow := range m.arrows {
		arrow.Tick(world, finder)

		if arrow.Entity.Removed() {
			result.RemovedIDs = append(result.RemovedIDs, id)
			continue
		}

		if hr := arrow.LastHitResult(); hr != nil {
			result.ArrowHits = append(result.ArrowHits, ArrowHitEvent{
				ArrowID:   id,
				HitResult: hr,
			})
		}
	}

	for id, t := range m.thrown {
		t.Tick(world, finder)

		if t.Entity.Removed() {
			result.RemovedIDs = append(result.RemovedIDs, id)
			continue
		}

		if hr := t.LastHitResult(); hr != nil {
			result.ThrownHits = append(result.ThrownHits, ThrownHitEvent{
				ThrownID:   id,
				ThrownType: t.ThrownType(),
				HitResult:  hr,
			})
			t.Entity.Remove()
			result.RemovedIDs = append(result.RemovedIDs, id)
		}
	}

	for _, id := range result.RemovedIDs {
		delete(m.arrows, id)
		delete(m.thrown, id)
	}

	return result
}

// BlockAccessProvider is the interface for accessing block states (same as physics.BlockAccess).
type BlockAccessProvider interface {
	GetBlockStateAt(x, y, z int) int32
}
