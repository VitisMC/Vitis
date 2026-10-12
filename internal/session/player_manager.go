package session

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/vitismc/vitis/internal/advancement"
	"github.com/vitismc/vitis/internal/crafting"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// ErrPlayerLimitReached indicates that the authoritative player admission limit
// has been reached.
var ErrPlayerLimitReached = errors.New("player limit reached")

// OnlinePlayer holds info about a connected player for tab list and visibility.
type OnlinePlayer struct {
	Session                Session
	EntityID               int32
	UUID                   protocol.UUID
	Name                   string
	GameMode               int32
	SpeedBlocksPerSecond   float64
	Properties             []playpacket.PlayerProperty
	X, Y, Z                float64
	Yaw, Pitch             float32
	Windows                *inventory.WindowManager
	RecipeBook             *crafting.RecipeBook
	RecipeProgress         advancement.RecipeProgress
	RecipeCatalog          *crafting.Catalog
	RecipeInventoryState   int32
	RecipeEnteredBlock     string
	Flags                  byte
	Pose                   int32
	Sleeping               bool
	SleepBedState          int32
	SleepTicks             int
	SleepX, SleepY, SleepZ int
	SpawnDimension         string
	SpawnAngle             float32
	SpawnForced            bool
	RespawnPending         bool
	RespawnRetryTicks      int
	DeathProcessed         bool
	DeathKeepInventory     bool
	ManagedTracking        bool
	PortalCooldown         int32
	portalTime             int32
	portalKind             string
	portalBinding          *worldBinding
	SpawnX, SpawnY, SpawnZ int
	HasSpawnPoint          bool
	movement               movementState
}

// PlayerManager tracks online players and handles tab list broadcasts.
type PlayerManager struct {
	parent     *PlayerManager
	dimension  string
	mu         sync.RWMutex
	players    map[protocol.UUID]*OnlinePlayer
	maxPlayers atomic.Int64
}

// NewPlayerManager creates a new player manager.
func NewPlayerManager() *PlayerManager {
	pm := &PlayerManager{
		players: make(map[protocol.UUID]*OnlinePlayer, 64),
	}
	pm.Global().maxPlayers.Store(200)
	return pm
}

// SetMaxPlayers updates the admission limit for future play sessions.
func (pm *PlayerManager) SetMaxPlayers(maxPlayers int) {
	if pm == nil || maxPlayers <= 0 {
		return
	}
	pm.Global().maxPlayers.Store(int64(maxPlayers))
}

// MaxPlayers returns the configured player admission limit.
func (pm *PlayerManager) MaxPlayers() int {
	if pm == nil {
		return 0
	}
	return int(pm.Global().maxPlayers.Load())
}

