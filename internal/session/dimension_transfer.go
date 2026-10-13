package session

import (
	"fmt"
	"math"

	pose "github.com/vitismc/vitis/internal/data/generated/entity_pose"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world"
	"github.com/vitismc/vitis/internal/world/chunk"
)

const transferTimeoutTicks = 1200
const maxPreparingTransfers = 16

// DimensionTransfers owns prepared world transitions on the common tick loop.
type DimensionTransfers struct {
	players    *PlayerManager
	manager    *world.Manager
	worlds     map[string]PlayWorld
	pending    map[protocol.UUID]*dimensionTransfer
	tick       uint64
	portalWork int
}

// dimensionTransfer retains origin membership until its destination is ready.
type dimensionTransfer struct {
	session       *DefaultSession
	online        *OnlinePlayer
	origin        *world.World
	destination   *world.World
	binding       *worldBinding
	position      entity.Vec3
	rotation      entity.Vec2
	started       uint64
	respawn       bool
	useSpawnPoint bool
	portal        *portalPreparation
}

// NewDimensionTransfers validates the prepared dimension runtime adapters.
func NewDimensionTransfers(players *PlayerManager, manager *world.Manager, worlds []PlayWorld) (*DimensionTransfers, error) {
	if players == nil || manager == nil || len(worlds) == 0 {
		return nil, fmt.Errorf("dimension transfers: missing runtime")
	}
	t := &DimensionTransfers{players: players.Global(), manager: manager, worlds: make(map[string]PlayWorld), pending: make(map[protocol.UUID]*dimensionTransfer)}
	for _, entry := range worlds {
		if _, ok := manager.GetByDimension(entry.Dimension); !ok || entry.Accessor == nil || t.worlds[entry.Dimension].Accessor != nil {
			return nil, fmt.Errorf("dimension transfers: invalid dimension %q", entry.Dimension)
		}
		t.worlds[entry.Dimension] = entry
	}
	return t, nil
}

// Request starts a transfer without changing the player's origin world.
// This method and Tick must only be called on the common world tick goroutine.
func (t *DimensionTransfers) Request(s Session, dimension string, position entity.Vec3, rotation entity.Vec2) error {
	return t.request(s, dimension, position, rotation, false)
}

// RequestRespawn prepares the personal spawn or the Overworld's default spawn.
func (t *DimensionTransfers) RequestRespawn(s Session) error {
	return t.requestPersonalSpawn(s, true)
}

// requestPersonalSpawn resolves a personal spawn for death or living End return.
func (t *DimensionTransfers) requestPersonalSpawn(s Session, respawn bool) error {
	if s == nil {
		return fmt.Errorf("respawn: missing session")
	}
	online := t.players.GetBySession(s)
	player, ok := s.Player().(*entity.Player)
	if online == nil || !ok || player == nil || (respawn && (!player.Living().IsDead() || !player.Living().DeathHandled())) {
		return fmt.Errorf("respawn: player is not ready")
	}
	if t.pending[online.UUID] != nil {
		return nil
	}
	dimension := "minecraft:overworld"
	position, rotation := entity.Vec3{}, entity.Vec2{}
	if online.HasSpawnPoint {
		dimension = online.SpawnDimension
		if dimension == "" {
			dimension = "minecraft:overworld"
		}
		if t.worlds[dimension].Accessor != nil {
			position = entity.Vec3{X: float64(online.SpawnX), Y: float64(online.SpawnY), Z: float64(online.SpawnZ)}
			rotation.X = online.SpawnAngle
		} else {
			online.HasSpawnPoint = false
			dimension = "minecraft:overworld"
		}
	}
	if !online.HasSpawnPoint {
		entry := t.worlds[dimension]
		if entry.Config.ReadWorldSpawn == nil {
			return fmt.Errorf("respawn: default spawn unavailable")
		}
		x, y, z, angle := entry.Config.ReadWorldSpawn()
		position, rotation.X = entity.Vec3{X: float64(x) + .5, Y: float64(y), Z: float64(z) + .5}, angle
	}
	if err := t.request(s, dimension, position, rotation, respawn); err != nil {
		return err
	}
	t.pending[online.UUID].useSpawnPoint = true
	return nil
}

