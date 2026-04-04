package entity

// Tracker orchestrates per-tick entity tracking for all players in a world.
// It must be called exclusively from the world tick goroutine.
type Tracker struct {
	players []*Player
}

// NewTracker creates a new entity tracker.
func NewTracker() *Tracker {
	return &Tracker{}
}

// AddPlayer registers a player for tracking updates.
func (t *Tracker) AddPlayer(p *Player) {
	if p == nil {
		return
	}
	t.players = append(t.players, p)
}

// RemovePlayer unregisters a player from tracking updates.
func (t *Tracker) RemovePlayer(id int32) {
	for i, p := range t.players {
		if p.ID() == id {
			t.players[i] = t.players[len(t.players)-1]
			t.players[len(t.players)-1] = nil
			t.players = t.players[:len(t.players)-1]
			return
		}
	}
}

// Players returns the current player list.
func (t *Tracker) Players() []*Player {
	return t.players
}

// Tick runs one tracking cycle for all registered players.
// For each player it computes visible entities, sends spawn/despawn packets
// for entities entering/leaving range, and movement updates for dirty entities.
func (t *Tracker) Tick(mgr *Manager) {
	if mgr == nil {
		return
	}

	for _, p := range t.players {
		if p.Removed() {
			continue
		}

		spawns, despawns := p.UpdateTracking(mgr)

		spawnSet := make(map[int32]struct{}, len(spawns))
		for _, eid := range spawns {
			e := mgr.Get(eid)
			if e != nil && !e.Removed() {
				p.SendSpawnEntity(e)
				spawnSet[eid] = struct{}{}
			}
		}

		if len(despawns) > 0 {
			p.SendDespawnEntities(despawns)
		}

		for eid := range p.TrackedEntities() {
			if _, justSpawned := spawnSet[eid]; justSpawned {
				continue
			}
			e := mgr.Get(eid)
			if e == nil || e.Removed() {
				continue
			}
			if e.ClientSimulated() && e.ID() == p.ID() {
				continue
			}
			if e.Dirty() != 0 {
				p.SendMovementUpdate(e)
			}
		}
	}

	mgr.SnapshotAll()
}
