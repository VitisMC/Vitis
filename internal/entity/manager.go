package entity

const defaultEntityMapCapacity = 256

// Manager is a per-world entity manager owned by the world tick goroutine.
// All methods must be called exclusively from the world tick goroutine.
type Manager struct {
	entities      map[int32]*Entity
	chunkEntities map[int64][]int32
	nextID        int32
	removeScratch []int32
}

// NewManager creates a new entity manager.
func NewManager() *Manager {
	return &Manager{
		entities:      make(map[int32]*Entity, defaultEntityMapCapacity),
		chunkEntities: make(map[int64][]int32, defaultEntityMapCapacity),
	}
}

// AllocateID returns the next entity ID.
func (m *Manager) AllocateID() int32 {
	m.nextID++
	return m.nextID
}

// Add inserts an entity into the manager and the spatial index.
func (m *Manager) Add(e *Entity) {
	if e == nil {
		return
	}
	m.entities[e.id] = e
	key := ChunkKey(e.chunkX, e.chunkZ)
	m.chunkEntities[key] = append(m.chunkEntities[key], e.id)
}

// Remove marks an entity for deferred removal.
func (m *Manager) Remove(id int32) {
	e, ok := m.entities[id]
	if !ok {
		return
	}
	e.Remove()
}

// Get returns an entity by ID.
func (m *Manager) Get(id int32) *Entity {
	return m.entities[id]
}

// Count returns the number of live entities.
func (m *Manager) Count() int {
	return len(m.entities)
}

// EntitiesInChunk returns entity IDs currently in the given chunk.
func (m *Manager) EntitiesInChunk(chunkKey int64) []int32 {
	return m.chunkEntities[chunkKey]
}

// Entities returns the full entity map for iteration.
func (m *Manager) Entities() map[int32]*Entity {
	return m.entities
}

// Tick advances the entity manager for one world tick.
// It updates chunk membership for moved entities, snapshots previous state,
// and processes deferred removals.
func (m *Manager) Tick(_ uint64) {
	m.removeScratch = m.removeScratch[:0]

	for id, e := range m.entities {
		if e.removed {
			m.removeScratch = append(m.removeScratch, id)
			continue
		}

		if e.dirty&DirtyPosition != 0 {
			m.updateChunkMembership(e)
		}
	}

	for _, id := range m.removeScratch {
		m.removeImmediate(id)
	}
}

// SnapshotAll stores current state as previous-tick state and clears dirty flags for all entities.
func (m *Manager) SnapshotAll() {
	for _, e := range m.entities {
		e.SnapshotPrev()
		e.ClearDirty()
	}
}

func (m *Manager) updateChunkMembership(e *Entity) {
	oldKey := ChunkKey(e.chunkX, e.chunkZ)
	if !e.UpdateChunkCoords() {
		return
	}
	newKey := ChunkKey(e.chunkX, e.chunkZ)

	ids := m.chunkEntities[oldKey]
	for i, eid := range ids {
		if eid == e.id {
			ids[i] = ids[len(ids)-1]
			ids = ids[:len(ids)-1]
			break
		}
	}
	if len(ids) == 0 {
		delete(m.chunkEntities, oldKey)
	} else {
		m.chunkEntities[oldKey] = ids
	}

	m.chunkEntities[newKey] = append(m.chunkEntities[newKey], e.id)
}

func (m *Manager) removeImmediate(id int32) {
	e, ok := m.entities[id]
	if !ok {
		return
	}

	key := ChunkKey(e.chunkX, e.chunkZ)
	ids := m.chunkEntities[key]
	for i, eid := range ids {
		if eid == id {
			ids[i] = ids[len(ids)-1]
			ids = ids[:len(ids)-1]
			break
		}
	}
	if len(ids) == 0 {
		delete(m.chunkEntities, key)
	} else {
		m.chunkEntities[key] = ids
	}

	delete(m.entities, id)
}
