package session

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vitismc/vitis/internal/advancement"
	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/block/behavior"
	"github.com/vitismc/vitis/internal/command"
	"github.com/vitismc/vitis/internal/crafting"
	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	genentity "github.com/vitismc/vitis/internal/data/generated/entity"
	entitypose "github.com/vitismc/vitis/internal/data/generated/entity_pose"
	entitystatus "github.com/vitismc/vitis/internal/data/generated/entity_status"
	genparticle "github.com/vitismc/vitis/internal/data/generated/particle"
	gensound "github.com/vitismc/vitis/internal/data/generated/sound"
	soundcategory "github.com/vitismc/vitis/internal/data/generated/sound_category"
	worldevent "github.com/vitismc/vitis/internal/data/generated/world_event"
	"github.com/vitismc/vitis/internal/enchantment"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/metadata"
	"github.com/vitismc/vitis/internal/equipment"
	"github.com/vitismc/vitis/internal/experience"
	"github.com/vitismc/vitis/internal/food"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/logger"
	"github.com/vitismc/vitis/internal/mining"
	"github.com/vitismc/vitis/internal/operator"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/registry"
	"github.com/vitismc/vitis/internal/world/blockentity"
)

var teleportIDCounter atomic.Int32
var keepAliveValueCounter atomic.Int64

const playKeepAliveStartedKey = "play_keepalive_started"

const (
	maxPlayerCoordinate       = 3.0e7
	maxMovementDistanceSquare = 100.0 * 100.0
)

func nextTeleportID() int32 {
	return teleportIDCounter.Add(1)
}

func nextKeepAliveValue() int64 {
	return keepAliveValueCounter.Add(1)
}

func finiteFloat32(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}

func validPlayerPosition(x, y, z float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0) && !math.IsNaN(y) && !math.IsInf(y, 0) &&
		!math.IsNaN(z) && !math.IsInf(z, 0) && math.Abs(x) <= maxPlayerCoordinate &&
		math.Abs(y) <= maxPlayerCoordinate && math.Abs(z) <= maxPlayerCoordinate
}

func validateMovement(s Session, pm *PlayerManager, x, y, z float64, yaw, pitch float32, hasRotation bool) (bool, error) {
	if !validPlayerPosition(x, y, z) || (hasRotation && (!finiteFloat32(yaw) || !finiteFloat32(pitch))) {
		return false, fmt.Errorf("invalid player movement values")
	}
	ds, ok := s.(*DefaultSession)
	if ok && ds.hasPendingTeleport() {
		return false, nil
	}
	if pm == nil {
		return true, nil
	}
	op := pm.GetBySession(s)
	if op == nil {
		return true, nil
	}
	dx, dy, dz := x-op.X, y-op.Y, z-op.Z
	if dx*dx+dy*dy+dz*dz <= maxMovementDistanceSquare {
		return true, nil
	}
	teleportID := nextTeleportID()
	if ok {
		ds.setPendingTeleport(teleportID)
	}
	if err := s.Send(&playpacket.SyncPlayerPosition{
		TeleportID: teleportID,
		X:          op.X,
		Y:          op.Y,
		Z:          op.Z,
		Yaw:        op.Yaw,
		Pitch:      op.Pitch,
	}); err != nil {
		if ok {
			ds.pendingTeleport.CompareAndSwap(teleportID, 0)
		}
		return false, err
	}
	return false, nil
}

func blockWithinReach(player *entity.Player, position playpacket.BlockPos, reach float64) bool {
	if player == nil {
		return false
	}
	pos := player.Position()
	dx := (float64(position.X) + 0.5) - pos.X
	dy := (float64(position.Y) + 0.5) - (pos.Y + 1.62)
	dz := (float64(position.Z) + 0.5) - pos.Z
	return dx*dx+dy*dy+dz*dz <= reach*reach
}

func resyncBlock(s Session, wa WorldAccessor, position playpacket.BlockPos) error {
	stateID := int32(0)
	if wa != nil {
		stateID = wa.GetBlock(int(position.X), int(position.Y), int(position.Z))
	}
	if stateID < 0 {
		return nil
	}
	return s.Send(&playpacket.BlockUpdate{Position: position, BlockID: stateID})
}

func rejectBlockDig(s Session, wa WorldAccessor, position playpacket.BlockPos, sequence int32) error {
	if err := resyncBlock(s, wa, position); err != nil {
		return err
	}
	return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
}

// StartPlayKeepAlive starts periodic clientbound keepalive probes for an active play session.
func StartPlayKeepAlive(s Session, interval time.Duration) {
	if s == nil {
		return
	}
	if interval <= 0 {
		interval = 10 * time.Second
	}

	if ds, ok := s.(*DefaultSession); ok {
		if _, loaded := ds.SessionData().LoadOrStore(playKeepAliveStartedKey, true); loaded {
			return
		}
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		timeout := interval * 3
		sendProbe := func() bool {
			ds, ok := s.(*DefaultSession)
			if !ok {
				return false
			}
			if outstanding := ds.keepAliveID.Load(); outstanding != 0 {
				sentAt := time.Unix(0, ds.keepAliveSentAt.Load())
				if time.Since(sentAt) >= timeout {
					_ = s.ForceClose(fmt.Errorf("%w: id=%d", ErrKeepAliveTimeout, outstanding))
					return false
				}
				return true
			}
			id := nextKeepAliveValue()
			if !ds.beginKeepAlive(id) {
				return true
			}
			if err := s.Send(&playpacket.KeepAliveClientbound{Value: id}); err != nil {
				_ = s.ForceClose(err)
				return false
			}
			return true
		}
		if !sendProbe() {
			return
		}

		for {
			select {
			case <-s.Context().Done():
				return
			case <-ticker.C:
				if !sendProbe() {
					return
				}
			}
		}
	}()
}

// PlayBootstrapConfig holds configuration for the play state bootstrap sequence.
type PlayBootstrapConfig struct {
	DimensionName         string
	PortalCooldown        int32
	RequestRespawn        func(Session) error
	ViewDistance          int32
	SimulationDistance    int32
	SpawnX                float64
	SpawnY                float64
	SpawnZ                float64
	SpawnYaw              float32
	SpawnPitch            float32
	GameMode              int32
	Difficulty            int32
	InitialHealth         float32
	InitialFood           int32
	InitialFoodSaturation float32
	InitialXPLevel        int32
	InitialXPTotal        int32
	InitialXPBar          float32
	InitialSelectedSlot   int32
	WorldAge              int64
	TimeOfDay             int64
	TickDayTime           bool
	RegistryManager       *registry.Manager
	CommandRegistry       *command.Registry
	OperatorList          *operator.List
	PlayerUUID            protocol.UUID
	PlayerName            string
	TabHeader             string
	TabFooter             string
	WorldBorderInit       protocol.Packet
	WeatherPackets        []protocol.Packet
	ChatRateLimit         int
	CommandRateLimit      int
	KeepInventory         func() bool
	Crafting              *crafting.Runtime
	RecipeAdvancements    *advancement.Runtime
	ReadWorldSpawn        func() (int32, int32, int32, float32)
	GameRule              func(string) string
	CreateMap             func(int32, int32) (inventory.ItemStack, error)
}

// GameModeFromString converts a game mode name to its protocol ID.
func GameModeFromString(name string) int32 {
	switch name {
	case "survival":
		return 0
	case "creative":
		return 1
	case "adventure":
		return 2
	case "spectator":
		return 3
	default:
		return 0
	}
}

// DifficultyFromString converts a difficulty name to its protocol ID.
func DifficultyFromString(name string) int32 {
	switch name {
	case "peaceful":
		return 0
	case "easy":
		return 1
	case "hard":
		return 3
	default:
		return 2
	}
}

// DefaultPlayBootstrapConfig returns sane defaults for play bootstrap.
func DefaultPlayBootstrapConfig() PlayBootstrapConfig {
	return PlayBootstrapConfig{
		ViewDistance:          10,
		SimulationDistance:    10,
		SpawnX:                0.5,
		SpawnY:                65.0,
		SpawnZ:                0.5,
		Difficulty:            2,
		InitialHealth:         20,
		InitialFood:           20,
		InitialFoodSaturation: 5,
		TimeOfDay:             6000,
		TickDayTime:           true,
		ChatRateLimit:         5,
		CommandRateLimit:      10,
	}
}

type fixedWindowLimiter struct {
	mu          sync.Mutex
	windowStart time.Time
	count       int
}

func (l *fixedWindowLimiter) allow(now time.Time, limit int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.windowStart.IsZero() || now.Sub(l.windowStart) >= 10*time.Second {
		l.windowStart = now
		l.count = 0
	}
	if l.count >= limit {
		return false
	}
	l.count++
	return true
}

func allowSessionAction(s Session, key string, limit int) bool {
	if limit <= 0 {
		return false
	}
	ds, ok := s.(*DefaultSession)
	if !ok {
		return true
	}
	value, _ := ds.SessionData().LoadOrStore(key, &fixedWindowLimiter{})
	limiter, ok := value.(*fixedWindowLimiter)
	return ok && limiter.allow(time.Now(), limit)
}

func checkFallDamage(s Session, oldY, newY float64, onGround bool) {
	p, ok := s.Player().(*entity.Player)
	if !ok || p == nil {
		return
	}
	living := p.Living()
	if living.IsDead() || living.GameMode() == 1 || living.GameMode() == 3 || living.Effects().Has(effectdata.EffectSlowFalling) || living.Effects().Has(effectdata.EffectLevitation) {
		living.SetFallDistance(0)
		return
	}

	if newY < oldY {
		living.SetFallDistance(living.FallDistance() + (oldY - newY))
	}
	if onGround {
		fd := living.FallDistance()
		living.SetFallDistance(0)
		if boost := living.Effects().Get(effectdata.EffectJumpBoost); boost != nil {
			fd -= float64(boost.Amplifier) + 1
		}
		if fd > 3.0 {
			dmg := float32(math.Ceil(fd - 3.0))
			actual := living.ApplyDamage(entity.NewDamageSource(entity.DamageTypeFall, dmg), living.ArmorDefense(), living.ArmorToughness(), living.ArmorEnchProtection())
			if actual > 0 {
				_ = s.Send(&playpacket.UpdateHealth{
					Health:         living.Health(),
					Food:           living.FoodLevel(),
					FoodSaturation: living.FoodSaturation(),
				})
				_ = s.Send(&playpacket.HurtAnimation{
					EntityID: p.ID(),
					Yaw:      0,
				})
				if living.IsDead() {
					name := p.Username()
					_ = s.Send(&playpacket.DeathCombatEvent{
						PlayerID: p.ID(),
						Message:  name + " hit the ground too hard",
					})
				}
			}
		}
	}
}

func applyMovementExhaustion(s Session, op *OnlinePlayer, oldX, oldY, oldZ, newX, newY, newZ float64, onGround bool) {
	p, ok := s.Player().(*entity.Player)
	if !ok || p == nil {
		return
	}
	living := p.Living()
	if living.IsDead() || living.GameMode() == 1 || living.GameMode() == 3 {
		return
	}

	dx := newX - oldX
	dy := newY - oldY
	dz := newZ - oldZ
	horizontal := math.Sqrt(dx*dx + dz*dz)

	isSprinting := op.Flags&0x08 != 0

	if dy > 0 && p.OnGround() && !onGround {
		if isSprinting {
			living.AddExhaustion(0.2)
		} else {
			living.AddExhaustion(0.05)
		}
	}

	if horizontal > 0.0001 {
		if isSprinting {
			living.AddExhaustion(float32(horizontal * 0.1))
		}
	}
}

func chunkCoord(v float64) int32 {
	return int32(math.Floor(v / 16))
}