// AddPlayer registers a new player and broadcasts their info to all existing players.
// Also sends all existing players' info to the new player.
func (pm *PlayerManager) AddPlayer(p *OnlinePlayer) error {
	if pm == nil || p == nil || p.Session == nil || p.UUID == (protocol.UUID{}) {
		return fmt.Errorf("add player: invalid player")
	}
	if p.RecipeBook == nil {
		p.RecipeBook = crafting.NewRecipeBook()
	}
	pm.Global().mu.Lock()
	defer pm.Global().mu.Unlock()
	if existing := pm.Global().players[p.UUID]; existing != nil {
		return fmt.Errorf("%w: uuid=%s session=%d", ErrPlayerAlreadyOnline, protocol.UUIDToString(p.UUID), existing.Session.ID())
	}
	if int64(len(pm.Global().players)) >= pm.Global().maxPlayers.Load() {
		return fmt.Errorf("%w: max=%d", ErrPlayerLimitReached, pm.Global().maxPlayers.Load())
	}

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

	existingEntries := make([]playpacket.PlayerInfoEntry, 0, len(pm.Global().players))
	for _, existing := range pm.Global().players {
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

	newEquipPkt := BuildFullEquipmentPacket(p.EntityID, p.Windows)

	for _, existing := range pm.Global().players {
		_ = existing.Session.Send(newPlayerInfo)
		if SessionDimension(existing.Session) != SessionDimension(p.Session) || p.ManagedTracking || existing.ManagedTracking {
			continue
		}
		_ = existing.Session.Send(newSpawnPkt)
		if newEquipPkt != nil {
			_ = existing.Session.Send(newEquipPkt)
		}

		existingSpawn := &playpacket.SpawnEntity{
			EntityID:   existing.EntityID,
			EntityUUID: existing.UUID,
			Type:       147,
			X:          existing.X,
			Y:          existing.Y,
			Z:          existing.Z,
		}
		_ = p.Session.Send(existingSpawn)
		if existingEquip := BuildFullEquipmentPacket(existing.EntityID, existing.Windows); existingEquip != nil {
			_ = p.Session.Send(existingEquip)
		}
	}

	pm.Global().players[p.UUID] = p
	return nil
}

// RemovePlayer removes a player and broadcasts their removal to all remaining players.
func (pm *PlayerManager) RemovePlayer(uuid protocol.UUID, owner Session) bool {
	pm.Global().mu.Lock()
	defer pm.Global().mu.Unlock()

	removed, ok := pm.Global().players[uuid]
	if !ok || removed == nil || (owner != nil && removed.Session != owner) {
		return false
	}
	delete(pm.Global().players, uuid)

	removePkt := &playpacket.PlayerInfoRemove{
		UUIDs: []protocol.UUID{uuid},
	}
	for _, existing := range pm.Global().players {
		_ = existing.Session.Send(removePkt)
		if SessionDimension(existing.Session) == SessionDimension(removed.Session) && !removed.ManagedTracking {
			_ = existing.Session.Send(&playpacket.RemoveEntities{EntityIDs: []int32{removed.EntityID}})
		}
	}
	return true
}

// GetBySession returns the OnlinePlayer associated with the given session, or nil.
func (pm *PlayerManager) GetBySession(s Session) *OnlinePlayer {
	if s == nil {
		return nil
	}
	pm.Global().mu.RLock()
	defer pm.Global().mu.RUnlock()
	for _, p := range pm.Global().players {
		if p.Session == s && pm.contains(p) {
			return p
		}
	}
	return nil
}

// Count returns the number of online players.
func (pm *PlayerManager) Count() int {
	pm.Global().mu.RLock()
	defer pm.Global().mu.RUnlock()
	count := 0
	for _, p := range pm.Global().players {
		if pm.contains(p) {
			count++
		}
	}
	return count
}

// Broadcast sends a packet to all online players.
func (pm *PlayerManager) Broadcast(pkt protocol.Packet) {
	pm.Global().mu.RLock()
	defer pm.Global().mu.RUnlock()
	for _, p := range pm.Global().players {
		if !pm.contains(p) {
			continue
		}
		_ = p.Session.Send(pkt)
	}
}

// BroadcastExcept sends a packet to all online players except the one with the given UUID.
func (pm *PlayerManager) BroadcastExcept(uuid protocol.UUID, pkt protocol.Packet) {
	pm.Global().mu.RLock()
	defer pm.Global().mu.RUnlock()
	for _, p := range pm.Global().players {
		if !pm.contains(p) {
			continue
		}
		if p.UUID != uuid {
			_ = p.Session.Send(pkt)
		}
	}
}

// GetByUUID returns the OnlinePlayer with the given UUID, or nil.
func (pm *PlayerManager) GetByUUID(uuid protocol.UUID) *OnlinePlayer {
	pm.Global().mu.RLock()
	defer pm.Global().mu.RUnlock()
	p := pm.Global().players[uuid]
	if !pm.contains(p) {
		return nil
	}
	return p
}

// OnlinePlayerNames returns a slice of all online player names.
func (pm *PlayerManager) OnlinePlayerNames() []string {
	pm.Global().mu.RLock()
	defer pm.Global().mu.RUnlock()
	names := make([]string, 0, len(pm.Global().players))
	for _, p := range pm.Global().players {
		if !pm.contains(p) {
			continue
		}
		names = append(names, p.Name)
	}
	return names
}

// Players returns a snapshot slice of all online players.
func (pm *PlayerManager) Players() []*OnlinePlayer {
	pm.Global().mu.RLock()
	defer pm.Global().mu.RUnlock()
	out := make([]*OnlinePlayer, 0, len(pm.Global().players))
	for _, p := range pm.Global().players {
		if !pm.contains(p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// AllSleeping returns true if every online player is currently sleeping.
// Returns false if there are no players.
func (pm *PlayerManager) AllSleeping() bool {
	pm.Global().mu.RLock()
	defer pm.Global().mu.RUnlock()
	eligible := 0
	for _, p := range pm.Global().players {
		if !pm.contains(p) {
			continue
		}
		if p.GameMode == 3 {
			continue
		}
		eligible++
		if !p.Sleeping {
			return false
		}
	}
	return eligible > 0
}

// GetByEntityID returns the OnlinePlayer with the given entity ID, or nil.
func (pm *PlayerManager) GetByEntityID(entityID int32) *OnlinePlayer {
	pm.Global().mu.RLock()
	defer pm.Global().mu.RUnlock()
	for _, p := range pm.Global().players {
		if !pm.contains(p) {
			continue
		}
		if p.EntityID == entityID && pm.contains(p) {
			return p
		}
	}
	return nil
}