// request validates ownership and acquires a bounded destination residency ticket.
func (t *DimensionTransfers) request(s Session, dimension string, position entity.Vec3, rotation entity.Vec2, respawn bool) error {
	concrete, ok := s.(*DefaultSession)
	if !ok || concrete == nil || s.Context().Err() != nil || !validPlayerPosition(position.X, position.Y, position.Z) || !finiteFloat32(rotation.X) || !finiteFloat32(rotation.Y) {
		return fmt.Errorf("dimension transfer: invalid request")
	}
	online := t.players.GetBySession(s)
	player, ok := s.Player().(*entity.Player)
	if online == nil || !ok || player == nil || player.Removed() || (!respawn && player.Living().IsDead()) {
		return fmt.Errorf("dimension transfer: player unavailable")
	}
	if t.pending[online.UUID] != nil {
		return fmt.Errorf("dimension transfer already pending")
	}
	if len(t.pending) >= maxPreparingTransfers {
		return fmt.Errorf("dimension preparation queue is full")
	}
	origin, sourceFound := t.manager.GetByDimension(SessionDimension(s))
	destination, targetFound := t.manager.GetByDimension(dimension)
	if !sourceFound || !targetFound || t.worlds[dimension].Accessor == nil || origin.GetEntity(player.ID()) != player.Entity {
		return fmt.Errorf("dimension transfer: world unavailable")
	}
	if !respawn && concrete.hasPendingTeleport() {
		return fmt.Errorf("dimension transfer: teleport confirmation pending")
	}
	transfer := &dimensionTransfer{session: concrete, online: online, origin: origin, destination: destination, binding: concrete.world.Load(), position: position, rotation: rotation, started: t.tick, respawn: respawn}
	if err := t.pinDestination(transfer); err != nil {
		return err
	}
	t.pending[online.UUID] = transfer
	return nil
}

// pinDestination retains collision dependencies without activating pregenerated chunks.
func (t *DimensionTransfers) pinDestination(transfer *dimensionTransfer) error {
	return transfer.destination.SetChunkTicket(chunk.Ticket{
		Kind: chunk.TicketPortal, Owner: transfer.session.ID(), X: chunkCoord(transfer.position.X), Z: chunkCoord(transfer.position.Z),
		ExpiresAt: transfer.destination.CurrentTick() + transferTimeoutTicks,
	})
}