// RegisterPlayHandlers registers handlers for Play-state packets on a packet router.
func RegisterPlayHandlers(router PacketRouter, cfg PlayBootstrapConfig, pm *PlayerManager, wa WorldAccessor) error {
	if router == nil {
		return ErrNilRegistry
	}
	if err := registerRecipePlacementHandler(router, pm, wa, cfg); err != nil {
		return err
	}
	if err := registerRecipeBookHandlers(router, pm); err != nil {
		return err
	}

	confirmTeleportID := playpacket.NewConfirmTeleportation().ID()
	if err := router.Register(protocol.StatePlay, confirmTeleportID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.ConfirmTeleportation)
		if !ok {
			return protocol.ErrNilPacket
		}
		ds, ok := s.(*DefaultSession)
		if !ok {
			return fmt.Errorf("confirm teleport: invalid session type")
		}
		if wa == nil || !wa.Submit(&teleportAckCommand{session: ds, id: pkt.TeleportID, players: pm}) {
			return fmt.Errorf("world command queue is unavailable or full")
		}
		logger.Debug("confirm_teleportation", "session", s.ID(), "teleport_id", pkt.TeleportID)
		return nil
	}); err != nil {
		return err
	}

	keepAliveID := playpacket.NewKeepAliveServerbound().ID()
	if err := router.Register(protocol.StatePlay, keepAliveID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.KeepAliveServerbound)
		if !ok {
			return protocol.ErrNilPacket
		}
		ds, ok := s.(*DefaultSession)
		if !ok {
			return fmt.Errorf("keepalive: invalid session type")
		}
		return ds.acknowledgeKeepAlive(pkt.Value)
	}); err != nil {
		return err
	}

	positionID := playpacket.NewSetPlayerPosition().ID()
	if err := router.Register(protocol.StatePlay, positionID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.SetPlayerPosition)
		if !ok {
			return protocol.ErrNilPacket
		}
		return submitMovement(s, wa, movementCommand{
			players: pm, hasPosition: true,
			x: pkt.X, y: pkt.Y, z: pkt.Z, onGround: pkt.OnGround,
		})
	}); err != nil {
		return err
	}

	posRotID := playpacket.NewSetPlayerPositionAndRotation().ID()
	if err := router.Register(protocol.StatePlay, posRotID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.SetPlayerPositionAndRotation)
		if !ok {
			return protocol.ErrNilPacket
		}
		return submitMovement(s, wa, movementCommand{
			players: pm, hasPosition: true, hasRotation: true,
			x: pkt.X, y: pkt.Y, z: pkt.Z,
			yaw: pkt.Yaw, pitch: pkt.Pitch, onGround: pkt.OnGround,
		})
	}); err != nil {
		return err
	}

	rotID := playpacket.NewSetPlayerRotation().ID()
	if err := router.Register(protocol.StatePlay, rotID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.SetPlayerRotation)
		if !ok {
			return protocol.ErrNilPacket
		}
		return submitMovement(s, wa, movementCommand{
			players: pm, hasRotation: true,
			yaw: pkt.Yaw, pitch: pkt.Pitch, onGround: pkt.OnGround,
		})
	}); err != nil {
		return err
	}

	onGroundID := playpacket.NewSetPlayerOnGround().ID()
	if err := router.Register(protocol.StatePlay, onGroundID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.SetPlayerOnGround)
		if !ok {
			return protocol.ErrNilPacket
		}
		return submitMovement(s, wa, movementCommand{players: pm, onGround: pkt.OnGround})
	}); err != nil {
		return err
	}

	chunkBatchReceivedID := playpacket.NewChunkBatchReceived().ID()
	if err := router.Register(protocol.StatePlay, chunkBatchReceivedID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.ChunkBatchReceived)
		if !ok {
			return protocol.ErrNilPacket
		}
		if math.IsNaN(float64(pkt.ChunksPerTick)) || math.IsInf(float64(pkt.ChunksPerTick), 0) || pkt.ChunksPerTick < 0 || pkt.ChunksPerTick > 1000 {
			return fmt.Errorf("invalid chunk batch rate: %v", pkt.ChunksPerTick)
		}
		if ds, ok := s.(*DefaultSession); ok {
			ds.SessionData().Store("chunk_batch_rate", pkt.ChunksPerTick)
			ds.AcknowledgeChunkBatch()
		}
		return nil
	}); err != nil {
		return err
	}

	playerLoadedID := playpacket.NewPlayerLoaded().ID()
	if err := router.Register(protocol.StatePlay, playerLoadedID, func(s Session, packet protocol.Packet) error {
		if _, ok := packet.(*playpacket.PlayerLoaded); !ok {
			return protocol.ErrNilPacket
		}
		if wa == nil || !wa.Submit(&clientLoadedCommand{session: s}) {
			return fmt.Errorf("world command queue is unavailable or full")
		}
		return nil
	}); err != nil {
		return err
	}

	abilitiesID := playpacket.NewPlayerAbilitiesServerbound().ID()
	if err := router.Register(protocol.StatePlay, abilitiesID, func(s Session, packet protocol.Packet) error {
		abilities, ok := packet.(*playpacket.PlayerAbilitiesServerbound)
		if !ok {
			return protocol.ErrNilPacket
		}
		if wa == nil || !wa.Submit(&abilitiesCommand{session: s, players: pm, flying: abilities.Flags&playpacket.AbilityFlying != 0}) {
			return fmt.Errorf("world command queue is unavailable or full")
		}
		return nil
	}); err != nil {
		return err
	}

	chatCommandID := playpacket.NewChatCommand().ID()
	if err := router.Register(protocol.StatePlay, chatCommandID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.ChatCommand)
		if !ok {
			return protocol.ErrNilPacket
		}
		if !allowSessionAction(s, "command_rate", cfg.CommandRateLimit) {
			return s.Send(&playpacket.SystemChatMessage{Text: "You are sending commands too quickly", Overlay: true})
		}
		logger.Debug("chat_command", "session", s.ID(), "command", pkt.Command)
		if cfg.CommandRegistry != nil {
			sender := newSessionCommandSender(s, pm, cfg.OperatorList)
			return cfg.CommandRegistry.Dispatch(sender, pkt.Command)
		}
		return nil
	}); err != nil {
		return err
	}

	chatCommandSignedID := playpacket.NewChatCommandSigned().ID()
	if err := router.Register(protocol.StatePlay, chatCommandSignedID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.ChatCommandSigned)
		if !ok {
			return protocol.ErrNilPacket
		}
		if !allowSessionAction(s, "command_rate", cfg.CommandRateLimit) {
			return s.Send(&playpacket.SystemChatMessage{Text: "You are sending commands too quickly", Overlay: true})
		}
		logger.Debug("chat_command_signed", "session", s.ID(), "command", pkt.Command)
		if cfg.CommandRegistry != nil {
			sender := newSessionCommandSender(s, pm, cfg.OperatorList)
			return cfg.CommandRegistry.Dispatch(sender, pkt.Command)
		}
		resp := playpacket.NewSystemChatText("Unknown command: /" + pkt.Command)
		return s.Send(resp)
	}); err != nil {
		return err
	}

	miningTracker := mining.NewTracker()
	eatingTracker := food.NewEatingTracker()

	blockDigID := playpacket.NewBlockDig().ID()
	if err := router.Register(protocol.StatePlay, blockDigID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.BlockDig)
		if !ok {
			return protocol.ErrNilPacket
		}
		if pkt.Status < playpacket.DigStarted || pkt.Status > playpacket.DigSwapHands || pkt.Face < 0 || pkt.Face > 5 || pkt.Sequence < 0 {
			return fmt.Errorf("invalid block dig action: status=%d face=%d sequence=%d", pkt.Status, pkt.Face, pkt.Sequence)
		}
		if wa == nil || !wa.Submit(&digActionCommand{
			session: s, players: pm, world: wa, packet: *pkt,
			mining: miningTracker, eating: eatingTracker, receivedAt: time.Now(),
		}) {
			return rejectBlockDig(s, wa, pkt.Position, pkt.Sequence)
		}
		return nil
	}); err != nil {
		return err
	}

	useItemOnID := playpacket.NewUseItemOn().ID()
	if err := router.Register(protocol.StatePlay, useItemOnID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.UseItemOn)
		if !ok {
			return protocol.ErrNilPacket
		}
		if pkt.Hand < 0 || pkt.Hand > 1 || pkt.Face < 0 || pkt.Face > 5 || pkt.Sequence < 0 {
			return fmt.Errorf("invalid use item on packet")
		}
		if pm != nil && wa != nil {
			if op := pm.GetBySession(s); op != nil && op.Windows != nil {
				useSlot := inventory.HotbarStart + int(op.Windows.HeldSlot())
				if pkt.Hand == 1 {
					useSlot = inventory.OffhandSlot
				}
				name := item.NameByID(op.Windows.GetSlot(useSlot).ItemID)
				if name == "minecraft:flint_and_steel" || name == "minecraft:fire_charge" {
					if !pkt.WorldBorderHit && wa.Submit(&ignitionCommand{session: s, players: pm, world: wa, packet: *pkt}) {
						return nil
					}
					return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: pkt.Sequence})
				}
			}
		}

		if wa != nil {
			clickedState := wa.GetBlock(int(pkt.Position.X), int(pkt.Position.Y), int(pkt.Position.Z))
			if clickedState > 0 {
				ctx := &behavior.Context{
					X: int(pkt.Position.X), Y: int(pkt.Position.Y), Z: int(pkt.Position.Z),
					StateID: clickedState, Face: pkt.Face,
				}
				if behavior.GetByState(clickedState).OnUse(ctx) {
					if ctx.StateID == -2 && behavior.IsBed(clickedState) {
						if wa.Submit(&bedUseCommand{session: s, players: pm, world: wa, x: int(pkt.Position.X), y: int(pkt.Position.Y), z: int(pkt.Position.Z), sequence: pkt.Sequence}) {
							return nil
						}
						return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: pkt.Sequence})
					}
					if ctx.StateID == -1 {
						if _, _, _, furnace := behavior.IsFurnace(clickedState); furnace {
							if wa.Submit(&openBlockWindowCommand{session: s, players: pm, world: wa, x: pkt.Position.X, y: pkt.Position.Y, z: pkt.Position.Z, sequence: pkt.Sequence}) {
								return nil
							}
						}
						if _, _, _, container := behavior.IsContainer(clickedState); container {
							if wa.Submit(&openBlockWindowCommand{session: s, players: pm, world: wa, x: pkt.Position.X, y: pkt.Position.Y, z: pkt.Position.Z, sequence: pkt.Sequence}) {
								return nil
							}
						}
						return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: pkt.Sequence})
					}
					wa.SetBlock(int(pkt.Position.X), int(pkt.Position.Y), int(pkt.Position.Z), ctx.StateID)
					blockUpdate := &playpacket.BlockUpdate{
						Position: pkt.Position,
						BlockID:  ctx.StateID,
					}
					if err := s.Send(blockUpdate); err != nil {
						return err
					}
					if pm != nil {
						if op := pm.GetBySession(s); op != nil {
							pm.BroadcastExcept(op.UUID, blockUpdate)
						}
					}

					// Double-door sync
					if behavior.IsDoor(ctx.StateID) && wa != nil {
						bx, by, bz := int(pkt.Position.X), int(pkt.Position.Y), int(pkt.Position.Z)
						if ax, ay, az, adjState, ok := behavior.FindAdjacentDoor(ctx.StateID, bx, by, bz, wa.GetBlock); ok {
							wa.SetBlock(ax, ay, az, adjState)
							adjUpdate := &playpacket.BlockUpdate{
								Position: playpacket.BlockPos{X: int32(ax), Y: int32(ay), Z: int32(az)},
								BlockID:  adjState,
							}
							if pm != nil {
								pm.Broadcast(adjUpdate)
							} else {
								_ = s.Send(adjUpdate)
							}
						}
					}

					// Door/trapdoor/fence gate sounds
					if behavior.IsDoor(ctx.StateID) || behavior.IsFenceGate(ctx.StateID) {
						props := block.PropertiesFromState(ctx.StateID)
						soundName := "block.wooden_door.open"
						if props != nil && props["open"] == "false" {
							soundName = "block.wooden_door.close"
						}
						if behavior.IsFenceGate(ctx.StateID) {
							soundName = "block.fence_gate.open"
							if props != nil && props["open"] == "false" {
								soundName = "block.fence_gate.close"
							}
						}
						sid := gensound.SoundIDByName(soundName)
						if sid >= 0 && pm != nil {
							pm.Broadcast(&playpacket.SoundEffect{
								SoundID:       sid + 1,
								SoundCategory: soundcategory.CategoryBlock,
								X:             pkt.Position.X * 8,
								Y:             pkt.Position.Y * 8,
								Z:             pkt.Position.Z * 8,
								Volume:        1.0,
								Pitch:         1.0,
							})
						}
					}

					return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: pkt.Sequence})
				}
			}
		}

		dx, dy, dz := BlockFaceOffset(pkt.Face)
		targetX := pkt.Position.X + dx
		targetY := pkt.Position.Y + dy
		targetZ := pkt.Position.Z + dz

		if wa != nil {
			existing := wa.GetBlock(int(targetX), int(targetY), int(targetZ))
			if existing < 0 {
				return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: pkt.Sequence})
			}
			if block.IsSolid(existing) {
				_ = s.Send(&playpacket.BlockUpdate{
					Position: playpacket.BlockPos{X: targetX, Y: targetY, Z: targetZ},
					BlockID:  existing,
				})
				return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: pkt.Sequence})
			}
		}

		if pm != nil {
			tx, ty, tz := float64(targetX), float64(targetY), float64(targetZ)
			for _, op := range pm.Players() {
				px, py, pz := op.X, op.Y, op.Z
				if px >= tx && px < tx+1 && pz >= tz && pz < tz+1 && py >= ty-1.8 && py < ty+1 {
					actual := int32(0)
					if wa != nil {
						actual = wa.GetBlock(int(targetX), int(targetY), int(targetZ))
					}
					_ = s.Send(&playpacket.BlockUpdate{
						Position: playpacket.BlockPos{X: targetX, Y: targetY, Z: targetZ},
						BlockID:  actual,
					})
					return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: pkt.Sequence})
				}
			}
		}

		blockToPlace := int32(0)
		var op *OnlinePlayer
		if pm != nil {
			op = pm.GetBySession(s)
			if op != nil && op.Windows != nil {
				held := op.Windows.HeldItem()
				if held.ItemCount > 0 && held.ItemID > 0 {
					blockToPlace = resolveDirectionalBlockState(held.ItemID, pkt.Face, op.Yaw)
				}
			}
		}

		if blockToPlace > 0 && wa != nil {
			return submitPlaceBlock(s, pm, wa, op, targetX, targetY, targetZ, blockToPlace, pkt.Sequence)
		}

		return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: pkt.Sequence})
	}); err != nil {
		return err
	}

	armAnimID := playpacket.NewArmAnimation().ID()
	if err := router.Register(protocol.StatePlay, armAnimID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.ArmAnimation)
		if !ok {
			return protocol.ErrNilPacket
		}
		if pm == nil {
			return nil
		}
		op := pm.GetBySession(s)
		if op == nil {
			return nil
		}
		anim := playpacket.AnimationSwingMainArm
		if pkt.Hand == 1 {
			anim = playpacket.AnimationSwingOffhand
		}
		pm.BroadcastExcept(op.UUID, &playpacket.EntityAnimation{
			EntityID:  op.EntityID,
			Animation: anim,
		})
		return nil
	}); err != nil {
		return err
	}

	eaID := playpacket.NewEntityAction().ID()
	if err := router.Register(protocol.StatePlay, eaID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.EntityAction)
		if !ok {
			return protocol.ErrNilPacket
		}
		if pm == nil {
			return nil
		}
		if wa == nil || !wa.Submit(&playerModeCommand{session: s, players: pm, world: wa, request: *pkt}) {
			return fmt.Errorf("world command queue is unavailable or full")
		}
		return nil
	}); err != nil {
		return err
	}

	heldItemID := playpacket.NewServerboundHeldItemSlot().ID()
	if err := router.Register(protocol.StatePlay, heldItemID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.ServerboundHeldItemSlot)
		if !ok {
			return protocol.ErrNilPacket
		}
		if pkt.Slot < 0 || pkt.Slot > 8 {
			return fmt.Errorf("held item slot out of range: %d", pkt.Slot)
		}
		if wa == nil || !wa.Submit(&heldSlotCommand{
			session: s, players: pm, eating: eatingTracker, slot: int32(pkt.Slot),
		}) {
			if pm != nil {
				if op := pm.GetBySession(s); op != nil && op.Windows != nil {
					return s.Send(&playpacket.ClientboundHeldItemSlot{Slot: op.Windows.HeldSlot()})
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}

	clientCommandID := playpacket.NewClientCommand().ID()
	if err := router.Register(protocol.StatePlay, clientCommandID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.ClientCommand)
		if !ok {
			return protocol.ErrNilPacket
		}
		if pkt.ActionID == 0 {
			if wa != nil && wa.Submit(&respawnPlayerCommand{session: s, players: pm, world: wa, config: cfg}) {
				return nil
			}
			return fmt.Errorf("respawn queue unavailable")
		}
		return nil
	}); err != nil {
		return err
	}

	settingsID := playpacket.NewServerboundSettings().ID()
	if err := router.Register(protocol.StatePlay, settingsID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.ServerboundSettings)
		if !ok {
			return protocol.ErrNilPacket
		}
		if ds, ok := s.(*DefaultSession); ok {
			ds.SessionData().Store("client_view_distance", int32(pkt.ViewDistance))
		}
		return nil
	}); err != nil {
		return err
	}

	creativeSlotID := playpacket.NewSetCreativeSlot().ID()
	if err := router.Register(protocol.StatePlay, creativeSlotID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.SetCreativeSlot)
		if !ok {
			return protocol.ErrNilPacket
		}
		player, playerOK := s.Player().(*entity.Player)
		if !playerOK || player == nil || player.Living().GameMode() != 1 {
			return fmt.Errorf("creative slot action without creative capability")
		}
		if pkt.SlotIndex < 1 || int(pkt.SlotIndex) >= inventory.PlayerInventorySize {
			return fmt.Errorf("creative slot index out of range: %d", pkt.SlotIndex)
		}
		if !pkt.Item.Empty() {
			maxStack := pkt.Item.MaxStackSize()
			if maxStack <= 0 || pkt.Item.ItemCount > maxStack {
				return fmt.Errorf("invalid creative item id=%d count=%d", pkt.Item.ItemID, pkt.Item.ItemCount)
			}
		}
		if wa == nil || !wa.Submit(&creativeSlotCommand{
			session: s, players: pm, slot: pkt.SlotIndex, item: pkt.Item.Clone(),
		}) {
			if pm != nil {
				if op := pm.GetBySession(s); op != nil && op.Windows != nil {
					return sendWindowSnapshot(s, op.Windows, 0)
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}

	useItemID := playpacket.NewUseItem().ID()
	if err := router.Register(protocol.StatePlay, useItemID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.UseItem)
		if !ok {
			return protocol.ErrNilPacket
		}

		if wa == nil || !wa.Submit(&useItemCommand{session: s, players: pm, world: wa, request: *pkt, config: cfg, eating: eatingTracker, mining: miningTracker}) {
			return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: pkt.Sequence})
		}
		return nil
	}); err != nil {
		return err
	}

	chatMessageID := playpacket.NewServerboundChatMessage().ID()
	if err := router.Register(protocol.StatePlay, chatMessageID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.ServerboundChatMessage)
		if !ok {
			return protocol.ErrNilPacket
		}
		if !allowSessionAction(s, "chat_rate", cfg.ChatRateLimit) {
			return s.Send(&playpacket.SystemChatMessage{Text: "You are sending messages too quickly", Overlay: true})
		}
		senderName := "Player"
		if ds, ok := s.(*DefaultSession); ok {
			if raw, found := ds.SessionData().Load("login_name"); found {
				if name, ok := raw.(string); ok {
					senderName = name
				}
			}
		}
		logger.Info("chat", "player", senderName, "message", pkt.Message)
		if pm != nil {
			pm.Global().Broadcast(playpacket.NewDisguisedChatSimple(pkt.Message, senderName))
		}
		return nil
	}); err != nil {
		return err
	}

	if err := registerMenuButtonHandler(router, pm, wa, cfg.Crafting); err != nil {
		return err
	}
	windowClickID := playpacket.NewWindowClick().ID()
	if err := router.Register(protocol.StatePlay, windowClickID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.WindowClick)
		if !ok {
			return protocol.ErrNilPacket
		}
		if pm == nil {
			return nil
		}
		if wa == nil || !wa.Submit(&windowClickCommand{session: s, players: pm, world: wa, packet: pkt}) {
			if op := pm.GetBySession(s); op != nil && op.Windows != nil {
				return sendWindowSnapshot(s, op.Windows, pkt.WindowID)
			}
		}
		return nil
	}); err != nil {
		return err
	}

	editBookID := playpacket.NewEditBook().ID()
	if err := router.Register(protocol.StatePlay, editBookID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.EditBook)
		if !ok {
			return protocol.ErrNilPacket
		}
		if pm == nil {
			return nil
		}
		op := pm.GetBySession(s)
		if op == nil || op.Windows == nil {
			return nil
		}

		// The slot index is the hotbar slot
		invIdx := inventory.HotbarStart + int(pkt.Slot)
		held := op.Windows.Inventory().Get(invIdx)
		if held.Empty() {
			return nil
		}

		bookName := item.NameByID(held.ItemID)
		if bookName != "minecraft:writable_book" {
			return nil
		}

		if pkt.HasTitle {
			// Signing: convert writable_book to written_book
			writtenID := item.IDByName("minecraft:written_book")
			if writtenID < 0 {
				return nil
			}
			newSlot := inventory.NewSlot(writtenID, 1)
			// Store title and pages as raw component data (simplified)
			op.Windows.Inventory().Set(invIdx, newSlot)
			_ = sendWindowResync(s, op.Windows, 0)
		} else {
			// Just editing pages - keep the writable_book, acknowledge
			// Pages are stored client-side until signing; server just acknowledges
		}

		return nil
	}); err != nil {
		return err
	}

	updateSignID := playpacket.NewServerboundUpdateSign().ID()
	if err := router.Register(protocol.StatePlay, updateSignID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.ServerboundUpdateSign)
		if !ok {
			return protocol.ErrNilPacket
		}
		if wa == nil {
			return nil
		}
		bes := wa.BlockEntities()
		if bes == nil {
			return nil
		}
		sd := bes.GetSign(int(pkt.X), int(pkt.Y), int(pkt.Z))
		if sd == nil {
			sd = &blockentity.SignData{}
			bes.SetSign(int(pkt.X), int(pkt.Y), int(pkt.Z), sd)
		}
		if pkt.IsFront {
			sd.FrontLines = pkt.Lines
		} else {
			sd.BackLines = pkt.Lines
		}
		bes.SetSign(int(pkt.X), int(pkt.Y), int(pkt.Z), sd)

		nbtData := blockentity.EncodeSignNBT(sd)
		beType := blockentity.TypeSign
		blockState := wa.GetBlock(int(pkt.X), int(pkt.Y), int(pkt.Z))
		blockName := block.NameFromState(blockState)
		if strings.Contains(blockName, "hanging") {
			beType = blockentity.TypeHangSign
		}
		if pm != nil {
			pm.Broadcast(&playpacket.BlockEntityData{
				X: pkt.X, Y: pkt.Y, Z: pkt.Z,
				Type:    beType,
				NBTData: nbtData,
			})
		}
		return nil
	}); err != nil {
		return err
	}

	closeWindowID := playpacket.NewCloseWindow().ID()
	if err := router.Register(protocol.StatePlay, closeWindowID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.CloseWindow)
		if !ok {
			return protocol.ErrNilPacket
		}
		if pkt.WindowID <= 0 {
			return nil
		}
		if wa == nil || !wa.Submit(&closeWindowCommand{
			session: s, players: pm, world: wa, windowID: pkt.WindowID,
		}) {
			if pm != nil {
				if op := pm.GetBySession(s); op != nil && op.Windows != nil {
					if active := op.Windows.ActiveWindow(); active != nil && active.ID == pkt.WindowID {
						if err := s.Send(&playpacket.OpenScreen{
							WindowID: active.ID, WindowType: active.Type, Title: active.Title,
						}); err != nil {
							return err
						}
						return sendWindowSnapshot(s, op.Windows, active.ID)
					}
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}

	useEntityID := playpacket.NewUseEntity().ID()
	if err := router.Register(protocol.StatePlay, useEntityID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.UseEntity)
		if !ok {
			return protocol.ErrNilPacket
		}
		if pm == nil {
			return nil
		}
		if pkt.Type == playpacket.UseEntityAttack {
			if wa == nil || !wa.Submit(&attackEntityCommand{session: s, players: pm, world: wa, target: pkt.EntityID}) {
				return fmt.Errorf("world command queue is unavailable or full")
			}
		}
		return nil
	}); err != nil {
		return err
	}

	tabCompleteID := playpacket.NewServerboundTabComplete().ID()
	if err := router.Register(protocol.StatePlay, tabCompleteID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.ServerboundTabComplete)
		if !ok {
			return protocol.ErrNilPacket
		}
		text := pkt.Text
		if cfg.CommandRegistry == nil {
			return nil
		}
		var sender command.Sender
		if cfg.OperatorList != nil {
			sender = newSessionCommandSender(s, pm, cfg.OperatorList)
		}
		suggestions := cfg.CommandRegistry.TabSuggestions(sender, text)
		if len(suggestions) == 0 && pm != nil {
			lastWord := text
			if idx := strings.LastIndexByte(text, ' '); idx >= 0 {
				lastWord = text[idx+1:]
			}
			prefix := strings.ToLower(lastWord)
			for _, name := range pm.OnlinePlayerNames() {
				if strings.HasPrefix(strings.ToLower(name), prefix) {
					suggestions = append(suggestions, name)
				}
			}
		}
		if len(suggestions) == 0 {
			return nil
		}
		lastSpace := 0
		for i := len(text) - 1; i >= 0; i-- {
			if text[i] == ' ' {
				lastSpace = i + 1
				break
			}
		}
		matches := make([]playpacket.TabCompleteMatch, len(suggestions))
		for i, sug := range suggestions {
			matches[i] = playpacket.TabCompleteMatch{Match: sug}
		}
		return s.Send(&playpacket.ClientboundTabComplete{
			TransactionID: pkt.TransactionID,
			Start:         int32(lastSpace),
			Length:        int32(len(text) - lastSpace),
			Matches:       matches,
		})
	}); err != nil {
		return err
	}

	playerInputID := playpacket.NewPlayerInput().ID()
	if err := router.Register(protocol.StatePlay, playerInputID, func(_ Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.PlayerInput)
		if !ok {
			return protocol.ErrNilPacket
		}
		if pkt.Flags&^byte(0x7F) != 0 {
			return fmt.Errorf("player input has reserved flags: 0x%02X", pkt.Flags)
		}
		// Input is meaningful only while a vehicle controller is attached. Until
		// vehicle ownership is implemented, accepting this validated neutral input
		// preserves the normal client tick flow without changing world state.
		return nil
	}); err != nil {
		return err
	}

	pingRequestID := playpacket.NewServerboundPingRequest().ID()
	if err := router.Register(protocol.StatePlay, pingRequestID, func(s Session, packet protocol.Packet) error {
		ping, ok := packet.(*playpacket.ServerboundPingRequest)
		if !ok {
			return protocol.ErrNilPacket
		}
		return s.Send(&playpacket.ClientboundPingResponse{Payload: ping.Payload})
	}); err != nil {
		return err
	}

	if err := registerPlayValidatedNoOps(router); err != nil {
		return err
	}

	return nil
}

