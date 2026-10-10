package entity

import "sync/atomic"

// Tracker orchestrates per-tick entity tracking for all players in a world.
// It must be called exclusively from the world tick goroutine.
type Tracker struct {
	players    []*Player
	spawnState func(*Player, *Entity)
	count      atomic.Int32
}

// NewTracker creates a new entity tracker.
func NewTracker() *Tracker {
	return &Tracker{}
}

// SetSpawnStateHandler synchronizes specialized state when an entity enters a player's view.
func (t *Tracker) SetSpawnStateHandler(handler func(*Player, *Entity)) { t.spawnState = handler }

// AddPlayer registers a player for tracking updates.
func (t *Tracker) AddPlayer(p *Player) {
	if p == nil {
		return
	}
	t.players = append(t.players, p)
	t.count.Store(int32(len(t.players)))
}

// RemovePlayer unregisters a player from tracking updates.
func (t *Tracker) RemovePlayer(id int32) {
	for i, p := range t.players {
		if p.ID() == id {
			t.players[i] = t.players[len(t.players)-1]
			t.players[len(t.players)-1] = nil
			t.players = t.players[:len(t.players)-1]
			t.count.Store(int32(len(t.players)))
			return
		}
	}
}

// Players returns the current player list.
func (t *Tracker) Players() []*Player {
	return t.players
}

// Count reports membership safely to background lifecycle and monitoring workers.
func (t *Tracker) Count() int {
	if t == nil {
		return 0
	}
	return int(t.count.Load())
}

// Tick runs one tracking cycle for all registered players.
// For each player it computes visible entities, sends spawn/despawn packets
// for entities entering/leaving range, and movement updates for dirty entities.
func (t *Tracker) Tick(mgr *Manager) {
	t.TickWithChunkFilter(mgr, nil)
}

// TickWithChunkFilter updates tracking only in chunks already visible to each client.
func (t *Tracker) TickWithChunkFilter(mgr *Manager, visible func(playerID, chunkX, chunkZ int32) bool) {
	if mgr == nil {
		return
	}

	for _, p := range t.players {
		if p.Removed() {
			continue
		}

		var playerFilter func(chunkX, chunkZ int32) bool
		if visible != nil {
			playerID := p.ID()
			playerFilter = func(chunkX, chunkZ int32) bool {
				return visible(playerID, chunkX, chunkZ)
			}
		}
		spawns, despawns := p.updateTracking(mgr, playerFilter)

		spawnSet := make(map[int32]struct{}, len(spawns))
		for _, eid := range spawns {
			e := mgr.Get(eid)
			if e != nil && !e.Removed() {
				p.SendSpawnEntity(e)
				if t.spawnState != nil {
					t.spawnState(p, e)
				}
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