// Tick advances preparation, cancellation, and publication of pending transfers.
func (t *DimensionTransfers) Tick() {
	t.tick++
	t.portalWork = 64
	t.tickPortalContacts()
	for uuid, transfer := range t.pending {
		s := transfer.session
		if s.Context().Err() != nil || s.world.Load() != transfer.binding || t.players.GetBySession(s) != transfer.online {
			t.cancel(uuid, transfer)
			continue
		}
		if t.tick-transfer.started >= transferTimeoutTicks {
			t.cancel(uuid, transfer)
			transfer.online.RespawnPending = false
			_ = s.Send(&play.SystemChatMessage{Text: "Destination preparation timed out", Overlay: true})
			continue
		}
		entry := t.worlds[transfer.destination.Dimension()]
		if err := prepareRespawnChunks(entry.Accessor, transfer.position.X, transfer.position.Z); err != nil {
			continue
		}
		if transfer.portal != nil {
			ready, err := t.preparePortal(transfer, entry)
			if err != nil {
				t.cancel(uuid, transfer)
				transfer.online.PortalCooldown = 300
				_ = s.Send(&play.SystemChatMessage{Text: err.Error(), Overlay: true})
				continue
			}
			if !ready {
				continue
			}
		}
		p := transfer.position
		if transfer.respawn || transfer.useSpawnPoint {
			online := transfer.online
			var resolved bool
			if online.HasSpawnPoint {
				p.X, p.Y, p.Z, resolved = playerRespawnPointInDimension(entry.Accessor, online, entry.Dimension)
				if !resolved {
					online.HasSpawnPoint = false
					t.cancel(uuid, transfer)
					_ = s.Send(&play.SystemChatMessage{Text: "Your respawn point was missing or obstructed"})
					if err := t.requestPersonalSpawn(s, transfer.respawn); err != nil {
						_ = s.ForceClose(err)
					}
					continue
				}
			} else {
				p.X, p.Y, p.Z, resolved = safeWorldRespawn(entry.Accessor, int(math.Floor(p.X)), int(p.Y), int(math.Floor(p.Z)))
			}
			if !resolved {
				continue
			}
		} else if !spawnPositionClear(entry.Accessor, p.X, p.Y, p.Z) {
			var resolved bool
			p.X, p.Y, p.Z, resolved = safeLocalRespawn(entry.Accessor, int(math.Floor(p.X)), int(p.Y), int(math.Floor(p.Z)))
			if !resolved {
				continue
			}
		}
		transfer.position = p
		if err := prepareRespawnChunks(entry.Accessor, p.X, p.Z); err != nil {
			continue
		}
		if err := transfer.destination.SetChunkTicket(chunk.Ticket{Kind: chunk.TicketPortal, Owner: transfer.session.ID(), X: chunkCoord(p.X), Z: chunkCoord(p.Z), ExpiresAt: transfer.destination.CurrentTick() + 300}); err != nil {
			t.cancel(uuid, transfer)
			_ = s.ForceClose(err)
			continue
		}
		if err := t.commit(transfer, entry); err != nil {
			t.cancel(uuid, transfer)
			_ = s.ForceClose(err)
			continue
		}
		// A successful portal ticket remains briefly after entry, as in normal travel.
		transfer.destination.RemoveChunkTicket(chunk.TicketView, transfer.session.ID()|(uint64(1)<<63))
		delete(t.pending, uuid)
	}
}

// cancel releases prepared residency while leaving origin membership unchanged.
func (t *DimensionTransfers) cancel(uuid protocol.UUID, transfer *dimensionTransfer) {
	transfer.destination.RemoveChunkTicket(chunk.TicketPortal, transfer.session.ID())
	transfer.destination.RemoveChunkTicket(chunk.TicketView, transfer.session.ID()|(uint64(1)<<63))
	delete(t.pending, uuid)
}