// SendPlayBootstrap sends the initial play-state packet sequence to the client.
func SendPlayBootstrap(s Session, entityID int32, cfg PlayBootstrapConfig) error {
	if cfg.InitialSelectedSlot < 0 || cfg.InitialSelectedSlot > 8 {
		return fmt.Errorf("invalid initial selected item slot")
	}
	dimension := cfg.DimensionName
	if dimension == "" {
		dimension = "minecraft:overworld"
	}
	teleportID := nextTeleportID()

	respawnScreen, limitedCrafting := true, false
	if cfg.GameRule != nil {
		respawnScreen = cfg.GameRule("doImmediateRespawn") != "true"
		limitedCrafting = cfg.GameRule("doLimitedCrafting") == "true"
	}
	loginPlay := &playpacket.LoginPlay{
		EntityID:            entityID,
		IsHardcore:          false,
		DimensionNames:      []string{"minecraft:overworld", "minecraft:the_nether", "minecraft:the_end"},
		MaxPlayers:          20,
		ViewDistance:        cfg.ViewDistance,
		SimulationDistance:  cfg.SimulationDistance,
		ReducedDebugInfo:    false,
		EnableRespawnScreen: respawnScreen,
		DoLimitedCrafting:   limitedCrafting,
		DimensionType:       dimensionTypeID(cfg.RegistryManager, dimension),
		DimensionName:       dimension,
		HashedSeed:          0,
		GameMode:            byte(cfg.GameMode),
		PreviousGameMode:    -1,
		IsDebug:             false,
		IsFlat:              false,
		HasDeathLocation:    false,
		PortalCooldown:      cfg.PortalCooldown,
		SeaLevel:            63,
		EnforcesSecureChat:  false,
	}
	if err := s.Send(loginPlay); err != nil {
		return err
	}
	if err := s.Send(&playpacket.Difficulty{DifficultyLevel: byte(cfg.Difficulty), Locked: false}); err != nil {
		return err
	}
	logger.Debug("sent login_play", "session", s.ID(), "entity_id", entityID)

	if cfg.PlayerName != "" {
		playerInfo := &playpacket.PlayerInfoUpdate{
			Actions: playpacket.ActionAddPlayer | playpacket.ActionUpdateListed | playpacket.ActionUpdateGameMode | playpacket.ActionUpdateLatency,
			Entries: []playpacket.PlayerInfoEntry{
				{
					UUID:     cfg.PlayerUUID,
					Name:     cfg.PlayerName,
					GameMode: cfg.GameMode,
					Listed:   true,
					Ping:     0,
				},
			},
		}
		if err := s.Send(playerInfo); err != nil {
			return err
		}
	}

	abilities := playerAbilitiesWithSpeed(cfg.GameMode, 0, cfg.GameMode == 3)
	if err := s.Send(abilities); err != nil {
		return err
	}

	if err := s.Send(&playpacket.SetTickingState{TickRate: 20.0, IsFrozen: false}); err != nil {
		return err
	}

	if err := s.Send(&playpacket.ServerData{MOTDText: "Vitis Server"}); err != nil {
		return err
	}

	if cfg.CommandRegistry != nil {
		var graphSender command.Sender
		if cfg.OperatorList != nil {
			graphSender = newSessionCommandSender(s, nil, cfg.OperatorList)
		}
		graphData := command.EncodeCommandGraph(cfg.CommandRegistry, graphSender)
		if err := s.Send(&playpacket.DeclareCommands{Data: graphData}); err != nil {
			return err
		}
	} else {
		if err := s.Send(playpacket.EmptyCommandGraph()); err != nil {
			return err
		}
	}

	if err := s.Send(&playpacket.UpdateHealth{Health: cfg.InitialHealth, Food: cfg.InitialFood, FoodSaturation: cfg.InitialFoodSaturation}); err != nil {
		return err
	}

	{
		ac := entity.DefaultPlayerAttributes()
		if err := s.Send(&playpacket.EntityUpdateAttributes{
			EntityID:   entityID,
			Properties: ac.ToProperties(),
		}); err != nil {
			return err
		}
	}

	if err := s.Send(&playpacket.UpdateTime{WorldAge: cfg.WorldAge, TimeOfDay: cfg.TimeOfDay, TickDayTime: cfg.TickDayTime}); err != nil {
		return err
	}

	if err := s.Send(&playpacket.SetExperience{ExperienceBar: cfg.InitialXPBar, ExperienceLevel: cfg.InitialXPLevel, TotalExperience: cfg.InitialXPTotal}); err != nil {
		return err
	}

	emptySlots := make([]inventory.Slot, inventory.PlayerInventorySize)
	if err := s.Send(&playpacket.SetContainerContent{
		WindowID: 0,
		StateID:  1,
		Slots:    emptySlots,
		Carried:  inventory.EmptySlot(),
	}); err != nil {
		return err
	}

	if err := s.Send(&playpacket.ClientboundHeldItemSlot{Slot: cfg.InitialSelectedSlot}); err != nil {
		return err
	}

	if err := s.Send(&playpacket.SetCursorItem{SlotData: inventory.EmptySlot()}); err != nil {
		return err
	}

	var wbPkt protocol.Packet
	if cfg.WorldBorderInit != nil {
		wbPkt = cfg.WorldBorderInit
	} else {
		wbPkt = &playpacket.InitializeWorldBorder{
			OldDiameter:            60000000,
			NewDiameter:            60000000,
			PortalTeleportBoundary: 29999984,
			WarningBlocks:          5,
			WarningTime:            15,
		}
	}
	if err := s.Send(wbPkt); err != nil {
		return err
	}

	for _, wp := range cfg.WeatherPackets {
		if err := s.Send(wp); err != nil {
			return err
		}
	}

	tabHeader := cfg.TabHeader
	if tabHeader == "" {
		tabHeader = "§6Vitis Server"
	}
	tabFooter := cfg.TabFooter
	if tabFooter == "" {
		tabFooter = "§7Powered by Go"
	}
	if err := s.Send(&playpacket.PlayerlistHeader{
		Header: tabHeader,
		Footer: tabFooter,
	}); err != nil {
		return err
	}

	spawnPos := &playpacket.SetDefaultSpawnPosition{
		X:     int32(cfg.SpawnX),
		Y:     int32(cfg.SpawnY),
		Z:     int32(cfg.SpawnZ),
		Angle: 0.0,
	}
	if err := s.Send(spawnPos); err != nil {
		return err
	}

	chunkX := chunkCoord(cfg.SpawnX)
	chunkZ := chunkCoord(cfg.SpawnZ)
	centerChunk := &playpacket.SetCenterChunk{
		ChunkX: chunkX,
		ChunkZ: chunkZ,
	}
	if err := s.Send(centerChunk); err != nil {
		return err
	}

	syncPos := &playpacket.SyncPlayerPosition{
		TeleportID: teleportID,
		X:          cfg.SpawnX,
		Y:          cfg.SpawnY,
		Z:          cfg.SpawnZ,
		VelocityX:  0,
		VelocityY:  0,
		VelocityZ:  0,
		Yaw:        cfg.SpawnYaw,
		Pitch:      cfg.SpawnPitch,
		Flags:      0,
	}
	if err := s.Send(syncPos); err != nil {
		return err
	}
	if ds, ok := s.(*DefaultSession); ok {
		ds.setPendingTeleport(teleportID)
	}
	logger.Debug("sent sync_player_position", "session", s.ID(), "teleport_id", teleportID)

	startWaiting := &playpacket.GameEvent{
		Event: 13,
		Value: 0,
	}
	if err := s.Send(startWaiting); err != nil {
		return err
	}

	logger.Debug("play bootstrap complete", "session", s.ID())

	return nil
}

