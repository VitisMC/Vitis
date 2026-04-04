package session

import (
	"sync"

	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// OnlinePlayer holds info about a connected player for tab list and visibility.
type OnlinePlayer struct {
	Session    Session
	EntityID   int32
	UUID       protocol.UUID
	Name       string
	GameMode   int32
	Properties []playpacket.PlayerProperty
	X, Y, Z    float64
	Yaw, Pitch float32
	Windows    *inventory.WindowManager
	Flags      byte
	Pose       int32
}

// PlayerManager tracks online players and handles tab list broadcasts.
type PlayerManager struct {
	mu      sync.RWMutex
	players map[protocol.UUID]*OnlinePlayer
}

// NewPlayerManager creates a new player manager.
func NewPlayerManager() *PlayerManager {
	return &PlayerManager{
		players: make(map[protocol.UUID]*OnlinePlayer, 64),
	}
}

// AddPlayer registers a new player and broadcasts their info to all existing players.
// Also sends all existing players' info to the new player.
func (pm *PlayerManager) AddPlayer(p *OnlinePlayer) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	newPlayerInfo := &playpacket.PlayerInfoUpdate{
		Actions: playpacket.ActionAddPlayer | playpacket.ActionUpdateListed | playpacket.ActionUpdateGameMode | playpacket.ActionUpdateLatency,
		Entries: []playpacket.PlayerInfoEntry{
			{
				UUID:       p.UUID,
				Name:       p.Name,
				Properties: p.Properties,
				GameMode:   p.GameMode,
				Listed:     true,
				Ping:       0,
			},
		},
	}

	newSpawnPkt := &playpacket.SpawnEntity{
		EntityID:   p.EntityID,
		EntityUUID: p.UUID,
		Type:       147,
		X:          p.X,
		Y:          p.Y,
		Z:          p.Z,
	}

	existingEntries := make([]playpacket.PlayerInfoEntry, 0, len(pm.players))
	for _, existing := range pm.players {
		existingEntries = append(existingEntries, playpacket.PlayerInfoEntry{
			UUID:       existing.UUID,
			Name:       existing.Name,
			Properties: existing.Properties,
			GameMode:   existing.GameMode,
			Listed:     true,
			Ping:       0,
		})
	}

	if len(existingEntries) > 0 {
		allPlayersInfo := &playpacket.PlayerInfoUpdate{
			Actions: playpacket.ActionAddPlayer | playpacket.ActionUpdateListed | playpacket.ActionUpdateGameMode | playpacket.ActionUpdateLatency,
			Entries: existingEntries,
		}
		_ = p.Session.Send(allPlayersInfo)
	}

	for _, existing := range pm.players {
		_ = existing.Session.Send(newPlayerInfo)
		_ = existing.Session.Send(newSpawnPkt)

		existingSpawn := &playpacket.SpawnEntity{
			EntityID:   existing.EntityID,
			EntityUUID: existing.UUID,
			Type:       147,
			X:          existing.X,
			Y:          existing.Y,
			Z:          existing.Z,
		}
		_ = p.Session.Send(existingSpawn)
	}

	pm.players[p.UUID] = p
}

// RemovePlayer removes a player and broadcasts their removal to all remaining players.
func (pm *PlayerManager) RemovePlayer(uuid protocol.UUID) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	removed, ok := pm.players[uuid]
	delete(pm.players, uuid)

	removePkt := &playpacket.PlayerInfoRemove{
		UUIDs: []protocol.UUID{uuid},
	}
	for _, existing := range pm.players {
		_ = existing.Session.Send(removePkt)
		if ok {
			_ = existing.Session.Send(&playpacket.RemoveEntities{EntityIDs: []int32{removed.EntityID}})
		}
	}
}

// GetBySession returns the OnlinePlayer associated with the given session, or nil.
func (pm *PlayerManager) GetBySession(s Session) *OnlinePlayer {
	if s == nil {
		return nil
	}
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	for _, p := range pm.players {
		if p.Session == s {
			return p
		}
	}
	return nil
}

// Count returns the number of online players.
func (pm *PlayerManager) Count() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.players)
}

// Broadcast sends a packet to all online players.
func (pm *PlayerManager) Broadcast(pkt protocol.Packet) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	for _, p := range pm.players {
		_ = p.Session.Send(pkt)
	}
}

// BroadcastExcept sends a packet to all online players except the one with the given UUID.
func (pm *PlayerManager) BroadcastExcept(uuid protocol.UUID, pkt protocol.Packet) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	for _, p := range pm.players {
		if p.UUID != uuid {
			_ = p.Session.Send(pkt)
		}
	}
}

// GetByUUID returns the OnlinePlayer with the given UUID, or nil.
func (pm *PlayerManager) GetByUUID(uuid protocol.UUID) *OnlinePlayer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.players[uuid]
}

// OnlinePlayerNames returns a slice of all online player names.
func (pm *PlayerManager) OnlinePlayerNames() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	names := make([]string, 0, len(pm.players))
	for _, p := range pm.players {
		names = append(names, p.Name)
	}
	return names
}

// GetByEntityID returns the OnlinePlayer with the given entity ID, or nil.
func (pm *PlayerManager) GetByEntityID(entityID int32) *OnlinePlayer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	for _, p := range pm.players {
		if p.EntityID == entityID {
			return p
		}
	}
	return nil
}