// commit publishes a prepared transition and fails closed on transport errors.
func (t *DimensionTransfers) commit(transfer *dimensionTransfer, entry PlayWorld) error {
	s, online := transfer.session, transfer.online
	player, ok := s.Player().(*entity.Player)
	if !ok || player == nil || s.Context().Err() != nil || transfer.origin.GetEntity(player.ID()) != player.Entity {
		return fmt.Errorf("dimension transfer: origin ownership lost")
	}
	if existing := transfer.destination.GetEntity(player.ID()); existing != nil && existing != player.Entity {
		return fmt.Errorf("dimension transfer: destination entity collision")
	}
	if online.Sleeping {
		wakePlayer(t.players.ForDimension(transfer.origin.Dimension()), online, t.worlds[transfer.origin.Dimension()].Accessor)
	}
	if online.Windows != nil {
		if active := online.Windows.ActiveWindow(); active != nil {
			if err := s.Send(&play.ClientboundCloseContainer{WindowID: active.ID}); err != nil {
				return err
			}
		}
		for _, stack := range online.Windows.CloseTemporaryCrafting() {
			spawnInventoryItem(t.players.ForDimension(transfer.origin.Dimension()), t.worlds[transfer.origin.Dimension()].Accessor, online.X, online.Y+.5, online.Z, stack)
		}
	}
	CancelPlayerActions(s)
	if !transfer.origin.DetachPlayer(player) {
		return fmt.Errorf("dimension transfer: failed to detach origin")
	}
	// A send failure keeps a valid authoritative origin for disconnect persistence.
	rollback := true
	defer func() {
		if rollback {
			transfer.origin.AddPlayer(player)
		}
	}()
	cooldown := online.PortalCooldown
	if !transfer.respawn && (transfer.portal != nil || transfer.useSpawnPoint) {
		cooldown = 300
	}
	kept := byte(3)
	if transfer.respawn {
		kept = 0
	}
	if err := s.Send(&play.Respawn{
		DimensionType: dimensionTypeID(entry.Config.RegistryManager, entry.Dimension), DimensionName: entry.Dimension,
		GameMode: byte(online.GameMode), PreviousGameMode: -1, SeaLevel: 63, DataKept: kept, PortalCooldown: cooldown,
	}); err != nil {
		return err
	}
	position := transfer.position
	teleportID := nextTeleportID()
	s.setPendingTeleport(teleportID)
	packets := []protocol.Packet{
		&play.Difficulty{DifficultyLevel: byte(transfer.destination.Difficulty())},
		&play.SetCenterChunk{ChunkX: chunkCoord(position.X), ChunkZ: chunkCoord(position.Z)},
		&play.SyncPlayerPosition{TeleportID: teleportID, X: position.X, Y: position.Y, Z: position.Z, Yaw: transfer.rotation.X, Pitch: transfer.rotation.Y},
		&play.GameEvent{Event: 13},
		&play.UpdateTime{WorldAge: transfer.destination.WorldAge(), TimeOfDay: transfer.destination.TimeOfDay(), TickDayTime: transfer.destination.DaylightCycle()},
	}
	for _, weather := range transfer.destination.Weather().JoinPackets() {
		packets = append(packets, &play.GameEvent{Event: weather.Event, Value: weather.Value})
	}
	for _, packet := range packets {
		if err := s.Send(packet); err != nil {
			return err
		}
	}
	if err := s.Context().Err(); err != nil {
		return err
	}
	if transfer.respawn {
		keepXP := online.DeathProcessed && online.DeathKeepInventory
		player.Living().RespawnWithKeepExperience(keepXP)
		online.DeathProcessed, online.RespawnPending, online.RespawnRetryTicks = false, false, 0
	} else {
		player.Living().SetFallDistance(0)
		player.SetVelocity(entity.Vec3{})
	}
	player.SetPosition(position)
	player.SetRotation(transfer.rotation)
	player.UpdateChunkCoords()
	player.SetClientLoaded(false)
	online.X, online.Y, online.Z = position.X, position.Y, position.Z
	online.movement = movementState{}
	online.Flags &^= 0x80
	online.Pose = pose.PoseStanding
	online.Yaw, online.Pitch = transfer.rotation.X, transfer.rotation.Y
	online.PortalCooldown = cooldown
	s.BindWorldDimension(entry.Dimension)
	transfer.destination.AddPlayer(player)
	rollback = false
	if transfer.respawn {
		s.SessionData().Store("player_flying", false)
	}
	flying, _ := s.SessionData().Load("player_flying")
	isFlying, _ := flying.(bool)
	if err := s.Send(playerAbilitiesWithSpeed(online.GameMode, online.SpeedBlocksPerSecond, isFlying)); err != nil {
		return err
	}
	if err := s.Send(&play.EntityUpdateAttributes{EntityID: player.ID(), Properties: player.Attributes().ToProperties()}); err != nil {
		return err
	}
	if err := s.Send(&play.UpdateHealth{Health: player.Living().Health(), Food: player.Living().FoodLevel(), FoodSaturation: player.Living().FoodSaturation()}); err != nil {
		return err
	}
	if err := s.Send(&play.SetExperience{ExperienceBar: player.Living().XPBar(), ExperienceLevel: player.Living().XPLevel(), TotalExperience: player.Living().XPTotal()}); err != nil {
		return err
	}
	for _, active := range player.Living().Effects().Active() {
		if err := s.Send(&play.EntityEffect{EntityID: player.ID(), EffectID: active.ID, Amplifier: active.Amplifier, Duration: active.Duration, Flags: active.Flags}); err != nil {
			return err
		}
	}
	if online.Windows != nil {
		if err := sendWindowResync(s, online.Windows, 0); err != nil {
			return err
		}
		return s.Send(&play.ClientboundHeldItemSlot{Slot: online.Windows.HeldSlot()})
	}
	return nil
}