func handleOpenContainer(s Session, pm *PlayerManager, wa WorldAccessor, clickedState int32, windowType int32, title string, slotCount int, bx, by, bz int, sequence int32) error {
	if pm == nil {
		return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
	}
	op := pm.GetBySession(s)
	if op == nil || op.Windows == nil {
		return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
	}

	actualWindowType := windowType
	actualSlotCount := slotCount
	actualTitle := title

	var w *inventory.Window
	if behavior.IsChest(clickedState) && wa != nil && wa.BlockEntities() != nil {
		if ox, oy, oz, paired := behavior.FindDoubleChestHalf(clickedState, bx, by, bz); paired {
			otherState := wa.GetBlock(ox, oy, oz)
			left := block.PropertiesFromState(clickedState)["type"] == "left"
			otherType := "left"
			if left {
				otherType = "right"
			}
			if block.NameFromState(otherState) == block.NameFromState(clickedState) && block.PropertiesFromState(otherState)["type"] == otherType {
				typeName := block.NameFromState(clickedState)
				first := wa.BlockEntities().Container(bx, by, bz, typeName, 27)
				second := wa.BlockEntities().Container(ox, oy, oz, typeName, 27)
				if first != nil && second != nil {
					if left {
						w = op.Windows.OpenDoubleChest("Large Chest", first, second)
					} else {
						w = op.Windows.OpenDoubleChest("Large Chest", second, first)
					}
					actualWindowType, actualTitle = inventory.WindowTypeGeneric9x6, "Large Chest"
					w.OtherX, w.OtherY, w.OtherZ = ox, oy, oz
				}
			}
		}
	}
	if w == nil {
		var container *inventory.Container
		if actualWindowType == inventory.WindowTypeCrafting || actualWindowType == inventory.WindowTypeStonecutter {
			container = inventory.NewContainer(actualSlotCount)
		} else if wa != nil && wa.BlockEntities() != nil {
			typeName := block.NameFromState(clickedState)
			if strings.Contains(typeName, "shulker_box") {
				typeName = "minecraft:shulker_box"
			}
			container = wa.BlockEntities().Container(bx, by, bz, typeName, actualSlotCount)
		}
		if container == nil {
			return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
		}
		w = op.Windows.OpenWindow(actualWindowType, actualTitle, container)
	}
	w.BlockX, w.BlockY, w.BlockZ = bx, by, bz

	if err := s.Send(&playpacket.OpenScreen{
		WindowID:   w.ID,
		WindowType: actualWindowType,
		Title:      actualTitle,
	}); err != nil {
		return err
	}

	slots := w.Container.Slots()
	stateID := op.Windows.StateID()
	if err := s.Send(&playpacket.SetContainerContent{
		WindowID: w.ID,
		StateID:  stateID,
		Slots:    slots,
		Carried:  op.Windows.CursorItem(),
	}); err != nil {
		return err
	}
	if actualWindowType == inventory.WindowTypeStonecutter {
		if err := sendStonecuttingSelection(s, op.Windows, w.ID); err != nil {
			return err
		}
	}

	// Send chest open animation via BlockAction
	if behavior.IsChest(clickedState) && pm != nil {
		viewerCount := byte(0)
		for _, viewer := range pm.Players() {
			if viewer != nil && viewer.Windows != nil && sameOpenBlockContainer(viewer.Windows.ActiveWindow(), w) && viewerCount < 255 {
				viewerCount++
			}
		}
		pm.Broadcast(&playpacket.BlockAction{
			Position:        playpacket.BlockPos{X: int32(bx), Y: int32(by), Z: int32(bz)},
			ActionID:        1, // action: number of players viewing
			ActionParameter: viewerCount,
			BlockType:       block.BlockIDFromState(clickedState),
		})
		if w.DoubleChest {
			otherState := wa.GetBlock(w.OtherX, w.OtherY, w.OtherZ)
			pm.Broadcast(&playpacket.BlockAction{
				Position: playpacket.BlockPos{X: int32(w.OtherX), Y: int32(w.OtherY), Z: int32(w.OtherZ)},
				ActionID: 1, ActionParameter: viewerCount, BlockType: block.BlockIDFromState(otherState),
			})
		}
	}

	return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
}

func handleOpenFurnace(s Session, pm *PlayerManager, wa WorldAccessor, furnaceType int, windowType int32, title string, x, y, z int32, sequence int32) error {
	if pm == nil {
		return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
	}
	op := pm.GetBySession(s)
	if op == nil || op.Windows == nil {
		return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
	}

	typeName := "minecraft:furnace"
	var container *inventory.Container
	if wa != nil && wa.BlockEntities() != nil {
		switch furnaceType {
		case behavior.FurnaceTypeBlast:
			typeName = "minecraft:blast_furnace"
		case behavior.FurnaceTypeSmoker:
			typeName = "minecraft:smoker"
		}
		container = wa.BlockEntities().Container(int(x), int(y), int(z), typeName, 3)
	}
	if container == nil {
		return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
	}
	w := op.Windows.OpenWindow(windowType, title, container)
	w.BlockX, w.BlockY, w.BlockZ = int(x), int(y), int(z)

	if err := s.Send(&playpacket.OpenScreen{
		WindowID:   w.ID,
		WindowType: windowType,
		Title:      title,
	}); err != nil {
		return err
	}

	slots := w.Container.Slots()
	stateID := op.Windows.StateID()
	if err := s.Send(&playpacket.SetContainerContent{
		WindowID: w.ID,
		StateID:  stateID,
		Slots:    slots,
		Carried:  op.Windows.CursorItem(),
	}); err != nil {
		return err
	}

	furnace := wa.BlockEntities().Furnace(int(x), int(y), int(z), typeName)
	for property := 0; property < 4; property++ {
		if err := s.Send(&playpacket.SetContainerProperty{
			WindowID: w.ID, Property: int16(property), Value: furnace.GetProperty(property),
		}); err != nil {
			return err
		}
	}

	return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
}

// handleRespawn restores a dead player at a prepared, collision-safe position.
func handleRespawn(s Session, cfg PlayBootstrapConfig, pm *PlayerManager, wa WorldAccessor) error {
	if cfg.RequestRespawn != nil {
		return cfg.RequestRespawn(s)
	}
	player, ok := s.Player().(*entity.Player)
	if !ok || player == nil || !player.Living().IsDead() {
		return fmt.Errorf("respawn requested while player is alive")
	}
	gameMode := player.Living().GameMode()
	spawnX, spawnY, spawnZ := cfg.SpawnX, cfg.SpawnY, cfg.SpawnZ
	spawnAngle := cfg.SpawnYaw
	spawnResolved := false
	if cfg.ReadWorldSpawn != nil {
		x, y, z, angle := cfg.ReadWorldSpawn()
		spawnX, spawnY, spawnZ, spawnAngle = float64(x)+0.5, float64(y), float64(z)+0.5, angle
	}
	if err := prepareRespawnChunks(wa, spawnX, spawnZ); err != nil {
		return err
	}
	if pm != nil {
		if op := pm.GetBySession(s); op != nil && op.HasSpawnPoint {
			if err := prepareRespawnChunks(wa, float64(op.SpawnX), float64(op.SpawnZ)); err != nil {
				return err
			}
			if x, y, z, ok := playerRespawnPoint(wa, op); ok {
				spawnAngle = op.SpawnAngle
				spawnX, spawnY, spawnZ = x, y, z
				spawnResolved = true
			} else {
				op.HasSpawnPoint = false
				_ = s.Send(&playpacket.SystemChatMessage{Text: "Your home bed was missing or obstructed", Overlay: false})
			}
		}
	}
	if accessor, ok := wa.(*DefaultWorldAccessor); ok && accessor.Chunks != nil && !spawnResolved {
		x, y, z, ok := safeWorldRespawn(wa, int(math.Floor(spawnX)), int(spawnY), int(math.Floor(spawnZ)))
		if !ok {
			return errRespawnObstructed
		}
		spawnX, spawnY, spawnZ = x, y, z
	}
	dimension := cfg.DimensionName
	if dimension == "" {
		dimension = "minecraft:overworld"
	}
	dimType := dimensionTypeID(cfg.RegistryManager, dimension)

	respawnPkt := &playpacket.Respawn{
		DimensionType:    dimType,
		DimensionName:    dimension,
		HashedSeed:       0,
		GameMode:         byte(gameMode),
		PreviousGameMode: -1,
		IsDebug:          false,
		IsFlat:           false,
		HasDeathLocation: false,
		PortalCooldown:   0,
		SeaLevel:         63,
		DataKept:         0,
	}
	if err := s.Send(respawnPkt); err != nil {
		return err
	}
	chunkX, chunkZ := chunkCoord(spawnX), chunkCoord(spawnZ)
	if err := s.Send(&playpacket.SetCenterChunk{ChunkX: chunkX, ChunkZ: chunkZ}); err != nil {
		return err
	}

	teleportID := nextTeleportID()
	if err := s.Send(&playpacket.SyncPlayerPosition{
		TeleportID: teleportID,
		X:          spawnX,
		Y:          spawnY,
		Z:          spawnZ,
		Yaw:        spawnAngle,
	}); err != nil {
		return err
	}
	if ds, ok := s.(*DefaultSession); ok {
		ds.setPendingTeleport(teleportID)
	}
	if err := s.Send(&playpacket.GameEvent{Event: 13}); err != nil {
		return err
	}
	player.SetClientLoaded(false)

	if err := s.Send(&playpacket.UpdateHealth{Health: 20.0, Food: 20, FoodSaturation: 5.0}); err != nil {
		return err
	}
	speed := 0.0
	if pm != nil {
		if online := pm.GetBySession(s); online != nil {
			speed = online.SpeedBlocksPerSecond
		}
	}
	if ds, ok := s.(*DefaultSession); ok {
		ds.SessionData().Store("player_flying", false)
	}
	if err := s.Send(playerAbilitiesWithSpeed(gameMode, speed, false)); err != nil {
		return err
	}
	if err := s.Send(&playpacket.EntityUpdateAttributes{EntityID: player.ID(), Properties: player.Attributes().ToProperties()}); err != nil {
		return err
	}
	keepExperience := cfg.KeepInventory != nil && cfg.KeepInventory()
	if pm != nil {
		if online := pm.GetBySession(s); online != nil && online.DeathProcessed {
			keepExperience = online.DeathKeepInventory
		}
	}
	player.Living().RespawnWithKeepExperience(keepExperience)
	if err := s.Send(&playpacket.SetExperience{ExperienceBar: player.Living().XPBar(), ExperienceLevel: player.Living().XPLevel(), TotalExperience: player.Living().XPTotal()}); err != nil {
		return err
	}

	if pm != nil {
		if op := pm.GetBySession(s); op != nil {
			op.X, op.Y, op.Z = spawnX, spawnY, spawnZ
			op.movement = movementState{}
			op.Flags &^= 0x80
			op.Pose = entitypose.PoseStanding
			op.DeathProcessed = false
			op.RespawnPending = false
			if op.Windows != nil {
				if err := sendWindowResync(s, op.Windows, 0); err != nil {
					return err
				}
			}
		}
	}

	player.SetPosition(entity.Vec3{X: spawnX, Y: spawnY, Z: spawnZ})
	player.SetRotationYP(spawnAngle, 0)
	if wa != nil {
		wa.ResendPlayerChunks(player.ID(), chunkX, chunkZ)
	}

	logger.Debug("respawned", "session", s.ID(), "x", spawnX, "y", spawnY, "z", spawnZ)
	return nil
}

// safeBedRespawn returns an unobstructed location adjacent to a valid bed head.
func safeBedRespawn(wa WorldAccessor, x, y, z int) (float64, float64, float64, bool) {
	if wa == nil {
		return 0, 0, 0, false
	}
	head := wa.GetBlock(x, y, z)
	if !behavior.IsBed(head) || block.PropertiesFromState(head)["part"] != "head" {
		return 0, 0, 0, false
	}
	ox, oy, oz := behavior.BedOtherHalfPos(head, x, y, z)
	foot := wa.GetBlock(ox, oy, oz)
	if block.NameFromState(foot) != block.NameFromState(head) || block.PropertiesFromState(foot)["part"] != "foot" || block.PropertiesFromState(foot)["facing"] != block.PropertiesFromState(head)["facing"] {
		return 0, 0, 0, false
	}
	for _, offset := range [][2]int{{0, 0}, {1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, -1}, {1, -1}, {-1, 1}} {
		px, pz := x+offset[0], z+offset[1]
		for _, py := range []int{y + 1, y} {
			if sx, sy, sz, valid := standingSpawnAt(wa, px, py, pz); valid {
				return sx, sy, sz, true
			}
		}
	}
	return 0, 0, 0, false
}

func handleAttack(s Session, targetEntityID int32, pm *PlayerManager) error {
	attacker := pm.GetBySession(s)
	if attacker == nil {
		return nil
	}
	attackerPlayer, ok := s.Player().(*entity.Player)
	if !ok || attackerPlayer == nil {
		return nil
	}
	attackerLiving := attackerPlayer.Living()
	if attackerLiving.IsDead() || attackerLiving.GameMode() == 3 || attacker.EntityID == targetEntityID {
		return nil
	}

	target := pm.GetByEntityID(targetEntityID)
	if target == nil {
		return nil
	}
	targetPlayer, ok := target.Session.Player().(*entity.Player)
	if !ok || targetPlayer == nil {
		return nil
	}
	targetLiving := targetPlayer.Living()
	if targetLiving.IsDead() {
		return nil
	}
	if targetLiving.GameMode() == 1 || targetLiving.GameMode() == 3 {
		return nil
	}
	attackerPos := attackerPlayer.Position()
	targetPos := targetPlayer.Position()
	reachDX := attackerPos.X - targetPos.X
	reachDY := attackerPos.Y - targetPos.Y
	reachDZ := attackerPos.Z - targetPos.Z
	maxReach := 3.0
	if attackerLiving.GameMode() == 1 {
		maxReach = 5.0
	}
	if reachDX*reachDX+reachDY*reachDY+reachDZ*reachDZ > maxReach*maxReach {
		return fmt.Errorf("attack target out of reach")
	}

	attackerWeapon := getHeldItemName(pm, s)
	isSword := strings.HasSuffix(attackerWeapon, "_sword")
	heldWeapon := inventory.ItemStack{}
	if attacker.Windows != nil {
		heldWeapon = attacker.Windows.HeldItem()
	}
	baseDamage, attackSpeedTicks, cooldownProgress := playerAttackDamage(attackerLiving, heldWeapon)
	critical := cooldownProgress > 0.9 && !attackerLiving.OnGround() && attackerLiving.FallDistance() > 0

	var totalDefense, totalToughness float64
	var enchProtection float64
	if target.Windows != nil {
		for slot := inventory.ArmorStart; slot <= inventory.ArmorEnd; slot++ {
			as := target.Windows.GetSlot(slot)
			if !as.Empty() {
				armorName := item.NameByID(as.ItemID)
				if ap := equipment.GetArmor(armorName); ap != nil {
					totalDefense += ap.Defense
					totalToughness += ap.Toughness
				}
				el := slotEnchantList(as)
				enchProtection += enchantment.ProtectionFactor(el)
			}
		}
	}
	if enchProtection > 20 {
		enchProtection = 20
	}

	// Shield blocking check
	isAxe := strings.HasSuffix(attackerWeapon, "_axe")
	if targetLiving.IsBlocking() && targetLiving.ShieldCooldown() <= 0 {
		// Check if attack is from the front 180° arc
		attackerPos := attackerPlayer.Position()
		targetPos := targetPlayer.Position()
		targetYaw := targetPlayer.Rotation().X
		dx := attackerPos.X - targetPos.X
		dz := attackerPos.Z - targetPos.Z
		yaw := float64(targetYaw) * math.Pi / 180
		if dx*-math.Sin(yaw)+dz*math.Cos(yaw) > 0 {
			// Attack blocked by shield
			if isAxe {
				// Axe disables shield for 5 seconds (100 ticks)
				targetLiving.SetShieldCooldown(100)
				targetLiving.SetBlocking(false)
			}
			// Play shield block sound
			pm.Broadcast(&playpacket.SoundEffect{
				SoundID:       gensound.SoundIDByName("item.shield.block") + 1,
				SoundCategory: soundcategory.CategoryPlayer,
				X:             int32(target.X * 8),
				Y:             int32(target.Y * 8),
				Z:             int32(target.Z * 8),
				Volume:        1.0,
				Pitch:         1.0,
			})
			attackerLiving.ResetAttackCooldown(attackSpeedTicks)
			return nil
		}
	}

	source := entity.NewAttackDamageSource(entity.DamageTypePlayerAttack, baseDamage, attacker.EntityID)
	source.AttackerType = "minecraft:player"
	if attacker.Windows != nil {
		source.Weapon = attacker.Windows.HeldItem().Clone()
	}
	actual := targetLiving.ApplyDamage(source, totalDefense, totalToughness, enchProtection)
	if actual <= 0 {
		return nil
	}

	attackerLiving.AddExhaustion(0.1)
	attackerLiving.ResetAttackCooldown(attackSpeedTicks)

	if attacker.Windows != nil {
		held := attacker.Windows.HeldItem()
		if !held.Empty() && item.IsDurable(held.ItemID) {
			unbreaking := held.EnchantLevel(item.EnchantUnbreakingID)
			if item.ShouldConsumeDurability(unbreaking, false) {
				newDmg, broken := item.ApplyDamage(held.ItemID, held.Damage, 1)
				heldIdx := inventory.HotbarStart + int(attacker.Windows.HeldSlot())
				if broken {
					attacker.Windows.Inventory().Clear(heldIdx)
					_ = sendWindowResync(s, attacker.Windows, 0)
				} else {
					held.SetDamage(newDmg)
					attacker.Windows.Inventory().Set(heldIdx, held)
					_ = sendWindowResync(s, attacker.Windows, 0)
				}
			}
		}
	}

	if isSword && cooldownProgress > 0.9 {
		sweepDamage := float32(1.0)
		for _, op := range pm.Players() {
			if op.EntityID == targetEntityID || op.EntityID == attacker.EntityID {
				continue
			}
			dx := op.X - target.X
			dz := op.Z - target.Z
			if dx*dx+dz*dz < 9.0 {
				sweepPlayer, ok := op.Session.Player().(*entity.Player)
				if !ok || sweepPlayer == nil {
					continue
				}
				sweepLiving := sweepPlayer.Living()
				if sweepLiving.IsDead() || sweepLiving.GameMode() == 1 || sweepLiving.GameMode() == 3 {
					continue
				}
				sweepSource := *source
				sweepSource.Type, sweepSource.Amount, sweepSource.PlayerCredit = entity.DamageTypeSweep, sweepDamage, true
				sweepLiving.ApplyDamage(&sweepSource, sweepLiving.ArmorDefense(), sweepLiving.ArmorToughness(), sweepLiving.ArmorEnchProtection())
				_ = op.Session.Send(&playpacket.UpdateHealth{
					Health:         sweepLiving.Health(),
					Food:           sweepLiving.FoodLevel(),
					FoodSaturation: sweepLiving.FoodSaturation(),
				})
				pm.Broadcast(&playpacket.HurtAnimation{
					EntityID: op.EntityID,
					Yaw:      attackerPlayer.Rotation().X,
				})
			}
		}
		pm.Broadcast(&playpacket.WorldParticles{
			ParticleID:    genparticle.ParticleSweepAttack,
			LongDistance:  true,
			X:             target.X,
			Y:             target.Y + 0.5,
			Z:             target.Z,
			ParticleCount: 1,
		})
	}

	_ = target.Session.Send(&playpacket.UpdateHealth{
		Health:         targetLiving.Health(),
		Food:           targetLiving.FoodLevel(),
		FoodSaturation: targetLiving.FoodSaturation(),
	})

	pm.Broadcast(&playpacket.HurtAnimation{
		EntityID: targetEntityID,
		Yaw:      attackerPlayer.Rotation().X,
	})

	pm.Broadcast(&playpacket.SoundEffect{
		SoundID:       gensound.SoundIDByName("entity.player.hurt") + 1,
		SoundCategory: soundcategory.CategoryPlayer,
		X:             int32(target.X * 8),
		Y:             int32(target.Y * 8),
		Z:             int32(target.Z * 8),
		Volume:        1.0,
		Pitch:         1.0,
	})

	pm.Broadcast(&playpacket.WorldParticles{
		ParticleID:    genparticle.ParticleDamageIndicator,
		LongDistance:  true,
		X:             target.X,
		Y:             target.Y + 1.0,
		Z:             target.Z,
		OffsetX:       0.3,
		OffsetY:       0.3,
		OffsetZ:       0.3,
		MaxSpeed:      0.0,
		ParticleCount: 5,
	})

	if critical {
		pm.Broadcast(&playpacket.EntityAnimation{
			EntityID:  attacker.EntityID,
			Animation: playpacket.AnimationCritical,
		})
		pm.Broadcast(&playpacket.WorldParticles{
			ParticleID:    genparticle.ParticleCrit,
			LongDistance:  true,
			X:             target.X,
			Y:             target.Y + 1.0,
			Z:             target.Z,
			OffsetX:       0.5,
			OffsetY:       0.5,
			OffsetZ:       0.5,
			MaxSpeed:      0.1,
			ParticleCount: 10,
		})
	}

	dx := target.X - attacker.X
	dz := target.Z - attacker.Z
	dist := math.Sqrt(dx*dx + dz*dz)
	if dist > 0.001 {
		dx /= dist
		dz /= dist
	} else {
		dx = 0
		dz = 1
	}
	kbStrength := 0.4
	if attacker.Windows != nil {
		held := attacker.Windows.HeldItem()
		if !held.Empty() {
			kbLevel := enchantment.KnockbackLevel(slotEnchantList(held))
			kbStrength += float64(kbLevel) * 0.4
		}
	}
	vx := int16(dx * kbStrength * 8000)
	vy := int16(0.4 * 8000)
	vz := int16(dz * kbStrength * 8000)
	_ = target.Session.Send(&playpacket.EntityVelocity{
		EntityID:  targetEntityID,
		VelocityX: vx,
		VelocityY: vy,
		VelocityZ: vz,
	})

	pm.BroadcastExcept(attacker.UUID, &playpacket.EntityAnimation{
		EntityID:  attacker.EntityID,
		Animation: playpacket.AnimationSwingMainArm,
	})

	if targetLiving.IsDead() {
		deathMsg := targetPlayer.Username() + " was slain by " + attackerPlayer.Username()
		_ = target.Session.Send(&playpacket.DeathCombatEvent{
			PlayerID: targetEntityID,
			Message:  deathMsg,
		})
		pm.Broadcast(&playpacket.EntityStatus{
			EntityID: targetEntityID,
			Status:   entitystatus.StatusDeath,
		})
		logger.Info("player killed", "message", deathMsg)
	}

	return nil
}

func dimensionTypeID(mgr *registry.Manager, name string) int32 {
	if mgr == nil {
		return 0
	}
	id := mgr.IDByName("minecraft:dimension_type", name)
	if id < 0 {
		return 0
	}
	return id
}

func breakBlock(s Session, pm *PlayerManager, wa WorldAccessor, pos playpacket.BlockPos, gm int32, canHarvest bool) {
	if wa == nil {
		return
	}
	bx, by, bz := int(pos.X), int(pos.Y), int(pos.Z)
	oldState := wa.GetBlock(bx, by, bz)
	if oldState <= 0 {
		_ = resyncBlock(s, wa, pos)
		return
	}
	var toolAtBreak inventory.Slot
	var online *OnlinePlayer
	if pm != nil {
		online = pm.GetBySession(s)
		if online != nil && online.Windows != nil {
			toolAtBreak = online.Windows.HeldItem()
		}
	}

	if gm != 1 {
		if p, ok := s.Player().(*entity.Player); ok && p != nil {
			p.Living().AddExhaustion(0.005)
		}
		if online != nil && online.Windows != nil {
			held := toolAtBreak.Clone()
			if !held.Empty() && item.IsDurable(held.ItemID) {
				unbreaking := held.EnchantLevel(item.EnchantUnbreakingID)
				if item.ShouldConsumeDurability(unbreaking, false) {
					newDmg, broken := item.ApplyDamage(held.ItemID, held.Damage, 1)
					heldIdx := inventory.HotbarStart + int(online.Windows.HeldSlot())
					if broken {
						online.Windows.Inventory().Clear(heldIdx)
						_ = sendWindowResync(s, online.Windows, 0)
					} else {
						held.SetDamage(newDmg)
						online.Windows.Inventory().Set(heldIdx, held)
						_ = sendWindowResync(s, online.Windows, 0)
					}
				}
			}
		}
	}

	if wa.BlockEntities() != nil {
		_, _, _, isContainer := behavior.IsContainer(oldState)
		_, _, _, isFurnace := behavior.IsFurnace(oldState)
		if isContainer || isFurnace {
			for _, stack := range wa.BlockEntities().RemoveContainer(bx, by, bz) {
				if !stack.Empty() {
					spawnInventoryItem(pm, wa, float64(bx), float64(by), float64(bz), stack)
				}
			}
		}
	}
	wa.SetBlock(bx, by, bz, 0)
	if behavior.IsBed(oldState) {
		ox, oy, oz := behavior.BedOtherHalfPos(oldState, bx, by, bz)
		other := wa.GetBlock(ox, oy, oz)
		if block.NameFromState(other) == block.NameFromState(oldState) && block.PropertiesFromState(other)["part"] != block.PropertiesFromState(oldState)["part"] {
			wa.SetBlock(ox, oy, oz, 0)
			update := &playpacket.BlockUpdate{Position: playpacket.BlockPos{X: int32(ox), Y: int32(oy), Z: int32(oz)}, BlockID: 0}
			if pm != nil {
				pm.Broadcast(update)
			} else {
				_ = s.Send(update)
			}
			wa.NotifyNeighbors(ox, oy, oz)
		}
	}

	var drops []behavior.Drop
	if oldState > 0 && canHarvest {
		ctx := &behavior.Context{
			X: bx, Y: by, Z: bz,
			StateID: oldState, PlayerGM: gm,
		}
		if !toolAtBreak.Empty() {
			ctx.ToolName = item.NameByID(toolAtBreak.ItemID)
			if len(toolAtBreak.Enchantments) > 0 {
				ctx.ToolEnchantments = toolAtBreak.Enchantments
			}
		}
		drops = behavior.GetByState(oldState).OnBreak(ctx)
	}

	blockUpdate := &playpacket.BlockUpdate{Position: pos, BlockID: 0}
	if err := s.Send(blockUpdate); err != nil {
		logger.Warn("block update send failed after break", "session", s.ID(), "error", err)
	}

	if pm != nil {
		if op := pm.GetBySession(s); op != nil {
			pm.BroadcastExcept(op.UUID, blockUpdate)
			if oldState > 0 {
				pm.BroadcastExcept(op.UUID, &playpacket.WorldEvent{
					Event: worldevent.EventPARTICLESDESTROYBLOCK,
					X:     int32(bx), Y: int32(by), Z: int32(bz),
					Data: oldState,
				})
			}
		}
	}

	wa.NotifyNeighbors(bx, by, bz)

	for _, drop := range drops {
		ie := wa.SpawnItemDrop(float64(bx), float64(by), float64(bz), drop.ItemID, drop.Count)
		if ie == nil || pm == nil {
			continue
		}
		if worldOwnsItemTracking(wa) {
			continue
		}
		ipos := ie.Position()
		vel := ie.Velocity()
		spawnPkt := &playpacket.SpawnEntity{
			EntityID:   ie.ID(),
			EntityUUID: ie.UUID(),
			Type:       genentity.EntityItem,
			X:          ipos.X,
			Y:          ipos.Y,
			Z:          ipos.Z,
			VelocityX:  velocityToProtocol(vel.X),
			VelocityY:  velocityToProtocol(vel.Y),
			VelocityZ:  velocityToProtocol(vel.Z),
		}
		pm.Broadcast(spawnPkt)

		md := metadata.New()
		md.SetSlot(metadata.IndexItemEntityItem, ie.Stack())
		pm.Broadcast(&playpacket.SetEntityMetadata{
			EntityID: ie.ID(),
			Metadata: md.Encode(),
		})
	}
}

type windowClickCommand struct {
	session Session
	players *PlayerManager
	world   WorldAccessor
	packet  *playpacket.WindowClick
}

// Execute applies one inventory transaction on the authoritative world tick.
func (c *windowClickCommand) Execute() {
	if c.session == nil || c.session.Context().Err() != nil {
		return
	}
	op := c.players.GetBySession(c.session)
	if op == nil || op.Windows == nil {
		return
	}
	pkt := c.packet
	if active := op.Windows.ActiveWindow(); active != nil && active.ID == pkt.WindowID && active.Type == inventory.WindowTypeStonecutter && !stonecuttingMenuValid(c.session, c.world, active) {
		closeInvalidMenu(c.session, c.players, c.world, active)
		return
	}
	var furnaceOutputBefore inventory.Slot
	if active := op.Windows.ActiveWindow(); active != nil && active.ID == pkt.WindowID && pkt.Slot == 2 {
		if active.Type == inventory.WindowTypeFurnace || active.Type == inventory.WindowTypeBlastFurnace || active.Type == inventory.WindowTypeSmoker {
			furnaceOutputBefore = active.Backing.Get(2)
		}
	}
	accepted, dropped := op.Windows.HandleClickWithDrop(pkt.WindowID, pkt.StateID, pkt.Slot, pkt.Button, pkt.Mode, pkt.ChangedSlots, pkt.CarriedItem)
	if accepted && pkt.WindowID == 0 {
		op.Windows.UpdateCraftOutput()
	} else if accepted {
		if active := op.Windows.ActiveWindow(); active != nil && active.ID == pkt.WindowID && active.Type == inventory.WindowTypeCrafting {
			op.Windows.UpdateCraftTableOutput()
		}
	}
	if accepted && c.world != nil {
		for _, stack := range op.Windows.TakeCraftDrops() {
			spawnInventoryItem(c.players, c.world, op.X, op.Y+1, op.Z, stack)
		}
	}
	if accepted && !dropped.Empty() && c.world != nil {
		spawnInventoryItem(c.players, c.world, op.X, op.Y+1, op.Z, dropped)
	}
	if accepted && !furnaceOutputBefore.Empty() && pkt.Mode != inventory.ClickTypeDrop && c.world != nil && c.world.BlockEntities() != nil {
		if active := op.Windows.ActiveWindow(); active != nil && active.ID == pkt.WindowID && active.Backing.Get(2).ItemCount < furnaceOutputBefore.ItemCount {
			typeName := block.NameFromState(c.world.GetBlock(active.BlockX, active.BlockY, active.BlockZ))
			if furnace := c.world.BlockEntities().Furnace(active.BlockX, active.BlockY, active.BlockZ, typeName); furnace != nil {
				xp := furnace.ExtractExperience()
				if xp > 0 {
					amount := max(int32(1), int32(math.Round(xp)))
					if player, ok := c.session.Player().(*entity.Player); ok && player != nil {
						living := player.Living()
						updated := experience.AddXP(living.XPLevel(), living.XPTotal(), amount)
						living.SetXP(updated.Level, updated.Total, updated.Bar)
						_ = c.session.Send(&playpacket.SetExperience{ExperienceBar: updated.Bar, ExperienceLevel: updated.Level, TotalExperience: updated.Total})
						c.world.BlockEntities().MarkContainerDirty(active.BlockX, active.BlockZ)
					}
				}
			}
		}
	}
	intermediateDrag := pkt.Mode == inventory.ClickTypeDrag && (pkt.Button == 0 || pkt.Button == 1 || pkt.Button == 4 || pkt.Button == 5)
	if accepted && !intermediateDrag && pkt.WindowID != 0 && c.world != nil && c.world.BlockEntities() != nil {
		if active := op.Windows.ActiveWindow(); active != nil && active.ID == pkt.WindowID && active.Type != inventory.WindowTypeCrafting && active.Type != inventory.WindowTypeStonecutter {
			c.world.BlockEntities().MarkContainerDirty(active.BlockX, active.BlockZ)
			if active.DoubleChest {
				c.world.BlockEntities().MarkContainerDirty(active.OtherX, active.OtherZ)
			}
			for _, other := range c.players.Players() {
				if other == nil || other.UUID == op.UUID || other.Windows == nil {
					continue
				}
				view := other.Windows.ActiveWindow()
				if sameOpenBlockContainer(view, active) {
					_ = sendWindowResync(other.Session, other.Windows, view.ID)
				}
			}
		}
	}
	if !accepted || !intermediateDrag {
		_ = sendWindowResync(c.session, op.Windows, pkt.WindowID)
		if active := op.Windows.ActiveWindow(); active != nil && active.ID == pkt.WindowID && active.Type == inventory.WindowTypeStonecutter {
			if err := sendStonecuttingSelection(c.session, op.Windows, active.ID); err != nil {
				logger.Warn("sync stonecutter selection failed", "session", c.session.ID(), "error", err)
			}
		}
	}
	equipmentChanged := pkt.WindowID == 0 && IsEquipmentSlot(pkt.Slot) || pkt.Mode == inventory.ClickTypeNumberKey && (pkt.Button == 40 || pkt.Button == op.Windows.HeldSlot())
	if accepted && equipmentChanged {
		if pkt.WindowID != 0 && pkt.Button == 40 {
			if err := sendWindowResync(c.session, op.Windows, 0); err != nil {
				logger.Warn("sync offhand swap failed", "session", c.session.ID(), "error", err)
			}
		}
		UpdateArmorStats(op)
		if equipPkt := BuildFullEquipmentPacket(op.EntityID, op.Windows); equipPkt != nil {
			c.players.BroadcastExcept(op.UUID, equipPkt)
		}
	}
}

func sameOpenBlockContainer(a, b *inventory.Window) bool {
	if a == nil || b == nil || a.Type != b.Type {
		return false
	}
	if a.Type == inventory.WindowTypeCrafting || a.Type == inventory.WindowTypeStonecutter {
		return false
	}
	if a.BlockX == b.BlockX && a.BlockY == b.BlockY && a.BlockZ == b.BlockZ {
		return true
	}
	return a.DoubleChest && b.DoubleChest && a.BlockX == b.OtherX && a.BlockY == b.OtherY && a.BlockZ == b.OtherZ && a.OtherX == b.BlockX && a.OtherY == b.BlockY && a.OtherZ == b.BlockZ
}

func sendWindowResync(s Session, windows *inventory.WindowManager, windowID int32) error {
	stateID, slots, cursor := windows.SnapshotForResync(windowID)
	return sendWindowContent(s, windowID, stateID, slots, cursor)
}

func sendWindowSnapshot(s Session, windows *inventory.WindowManager, windowID int32) error {
	stateID, slots, cursor := windows.Snapshot(windowID)
	return sendWindowContent(s, windowID, stateID, slots, cursor)
}

func sendWindowContent(s Session, windowID, stateID int32, slots []inventory.Slot, cursor inventory.Slot) error {
	if slots == nil {
		return nil
	}
	return s.Send(&playpacket.SetContainerContent{
		WindowID: windowID,
		StateID:  stateID,
		Slots:    slots,
		Carried:  cursor,
	})
}

type placeBlockCommand struct {
	session  Session
	players  *PlayerManager
	world    WorldAccessor
	player   *OnlinePlayer
	x, y, z  int32
	stateID  int32
	sequence int32
}

// Execute validates destination availability again before applying a queued placement.
func (c *placeBlockCommand) Execute() {
	if c.session == nil || c.session.Context().Err() != nil {
		return
	}
	if c.world == nil || c.world.GetBlock(int(c.x), int(c.y), int(c.z)) < 0 {
		_ = c.session.Send(&playpacket.AcknowledgeBlockChange{Sequence: c.sequence})
		return
	}
	if behavior.IsBed(c.stateID) {
		if !c.placeBed() {
			_ = c.session.Send(&playpacket.AcknowledgeBlockChange{Sequence: c.sequence})
			return
		}
	} else {
		c.world.SetBlock(int(c.x), int(c.y), int(c.z), c.stateID)
	}
	update := &playpacket.BlockUpdate{
		Position: playpacket.BlockPos{X: c.x, Y: c.y, Z: c.z},
		BlockID:  c.stateID,
	}
	_ = c.session.Send(update)

	if c.player != nil {
		if player, ok := c.session.Player().(*entity.Player); ok && player != nil && player.Living().GameMode() != 1 {
			c.player.Windows.ConsumeHeldItem()
			_ = c.session.Send(&playpacket.SetContainerSlot{
				WindowID: 0,
				StateID:  c.player.Windows.CurrentStateID(),
				SlotIdx:  int16(inventory.HotbarStart + int(c.player.Windows.HeldSlot())),
				SlotData: c.player.Windows.HeldItem(),
			})
		}
		c.players.BroadcastExcept(c.player.UUID, update)
		c.broadcastSound()
	}

	c.world.ScheduleTicksForBlock(int(c.x), int(c.y), int(c.z), c.stateID)
	c.world.NotifyNeighbors(int(c.x), int(c.y), int(c.z))
	if strings.Contains(block.NameFromState(c.stateID), "_sign") {
		if entities := c.world.BlockEntities(); entities != nil {
			entities.SetSign(int(c.x), int(c.y), int(c.z), &blockentity.SignData{})
		}
		_ = c.session.Send(&playpacket.OpenSignEditor{X: c.x, Y: c.y, Z: c.z, IsFront: true})
	}
	_ = c.session.Send(&playpacket.AcknowledgeBlockChange{Sequence: c.sequence})
}

func (c *placeBlockCommand) placeBed() bool {
	footProps := block.PropertiesFromState(c.stateID)
	if footProps == nil {
		return false
	}
	footProps["part"] = "foot"
	footProps["occupied"] = "false"
	name := block.NameFromState(c.stateID)
	footState := block.StateID(name, footProps)
	if footState < 0 {
		return false
	}
	hx, hy, hz := behavior.BedHeadPos(footState, int(c.x), int(c.y), int(c.z))
	if hx == int(c.x) && hy == int(c.y) && hz == int(c.z) {
		return false
	}
	reject := func() bool {
		_ = resyncBlock(c.session, c.world, playpacket.BlockPos{X: c.x, Y: c.y, Z: c.z})
		_ = resyncBlock(c.session, c.world, playpacket.BlockPos{X: int32(hx), Y: int32(hy), Z: int32(hz)})
		return false
	}
	if _, ok := c.world.GetChunk(chunkCoord(float64(c.x)), chunkCoord(float64(c.z))); !ok {
		return reject()
	}
	if _, ok := c.world.GetChunk(chunkCoord(float64(hx)), chunkCoord(float64(hz))); !ok {
		return reject()
	}
	if !isAirForPlacement(c.world.GetBlock(int(c.x), int(c.y), int(c.z))) || !isAirForPlacement(c.world.GetBlock(hx, hy, hz)) ||
		!block.IsSolid(c.world.GetBlock(int(c.x), int(c.y)-1, int(c.z))) || !block.IsSolid(c.world.GetBlock(hx, hy-1, hz)) {
		return reject()
	}
	if c.players != nil {
		for _, op := range c.players.Players() {
			footCollision := op.X >= float64(c.x) && op.X < float64(c.x+1) && op.Z >= float64(c.z) && op.Z < float64(c.z+1) && op.Y >= float64(c.y)-1.8 && op.Y < float64(c.y)+1
			headCollision := op.X >= float64(hx) && op.X < float64(hx+1) && op.Z >= float64(hz) && op.Z < float64(hz+1) && op.Y >= float64(hy)-1.8 && op.Y < float64(hy)+1
			if footCollision || headCollision {
				return reject()
			}
		}
	}
	headProps := block.PropertiesFromState(footState)
	headProps["part"] = "head"
	headState := block.StateID(name, headProps)
	if headState < 0 {
		return reject()
	}
	c.stateID = footState
	c.world.SetBlock(int(c.x), int(c.y), int(c.z), footState)
	c.world.SetBlock(hx, hy, hz, headState)
	headUpdate := &playpacket.BlockUpdate{Position: playpacket.BlockPos{X: int32(hx), Y: int32(hy), Z: int32(hz)}, BlockID: headState}
	if c.players != nil {
		c.players.Broadcast(headUpdate)
	} else {
		_ = c.session.Send(headUpdate)
	}
	c.world.NotifyNeighbors(hx, hy, hz)
	return true
}

func (c *placeBlockCommand) broadcastSound() {
	blockName := block.NameFromState(c.stateID)
	soundName := "block.stone.place"
	switch {
	case strings.Contains(blockName, "wood"), strings.Contains(blockName, "planks"), strings.Contains(blockName, "log"):
		soundName = "block.wood.place"
	case strings.Contains(blockName, "sand"), strings.Contains(blockName, "gravel"):
		soundName = "block.sand.place"
	case strings.Contains(blockName, "grass"), strings.Contains(blockName, "dirt"):
		soundName = "block.grass.place"
	case strings.Contains(blockName, "glass"):
		soundName = "block.glass.place"
	}
	soundID := gensound.SoundIDByName(soundName)
	if soundID >= 0 {
		c.players.Broadcast(&playpacket.SoundEffect{
			SoundID: soundID + 1, SoundCategory: soundcategory.CategoryBlock,
			X: c.x*8 + 4, Y: c.y*8 + 4, Z: c.z*8 + 4, Volume: 1, Pitch: 1,
		})
	}
}

func submitPlaceBlock(s Session, pm *PlayerManager, wa WorldAccessor, player *OnlinePlayer, x, y, z, stateID, sequence int32) error {
	command := &placeBlockCommand{
		session: s, players: pm, world: wa, player: player,
		x: x, y: y, z: z, stateID: stateID, sequence: sequence,
	}
	if wa.Submit(command) {
		return nil
	}
	_ = resyncBlock(s, wa, playpacket.BlockPos{X: x, Y: y, Z: z})
	return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
}

func slotEnchantList(s inventory.Slot) *enchantment.List {
	if len(s.Enchantments) == 0 {
		return nil
	}
	entries := make([]enchantment.Entry, 0, len(s.Enchantments))
	for id, lvl := range s.Enchantments {
		entries = append(entries, enchantment.Entry{ID: id, Level: lvl})
	}
	return enchantment.FromEntries(entries)
}

func getHeldItemName(pm *PlayerManager, s Session) string {
	if pm == nil {
		return ""
	}
	op := pm.GetBySession(s)
	if op == nil || op.Windows == nil {
		return ""
	}
	held := op.Windows.HeldItem()
	if held.Empty() {
		return ""
	}
	return item.NameByID(held.ItemID)
}

func velocityToProtocol(v float64) int16 {
	val := v * 8000
	if val > 32767 {
		return 32767
	}
	if val < -32768 {
		return -32768
	}
	return int16(val)
}

func completeEating(s Session, pm *PlayerManager, p *entity.Player, tracker *food.EatingTracker) {
	es := tracker.Cancel(p.ID())
	if es == nil {
		return
	}

	living := p.Living()
	newFood, newSat := food.Eat(living.FoodLevel(), living.FoodSaturation(), es.Nutrition, es.Saturation)
	living.SetFoodLevel(newFood)
	living.SetFoodSaturation(newSat)

	if pm != nil {
		op := pm.GetBySession(s)
		if op != nil && op.Windows != nil {
			remaining := op.Windows.ConsumeHeldItem()

			_ = s.Send(&playpacket.SetContainerSlot{
				WindowID: 0,
				StateID:  op.Windows.StateID(),
				SlotIdx:  int16(inventory.HotbarStart + int(op.Windows.HeldSlot())),
				SlotData: remaining,
			})
		}
	}

	_ = s.Send(&playpacket.UpdateHealth{
		Health:         living.Health(),
		Food:           living.FoodLevel(),
		FoodSaturation: living.FoodSaturation(),
	})

	if pm != nil {
		pos := p.Position()
		pm.Broadcast(&playpacket.SoundEffect{
			SoundID:       gensound.SoundIDByName("entity.player.burp") + 1,
			SoundCategory: soundcategory.CategoryPlayer,
			X:             int32(pos.X * 8),
			Y:             int32(pos.Y * 8),
			Z:             int32(pos.Z * 8),
			Volume:        1.0,
			Pitch:         1.0,
		})
	}
}

func handleBedUse(s Session, pm *PlayerManager, wa WorldAccessor, clickedState int32, bx, by, bz int, sequence int32) error {
	if pm == nil {
		return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
	}
	op := pm.GetBySession(s)
	if op == nil {
		return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
	}
	if op.Sleeping {
		return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
	}
	if player, ok := s.Player().(*entity.Player); !ok || player == nil || player.Living().IsDead() {
		return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
	}
	if dimension := SessionDimension(s); dimension == "minecraft:the_nether" || dimension == "minecraft:the_end" {
		if explosive, ok := wa.(interface{ ExplodeBed(int, int, int) bool }); ok {
			explosive.ExplodeBed(bx, by, bz)
		}
		return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
	}
	// Find the head position of the bed
	hx, hy, hz := behavior.BedHeadPos(clickedState, bx, by, bz)

	// Check if the bed is already occupied
	headState := wa.GetBlock(hx, hy, hz)
	if !behavior.IsBed(headState) || block.NameFromState(headState) != block.NameFromState(clickedState) || block.PropertiesFromState(headState)["part"] != "head" {
		return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
	}
	fx, fy, fz := behavior.BedOtherHalfPos(headState, hx, hy, hz)
	footState := wa.GetBlock(fx, fy, fz)
	if block.NameFromState(footState) != block.NameFromState(headState) || block.PropertiesFromState(footState)["part"] != "foot" || block.PropertiesFromState(footState)["facing"] != block.PropertiesFromState(headState)["facing"] {
		return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
	}
	if headState > 0 {
		props := block.PropertiesFromState(headState)
		if props != nil && (props["occupied"] == "true" || block.PropertiesFromState(footState)["occupied"] == "true") {
			// Bed is occupied - send action bar message
			_ = s.Send(&playpacket.SystemChatMessage{
				Text:    "This bed is occupied",
				Overlay: true,
			})
			return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
		}
	}

	// Set spawn point
	changedSpawn := !op.HasSpawnPoint || op.SpawnX != hx || op.SpawnY != hy || op.SpawnZ != hz
	op.SpawnX = hx
	op.SpawnY = hy
	op.SpawnZ = hz
	op.HasSpawnPoint = true
	op.SpawnDimension, op.SpawnAngle, op.SpawnForced = SessionDimension(s), 0, false
	if changedSpawn {
		_ = s.Send(&playpacket.SystemChatMessage{Text: "Respawn point set", Overlay: false})
	}
	clock, ok := wa.(interface {
		TimeOfDay() int64
		SetTimeOfDay(int64)
	})
	phase := int64(0)
	if ok {
		phase = (clock.TimeOfDay()%24000 + 24000) % 24000
	}
	if !ok || (phase < 12542 || phase > 23458) && !sleepThunder(wa) {
		_ = s.Send(&playpacket.SystemChatMessage{Text: "You can sleep only at night", Overlay: true})
		return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
	}

	if op.GameMode == 3 || !block.IsAir(wa.GetBlock(hx, hy+1, hz)) || !block.IsAir(wa.GetBlock(fx, fy+1, fz)) {
		_ = s.Send(&playpacket.SystemChatMessage{Text: "This bed is obstructed", Overlay: true})
		return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
	}
	if threats, ok := wa.(interface{ SleepThreat(int, int, int) bool }); ok && threats.SleepThreat(hx, hy, hz) {
		_ = s.Send(&playpacket.SystemChatMessage{Text: "You may not rest now; there are monsters nearby", Overlay: true})
		return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
	}
	// Set both halves to occupied
	if headState > 0 {
		newState := behavior.BedOccupiedState(headState, true)
		wa.SetBlock(hx, hy, hz, newState)
		pm.Broadcast(&playpacket.BlockUpdate{
			Position: playpacket.BlockPos{X: int32(hx), Y: int32(hy), Z: int32(hz)},
			BlockID:  newState,
		})
	}
	if footState > 0 {
		newFootState := behavior.BedOccupiedState(footState, true)
		wa.SetBlock(fx, fy, fz, newFootState)
		pm.Broadcast(&playpacket.BlockUpdate{
			Position: playpacket.BlockPos{X: int32(fx), Y: int32(fy), Z: int32(fz)},
			BlockID:  newFootState,
		})
	}

	// Mark player as sleeping
	op.Sleeping = true
	op.SleepTicks = 0
	op.SleepX, op.SleepY, op.SleepZ = hx, hy, hz
	op.SleepBedState = headState
	if player, ok := s.Player().(*entity.Player); ok {
		position := entity.Vec3{X: float64(hx) + 0.5, Y: float64(hy) + 0.6875, Z: float64(hz) + 0.5}
		player.SetPosition(position)
		op.X, op.Y, op.Z = position.X, position.Y, position.Z
	}
	op.Pose = entitypose.PoseSleeping

	// Send sleeping metadata to all players
	md := metadata.New()
	md.SetPose(metadata.IndexPose, entitypose.PoseSleeping)
	md.SetOptPosition(metadata.IndexLivingSleepingPos, &[3]int32{int32(hx), int32(hy), int32(hz)})
	pm.Broadcast(&playpacket.SetEntityMetadata{
		EntityID: op.EntityID,
		Metadata: md.Encode(),
	})

	_ = s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})

	// Night skipping is evaluated after the required sleeping delay on the tick.

	return nil
}

// skipToDawn advances the absolute clock and applies sleep weather rules.
func skipToDawn(pm *PlayerManager, wa WorldAccessor) {
	if clock, ok := wa.(interface {
		TimeOfDay() int64
		SetTimeOfDay(int64)
	}); ok {
		clock.SetTimeOfDay(clock.TimeOfDay() + 24000 - (clock.TimeOfDay()%24000+24000)%24000)
	}
	if weather, ok := wa.(interface{ ClearSleepWeather() }); ok {
		weather.ClearSleepWeather()
	}
	// Wake all players
	for _, op := range pm.Players() {
		wakePlayer(pm, op, wa)
	}

	pm.Broadcast(&playpacket.SystemChatMessage{
		Text:    "All players sleeping. Sweet dreams!",
		Overlay: true,
	})
}

// wakePlayer releases the occupied bed and restores an unobstructed standing position.
func wakePlayer(pm *PlayerManager, op *OnlinePlayer, wa WorldAccessor) {
	if op == nil || !op.Sleeping {
		return
	}
	op.Sleeping = false
	op.SleepTicks = 0
	op.Pose = entitypose.PoseStanding
	md := metadata.New()
	md.SetPose(metadata.IndexPose, entitypose.PoseStanding)
	md.SetOptPosition(metadata.IndexLivingSleepingPos, nil)
	pm.Broadcast(&playpacket.SetEntityMetadata{EntityID: op.EntityID, Metadata: md.Encode()})
	if wa != nil {
		saved := op.SleepBedState
		if !behavior.IsBed(saved) {
			saved = wa.GetBlock(op.SleepX, op.SleepY, op.SleepZ)
		}
		if behavior.IsBed(saved) {
			ox, oy, oz := behavior.BedOtherHalfPos(saved, op.SleepX, op.SleepY, op.SleepZ)
			for _, pos := range [][3]int{{op.SleepX, op.SleepY, op.SleepZ}, {ox, oy, oz}} {
				state := wa.GetBlock(pos[0], pos[1], pos[2])
				if block.NameFromState(state) != block.NameFromState(saved) || block.PropertiesFromState(state)["facing"] != block.PropertiesFromState(saved)["facing"] {
					continue
				}
				state = behavior.BedOccupiedState(state, false)
				wa.SetBlock(pos[0], pos[1], pos[2], state)
				pm.Broadcast(&playpacket.BlockUpdate{Position: playpacket.BlockPos{X: int32(pos[0]), Y: int32(pos[1]), Z: int32(pos[2])}, BlockID: state})
			}
		}
	}
	op.SleepBedState = 0
	if op.Session == nil || op.Session.Context().Err() != nil {
		return
	}
	player, ok := op.Session.Player().(*entity.Player)
	if !ok || player == nil || player.Living().IsDead() {
		return
	}
	x, y, z, valid := safeBedRespawn(wa, op.SleepX, op.SleepY, op.SleepZ)
	if !valid {
		x, y, z, valid = safeLocalRespawn(wa, op.SleepX, op.SleepY+1, op.SleepZ)
	}
	if !valid {
		return
	}
	player.SetPosition(entity.Vec3{X: x, Y: y, Z: z})
	op.X, op.Y, op.Z = x, y, z
	id := nextTeleportID()
	if ds, ok := op.Session.(*DefaultSession); ok {
		ds.setPendingTeleport(id)
	}
	if err := op.Session.Send(&playpacket.SyncPlayerPosition{TeleportID: id, X: x, Y: y, Z: z, Yaw: op.Yaw, Pitch: op.Pitch}); err != nil {
		logger.Warn("send wake position failed", "player", op.Name, "error", err)
	}
}
