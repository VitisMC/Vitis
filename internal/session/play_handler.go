package session

import (
	"math"
	"strings"
	"sync/atomic"
	"time"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/block/behavior"
	"github.com/vitismc/vitis/internal/command"
	genentity "github.com/vitismc/vitis/internal/data/generated/entity"
	entitystatus "github.com/vitismc/vitis/internal/data/generated/entity_status"
	genparticle "github.com/vitismc/vitis/internal/data/generated/particle"
	gensound "github.com/vitismc/vitis/internal/data/generated/sound"
	soundcategory "github.com/vitismc/vitis/internal/data/generated/sound_category"
	worldevent "github.com/vitismc/vitis/internal/data/generated/world_event"
	"github.com/vitismc/vitis/internal/enchantment"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/metadata"
	"github.com/vitismc/vitis/internal/equipment"
	"github.com/vitismc/vitis/internal/food"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/logger"
	"github.com/vitismc/vitis/internal/mining"
	"github.com/vitismc/vitis/internal/operator"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/registry"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/terrain"
)

var teleportIDCounter atomic.Int32
var keepAliveValueCounter atomic.Int64

const playKeepAliveStartedKey = "play_keepalive_started"

func nextTeleportID() int32 {
	return teleportIDCounter.Add(1)
}

func nextKeepAliveValue() int64 {
	return keepAliveValueCounter.Add(1)
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
		if err := s.Send(&playpacket.KeepAliveClientbound{Value: nextKeepAliveValue()}); err != nil {
			return
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-s.Context().Done():
				return
			case <-ticker.C:
				if err := s.Send(&playpacket.KeepAliveClientbound{Value: nextKeepAliveValue()}); err != nil {
					return
				}
			}
		}
	}()
}

// SpawnChunkProvider generates chunk packet payloads for the initial spawn area.
type SpawnChunkProvider interface {
	GenerateSpawnChunk(cx, cz int32) []byte
}

// PlayBootstrapConfig holds configuration for the play state bootstrap sequence.
type PlayBootstrapConfig struct {
	ViewDistance       int32
	SimulationDistance int32
	SpawnX             float64
	SpawnY             float64
	SpawnZ             float64
	GameMode           int32
	RegistryManager    *registry.Manager
	CommandRegistry    *command.Registry
	OperatorList       *operator.List
	PlayerUUID         protocol.UUID
	PlayerName         string
	TabHeader          string
	TabFooter          string
	SpawnChunks        SpawnChunkProvider
	WorldBorderInit    protocol.Packet
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

// DefaultPlayBootstrapConfig returns sane defaults for play bootstrap.
func DefaultPlayBootstrapConfig() PlayBootstrapConfig {
	return PlayBootstrapConfig{
		ViewDistance:       10,
		SimulationDistance: 10,
		SpawnX:             0.5,
		SpawnY:             65.0,
		SpawnZ:             0.5,
	}
}

func checkFallDamage(s Session, oldY, newY float64, onGround bool) {
	p, ok := s.Player().(*entity.Player)
	if !ok || p == nil {
		return
	}
	living := p.Living()
	if living.IsDead() || living.GameMode() == 1 || living.GameMode() == 3 {
		living.SetFallDistance(0)
		return
	}

	if newY < oldY && !onGround {
		living.SetFallDistance(living.FallDistance() + (oldY - newY))
	} else if onGround {
		fd := living.FallDistance()
		living.SetFallDistance(0)
		if fd > 3.0 {
			dmg := float32(fd - 3.0)
			actual := living.Damage(dmg, "fall")
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

func chunkCoord(v float64) int32 {
	return int32(math.Floor(v / 16))
}

func sendCenterChunkIfNeeded(s Session, oldX, oldZ, newX, newZ float64) {
	oldCX := chunkCoord(oldX)
	oldCZ := chunkCoord(oldZ)
	newCX := chunkCoord(newX)
	newCZ := chunkCoord(newZ)
	if oldCX != newCX || oldCZ != newCZ {
		_ = s.Send(&playpacket.SetCenterChunk{ChunkX: newCX, ChunkZ: newCZ})
	}
}

// RegisterPlayHandlers registers handlers for Play-state packets on a packet router.
func RegisterPlayHandlers(router PacketRouter, cfg PlayBootstrapConfig, pm *PlayerManager, wa WorldAccessor) error {
	if router == nil {
		return ErrNilRegistry
	}

	confirmTeleportID := playpacket.NewConfirmTeleportation().ID()
	if err := router.Register(protocol.StatePlay, confirmTeleportID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.ConfirmTeleportation)
		if !ok {
			return protocol.ErrNilPacket
		}
		logger.Debug("confirm_teleportation", "session", s.ID(), "teleport_id", pkt.TeleportID)
		return nil
	}); err != nil {
		return err
	}

	keepAliveID := playpacket.NewKeepAliveServerbound().ID()
	if err := router.Register(protocol.StatePlay, keepAliveID, func(s Session, packet protocol.Packet) error {
		return nil
	}); err != nil {
		return err
	}

	positionID := playpacket.NewSetPlayerPosition().ID()
	if err := router.Register(protocol.StatePlay, positionID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.SetPlayerPosition)
		if !ok {
			return protocol.ErrNilPacket
		}
		mp, ok := s.Player().(MovablePlayer)
		if !ok || mp == nil {
			return nil
		}
		if pm != nil {
			if op := pm.GetBySession(s); op != nil {
				checkFallDamage(s, op.Y, pkt.Y, pkt.OnGround)
				sendCenterChunkIfNeeded(s, op.X, op.Z, pkt.X, pkt.Z)
				op.X, op.Y, op.Z = pkt.X, pkt.Y, pkt.Z
				pm.BroadcastExcept(op.UUID, &playpacket.SyncEntityPosition{
					EntityID: op.EntityID,
					X:        pkt.X,
					Y:        pkt.Y,
					Z:        pkt.Z,
					OnGround: pkt.OnGround,
				})
			}
		}
		mp.SetPositionXYZ(pkt.X, pkt.Y, pkt.Z)
		mp.SetOnGround(pkt.OnGround)
		return nil
	}); err != nil {
		return err
	}

	posRotID := playpacket.NewSetPlayerPositionAndRotation().ID()
	if err := router.Register(protocol.StatePlay, posRotID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.SetPlayerPositionAndRotation)
		if !ok {
			return protocol.ErrNilPacket
		}
		mp, ok := s.Player().(MovablePlayer)
		if !ok || mp == nil {
			return nil
		}
		if pm != nil {
			if op := pm.GetBySession(s); op != nil {
				checkFallDamage(s, op.Y, pkt.Y, pkt.OnGround)
				sendCenterChunkIfNeeded(s, op.X, op.Z, pkt.X, pkt.Z)
				op.X, op.Y, op.Z = pkt.X, pkt.Y, pkt.Z
				op.Yaw, op.Pitch = pkt.Yaw, pkt.Pitch
				pm.BroadcastExcept(op.UUID, &playpacket.SyncEntityPosition{
					EntityID: op.EntityID,
					X:        pkt.X,
					Y:        pkt.Y,
					Z:        pkt.Z,
					Yaw:      pkt.Yaw,
					Pitch:    pkt.Pitch,
					OnGround: pkt.OnGround,
				})
				pm.BroadcastExcept(op.UUID, &playpacket.SetHeadRotation{
					EntityID: op.EntityID,
					HeadYaw:  byte(int32(pkt.Yaw*256.0/360.0) & 0xFF),
				})
			}
		}
		mp.SetPositionXYZ(pkt.X, pkt.Y, pkt.Z)
		mp.SetRotationYP(pkt.Yaw, pkt.Pitch)
		mp.SetOnGround(pkt.OnGround)
		return nil
	}); err != nil {
		return err
	}

	rotID := playpacket.NewSetPlayerRotation().ID()
	if err := router.Register(protocol.StatePlay, rotID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.SetPlayerRotation)
		if !ok {
			return protocol.ErrNilPacket
		}
		mp, ok := s.Player().(MovablePlayer)
		if !ok || mp == nil {
			return nil
		}
		mp.SetRotationYP(pkt.Yaw, pkt.Pitch)
		mp.SetOnGround(pkt.OnGround)
		if pm != nil {
			if op := pm.GetBySession(s); op != nil {
				op.Yaw, op.Pitch = pkt.Yaw, pkt.Pitch
				pm.BroadcastExcept(op.UUID, &playpacket.SyncEntityPosition{
					EntityID: op.EntityID,
					X:        op.X,
					Y:        op.Y,
					Z:        op.Z,
					Yaw:      pkt.Yaw,
					Pitch:    pkt.Pitch,
					OnGround: pkt.OnGround,
				})
				pm.BroadcastExcept(op.UUID, &playpacket.SetHeadRotation{
					EntityID: op.EntityID,
					HeadYaw:  byte(int32(pkt.Yaw*256.0/360.0) & 0xFF),
				})
			}
		}
		return nil
	}); err != nil {
		return err
	}

	onGroundID := playpacket.NewSetPlayerOnGround().ID()
	if err := router.Register(protocol.StatePlay, onGroundID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.SetPlayerOnGround)
		if !ok {
			return protocol.ErrNilPacket
		}
		mp, ok := s.Player().(MovablePlayer)
		if !ok || mp == nil {
			return nil
		}
		mp.SetOnGround(pkt.OnGround)
		return nil
	}); err != nil {
		return err
	}

	chunkBatchReceivedID := playpacket.NewChunkBatchReceived().ID()
	if err := router.Register(protocol.StatePlay, chunkBatchReceivedID, func(s Session, packet protocol.Packet) error {
		return nil
	}); err != nil {
		return err
	}

	playerLoadedID := playpacket.NewPlayerLoaded().ID()
	if err := router.Register(protocol.StatePlay, playerLoadedID, func(s Session, packet protocol.Packet) error {
		return nil
	}); err != nil {
		return err
	}

	abilitiesID := playpacket.NewPlayerAbilitiesServerbound().ID()
	if err := router.Register(protocol.StatePlay, abilitiesID, func(s Session, packet protocol.Packet) error {
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
		gm := int32(1)
		var playerEntity *entity.Player
		if p, ok := s.Player().(*entity.Player); ok && p != nil {
			gm = p.Living().GameMode()
			playerEntity = p
		}

		switch pkt.Status {
		case playpacket.DigStarted:
			if playerEntity != nil {
				eatingTracker.Cancel(playerEntity.ID())
			}
			if gm == 1 {
				breakBlock(s, pm, wa, pkt.Position, gm, true)
			} else {
				heldItemName := getHeldItemName(pm, s)
				onGround := playerEntity != nil && playerEntity.OnGround()
				stateID := int32(0)
				if wa != nil {
					stateID = wa.GetBlock(int(pkt.Position.X), int(pkt.Position.Y), int(pkt.Position.Z))
				}
				var effLevel int32
				if op := pm.GetBySession(s); op != nil && op.Windows != nil {
					held := op.Windows.HeldItem()
					if !held.Empty() {
						effLevel = enchantment.EfficiencyLevel(slotEnchantList(held))
					}
				}
				result := mining.CalculateBreakTime(stateID, heldItemName, onGround, effLevel)
				if result.Instant {
					breakBlock(s, pm, wa, pkt.Position, gm, result.CanHarvest)
				} else if result.Ticks > 0 && playerEntity != nil {
					miningTracker.Start(playerEntity.ID(),
						int(pkt.Position.X), int(pkt.Position.Y), int(pkt.Position.Z),
						stateID, result.Ticks, result.CanHarvest)
				}
			}

		case playpacket.DigCancelled:
			if playerEntity != nil {
				if ds := miningTracker.Cancel(playerEntity.ID()); ds != nil {
					if pm != nil {
						pm.Broadcast(&playpacket.BlockBreakAnimation{
							EntityID:     playerEntity.ID(),
							Position:     pkt.Position,
							DestroyStage: -1,
						})
					}
				}
			}

		case playpacket.DigFinished:
			if gm != 1 && playerEntity != nil {
				ds := miningTracker.Cancel(playerEntity.ID())
				if ds != nil {
					breakBlock(s, pm, wa, pkt.Position, gm, ds.CanHarvest)
				} else if wa != nil {
					actual := wa.GetBlock(int(pkt.Position.X), int(pkt.Position.Y), int(pkt.Position.Z))
					_ = s.Send(&playpacket.BlockUpdate{
						Position: pkt.Position,
						BlockID:  actual,
					})
				}
			}

		case playpacket.DigShootArrow:
			if playerEntity != nil {
				completeEating(s, pm, playerEntity, eatingTracker)
			}
		}

		return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: pkt.Sequence})
	}); err != nil {
		return err
	}

	useItemOnID := playpacket.NewUseItemOn().ID()
	if err := router.Register(protocol.StatePlay, useItemOnID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.UseItemOn)
		if !ok {
			return protocol.ErrNilPacket
		}

		if wa != nil {
			clickedState := wa.GetBlock(int(pkt.Position.X), int(pkt.Position.Y), int(pkt.Position.Z))
			if clickedState > 0 {
				ctx := &behavior.Context{
					X: int(pkt.Position.X), Y: int(pkt.Position.Y), Z: int(pkt.Position.Z),
					StateID: clickedState, Face: pkt.Face,
				}
				if behavior.GetByState(clickedState).OnUse(ctx) {
					if ctx.StateID == -1 {
						if furnaceType, wt, title, ok := behavior.IsFurnace(clickedState); ok {
							return handleOpenFurnace(s, pm, wa, furnaceType, wt, title, pkt.Position.X, pkt.Position.Y, pkt.Position.Z, pkt.Sequence)
						}
						if wt, title, slots, ok := behavior.IsContainer(clickedState); ok {
							return handleOpenContainer(s, pm, wt, title, slots, pkt.Sequence)
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
			wa.SetBlock(int(targetX), int(targetY), int(targetZ), blockToPlace)

			blockUpdate := &playpacket.BlockUpdate{
				Position: playpacket.BlockPos{X: targetX, Y: targetY, Z: targetZ},
				BlockID:  blockToPlace,
			}
			if err := s.Send(blockUpdate); err != nil {
				return err
			}

			if op != nil {
				living := func() *entity.LivingEntity {
					if p, ok := s.Player().(*entity.Player); ok && p != nil {
						return p.Living()
					}
					return nil
				}()
				if living != nil && living.GameMode() != 1 {
					op.Windows.ConsumeHeldItem()
					newHeld := op.Windows.HeldItem()
					_ = s.Send(&playpacket.SetContainerSlot{
						WindowID: 0,
						StateID:  op.Windows.CurrentStateID(),
						SlotIdx:  int16(inventory.HotbarStart + int(op.Windows.HeldSlot())),
						SlotData: newHeld,
					})
				}

				pm.BroadcastExcept(op.UUID, blockUpdate)

				blockName := block.NameFromState(blockToPlace)
				placeSoundName := "block.stone.place"
				if strings.Contains(blockName, "wood") || strings.Contains(blockName, "planks") || strings.Contains(blockName, "log") {
					placeSoundName = "block.wood.place"
				} else if strings.Contains(blockName, "sand") || strings.Contains(blockName, "gravel") {
					placeSoundName = "block.sand.place"
				} else if strings.Contains(blockName, "grass") || strings.Contains(blockName, "dirt") {
					placeSoundName = "block.grass.place"
				} else if strings.Contains(blockName, "glass") {
					placeSoundName = "block.glass.place"
				}
				soundID := gensound.SoundIDByName(placeSoundName)
				if soundID >= 0 {
					pm.Broadcast(&playpacket.SoundEffect{
						SoundID:       soundID + 1,
						SoundCategory: soundcategory.CategoryBlock,
						X:             int32(targetX*8 + 4),
						Y:             int32(targetY*8 + 4),
						Z:             int32(targetZ*8 + 4),
						Volume:        1.0,
						Pitch:         1.0,
					})
				}
			}
			wa.ScheduleTicksForBlock(int(targetX), int(targetY), int(targetZ), blockToPlace)
			wa.NotifyNeighbors(int(targetX), int(targetY), int(targetZ))
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

	entityActionID := playpacket.NewEntityAction().ID()
	if err := router.Register(protocol.StatePlay, entityActionID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.EntityAction)
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
		switch pkt.ActionID {
		case 0: // start sneaking
			op.Flags = (op.Flags | 0x02)
			op.Pose = 5
		case 1: // stop sneaking
			op.Flags = (op.Flags &^ 0x02)
			op.Pose = 0
		case 3: // start sprinting
			op.Flags = (op.Flags | 0x08)
		case 4: // stop sprinting
			op.Flags = (op.Flags &^ 0x08)
		default:
			return nil
		}
		md := metadata.New()
		md.SetByte(metadata.IndexBase, int8(op.Flags))
		md.SetPose(metadata.IndexPose, op.Pose)
		pm.BroadcastExcept(op.UUID, &playpacket.SetEntityMetadata{
			EntityID: op.EntityID,
			Metadata: md.Encode(),
		})
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
		if pm == nil {
			return nil
		}
		op := pm.GetBySession(s)
		if op == nil || op.Windows == nil {
			return nil
		}
		op.Windows.SetHeldSlot(int32(pkt.Slot))
		if p, ok := s.Player().(*entity.Player); ok && p != nil {
			eatingTracker.Cancel(p.ID())
		}
		held := op.Windows.HeldItem()
		eqSlot := playpacket.EquipmentSlot{SlotID: 0, Empty: held.Empty(), ItemID: held.ItemID, Count: int32(held.ItemCount)}
		pm.BroadcastExcept(op.UUID, &playpacket.EntityEquipment{
			EntityID: op.EntityID,
			Slots:    []playpacket.EquipmentSlot{eqSlot},
		})
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
			return handleRespawn(s, cfg, pm)
		}
		return nil
	}); err != nil {
		return err
	}

	settingsID := playpacket.NewServerboundSettings().ID()
	if err := router.Register(protocol.StatePlay, settingsID, func(s Session, packet protocol.Packet) error {
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
		if pm != nil {
			if ds, ok := s.(*DefaultSession); ok {
				if raw, found := ds.SessionData().Load("login_uuid"); found {
					if uuid, ok := raw.(protocol.UUID); ok {
						if op := pm.GetByUUID(uuid); op != nil && op.Windows != nil {
							op.Windows.HandleCreativeSet(pkt.SlotIndex, pkt.Item)
						}
					}
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

		if pm != nil {
			if p, ok := s.Player().(*entity.Player); ok && p != nil {
				living := p.Living()
				gm := living.GameMode()
				if gm != 1 && gm != 3 {
					op := pm.GetBySession(s)
					if op != nil && op.Windows != nil {
						held := op.Windows.HeldItem()
						if !held.Empty() {
							itemName := item.NameByID(held.ItemID)
							props := food.Get(itemName)
							if props != nil {
								canEat := living.FoodLevel() < 20 || props.CanAlwaysEat
								if canEat {
									eatingTracker.Start(p.ID(), itemName, held.ItemID, pkt.Hand, props)
								}
							}
						}
					}
				}
			}
		}

		return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: pkt.Sequence})
	}); err != nil {
		return err
	}

	chatMessageID := playpacket.NewServerboundChatMessage().ID()
	if err := router.Register(protocol.StatePlay, chatMessageID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*playpacket.ServerboundChatMessage)
		if !ok {
			return protocol.ErrNilPacket
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
			pm.Broadcast(playpacket.NewDisguisedChatSimple(pkt.Message, senderName))
		}
		return nil
	}); err != nil {
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
		if ds, ok := s.(*DefaultSession); ok {
			if raw, found := ds.SessionData().Load("login_uuid"); found {
				if uuid, ok := raw.(protocol.UUID); ok {
					if op := pm.GetByUUID(uuid); op != nil && op.Windows != nil {
						isCraftOutput := pkt.WindowID == 0 && pkt.Slot == int16(inventory.CraftOutputSlot)
						isCraftTableOutput := pkt.WindowID != 0 && pkt.Slot == 0 && op.Windows.ActiveWindow() != nil && op.Windows.ActiveWindow().Type == inventory.WindowTypeCrafting

						if isCraftOutput || isCraftTableOutput {
							outputSlot := op.Windows.GetSlot(int(pkt.Slot))
							if !outputSlot.Empty() {
								if isCraftOutput {
									op.Windows.ConsumeCraftIngredients()
								} else {
									op.Windows.ConsumeCraftTableIngredients()
								}
								op.Windows.HandleClick(pkt.WindowID, pkt.StateID, pkt.Slot, pkt.Button, pkt.Mode, nil, inventory.EmptySlot())
								if isCraftOutput {
									op.Windows.UpdateCraftOutput()
								} else {
									op.Windows.UpdateCraftTableOutput()
								}
							}
						} else {
							op.Windows.HandleClick(pkt.WindowID, pkt.StateID, pkt.Slot, pkt.Button, pkt.Mode, nil, inventory.EmptySlot())
							if pkt.WindowID == 0 && pkt.Slot >= int16(inventory.CraftInputStart) && pkt.Slot <= int16(inventory.CraftInputEnd) {
								op.Windows.UpdateCraftOutput()
							}
						}

						stateID, slots, cursor := op.Windows.SnapshotForResync(pkt.WindowID)
						if slots != nil {
							_ = s.Send(&playpacket.SetContainerContent{
								WindowID: pkt.WindowID,
								StateID:  stateID,
								Slots:    slots,
								Carried:  cursor,
							})
						}
					}
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}

	closeWindowID := playpacket.NewCloseWindow().ID()
	if err := router.Register(protocol.StatePlay, closeWindowID, func(s Session, packet protocol.Packet) error {
		if pm == nil {
			return nil
		}
		if ds, ok := s.(*DefaultSession); ok {
			if raw, found := ds.SessionData().Load("login_uuid"); found {
				if uuid, ok := raw.(protocol.UUID); ok {
					if op := pm.GetByUUID(uuid); op != nil && op.Windows != nil {
						op.Windows.CloseWindow()
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
			return handleAttack(s, pkt.EntityID, pm)
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

	for _, factory := range []func() protocol.Packet{
		playpacket.NewTickEnd,
		playpacket.NewServerboundPingRequest,
		playpacket.NewServerboundPong,
		playpacket.NewServerboundPlayCustomPayload,
		playpacket.NewPlayerInput,
		playpacket.NewServerboundRecipeBook,
		playpacket.NewMessageAcknowledgement,
		playpacket.NewChatSessionUpdate,
		playpacket.NewConfigurationAcknowledged,
		playpacket.NewVehicleMove,
		playpacket.NewSteerBoat,
		playpacket.NewUpdateSign,
		playpacket.NewResourcePackReceive,
		playpacket.NewSetDifficulty,
		playpacket.NewLockDifficulty,
		playpacket.NewEnchantItem,
		playpacket.NewEditBook,
		playpacket.NewPickItemFromBlock,
		playpacket.NewPickItemFromEntity,
		playpacket.NewCraftRecipeRequest,
		playpacket.NewDisplayedRecipe,
		playpacket.NewNameItem,
		playpacket.NewAdvancementTab,
		playpacket.NewSelectTrade,
		playpacket.NewSetBeaconEffect,
		playpacket.NewSpectate,
		playpacket.NewSetSlotState,
		playpacket.NewQueryBlockNbt,
		playpacket.NewQueryEntityNbt,
		playpacket.NewSelectBundleItem,
		playpacket.NewServerboundCookieResponse,
		playpacket.NewDebugSampleSubscription,
		playpacket.NewGenerateStructure,
		playpacket.NewUpdateCommandBlock,
		playpacket.NewUpdateCommandBlockMinecart,
		playpacket.NewUpdateJigsawBlock,
		playpacket.NewUpdateStructureBlock,
	} {
		pktID := factory().ID()
		if err := router.Register(protocol.StatePlay, pktID, func(s Session, packet protocol.Packet) error {
			return nil
		}); err != nil {
			return err
		}
	}

	return nil
}

// SendPlayBootstrap sends the initial play-state packet sequence to the client.
func SendPlayBootstrap(s Session, entityID int32, cfg PlayBootstrapConfig) error {
	teleportID := nextTeleportID()

	loginPlay := &playpacket.LoginPlay{
		EntityID:            entityID,
		IsHardcore:          false,
		DimensionNames:      []string{"minecraft:overworld", "minecraft:the_nether", "minecraft:the_end"},
		MaxPlayers:          20,
		ViewDistance:        cfg.ViewDistance,
		SimulationDistance:  cfg.SimulationDistance,
		ReducedDebugInfo:    false,
		EnableRespawnScreen: true,
		DoLimitedCrafting:   false,
		DimensionType:       dimensionTypeID(cfg.RegistryManager, "minecraft:overworld"),
		DimensionName:       "minecraft:overworld",
		HashedSeed:          0,
		GameMode:            byte(cfg.GameMode),
		PreviousGameMode:    -1,
		IsDebug:             false,
		IsFlat:              false,
		HasDeathLocation:    false,
		PortalCooldown:      0,
		SeaLevel:            63,
		EnforcesSecureChat:  true,
	}
	if err := s.Send(loginPlay); err != nil {
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

	var abilities *playpacket.PlayerAbilitiesClientbound
	if cfg.GameMode == 1 {
		abilities = playpacket.CreativeAbilities()
	} else {
		abilities = playpacket.SurvivalAbilities()
	}
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

	if err := s.Send(&playpacket.UpdateHealth{Health: 20.0, Food: 20, FoodSaturation: 5.0}); err != nil {
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

	if err := s.Send(&playpacket.UpdateTime{WorldAge: 0, TimeOfDay: 6000, TickDayTime: true}); err != nil {
		return err
	}

	if err := s.Send(&playpacket.SetExperience{ExperienceBar: 0, ExperienceLevel: 0, TotalExperience: 0}); err != nil {
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

	if err := s.Send(&playpacket.ClientboundHeldItemSlot{Slot: 0}); err != nil {
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
		Yaw:        0,
		Pitch:      0,
		Flags:      0,
	}
	if err := s.Send(syncPos); err != nil {
		return err
	}
	logger.Debug("sent sync_player_position", "session", s.ID(), "teleport_id", teleportID)

	startWaiting := &playpacket.GameEvent{
		Event: 13,
		Value: 0,
	}
	if err := s.Send(startWaiting); err != nil {
		return err
	}

	if err := s.Send(&playpacket.ChunkBatchStart{}); err != nil {
		return err
	}

	radius := int32(3)
	chunkCount := int32(0)
	for dx := -radius; dx <= radius; dx++ {
		for dz := -radius; dz <= radius; dz++ {
			cx := chunkX + dx
			cz := chunkZ + dz
			payload := generateSpawnChunkPayload(cfg, cx, cz)
			if payload == nil {
				continue
			}
			pkt := &playpacket.ChunkDataAndUpdateLight{
				ChunkX:  cx,
				ChunkZ:  cz,
				Payload: payload,
			}
			if err := s.Send(pkt); err != nil {
				return err
			}
			chunkCount++
		}
	}

	if err := s.Send(&playpacket.ChunkBatchFinished{BatchSize: chunkCount}); err != nil {
		return err
	}

	logger.Debug("play bootstrap complete", "session", s.ID(), "spawn_chunks", chunkCount)

	return nil
}

func generateSpawnChunkPayload(cfg PlayBootstrapConfig, cx, cz int32) []byte {
	if cfg.SpawnChunks != nil {
		return cfg.SpawnChunks.GenerateSpawnChunk(cx, cz)
	}
	biomeID := int32(0)
	if cfg.RegistryManager != nil {
		if id := cfg.RegistryManager.IDByName("minecraft:worldgen/biome", "minecraft:plains"); id >= 0 {
			biomeID = id
		}
	}
	gen := terrain.NewNoiseGenerator(42, biomeID)
	c := gen.Generate(cx, cz)
	return c.EncodePacketPayload()
}

// WorldChunkProvider implements SpawnChunkProvider using the world's chunk manager.
type WorldChunkProvider struct {
	Chunks   *chunk.Manager
	Fallback *terrain.NoiseGenerator
}

// GenerateSpawnChunk produces a chunk payload, trying the world's chunk manager first.
func (p *WorldChunkProvider) GenerateSpawnChunk(cx, cz int32) []byte {
	if p.Chunks != nil {
		p.Chunks.RequestLoad(cx, cz)
		p.Chunks.PumpLoadRequests()
		for i := 0; i < 100; i++ {
			p.Chunks.ApplyLoadCompletions(256)
			if c, ok := p.Chunks.GetChunk(cx, cz); ok {
				if payload := c.EncodePacketPayload(); payload != nil {
					return payload
				}
			}
			time.Sleep(time.Millisecond)
			p.Chunks.PumpLoadRequests()
		}
	}
	if p.Fallback != nil {
		c := p.Fallback.Generate(cx, cz)
		return c.EncodePacketPayload()
	}
	return nil
}

func handleOpenContainer(s Session, pm *PlayerManager, windowType int32, title string, slotCount int, sequence int32) error {
	if pm == nil {
		return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
	}
	op := pm.GetBySession(s)
	if op == nil || op.Windows == nil {
		return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
	}

	container := inventory.NewContainer(slotCount)
	w := op.Windows.OpenWindow(windowType, title, container)

	if err := s.Send(&playpacket.OpenScreen{
		WindowID:   w.ID,
		WindowType: windowType,
		Title:      title,
	}); err != nil {
		return err
	}

	slots := make([]inventory.Slot, slotCount)
	stateID := op.Windows.StateID()
	if err := s.Send(&playpacket.SetContainerContent{
		WindowID: w.ID,
		StateID:  stateID,
		Slots:    slots,
		Carried:  inventory.EmptySlot(),
	}); err != nil {
		return err
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

	container := inventory.NewContainer(3)
	w := op.Windows.OpenWindow(windowType, title, container)

	if err := s.Send(&playpacket.OpenScreen{
		WindowID:   w.ID,
		WindowType: windowType,
		Title:      title,
	}); err != nil {
		return err
	}

	slots := container.Slots()
	stateID := op.Windows.StateID()
	if err := s.Send(&playpacket.SetContainerContent{
		WindowID: w.ID,
		StateID:  stateID,
		Slots:    slots,
		Carried:  inventory.EmptySlot(),
	}); err != nil {
		return err
	}

	if err := s.Send(&playpacket.SetContainerProperty{
		WindowID: int8(w.ID),
		Property: 0,
		Value:    0,
	}); err != nil {
		return err
	}
	if err := s.Send(&playpacket.SetContainerProperty{
		WindowID: int8(w.ID),
		Property: 1,
		Value:    0,
	}); err != nil {
		return err
	}
	if err := s.Send(&playpacket.SetContainerProperty{
		WindowID: int8(w.ID),
		Property: 2,
		Value:    0,
	}); err != nil {
		return err
	}
	if err := s.Send(&playpacket.SetContainerProperty{
		WindowID: int8(w.ID),
		Property: 3,
		Value:    200,
	}); err != nil {
		return err
	}

	return s.Send(&playpacket.AcknowledgeBlockChange{Sequence: sequence})
}

func handleRespawn(s Session, cfg PlayBootstrapConfig, pm *PlayerManager) error {
	dimType := dimensionTypeID(cfg.RegistryManager, "minecraft:overworld")

	respawnPkt := &playpacket.Respawn{
		DimensionType:    dimType,
		DimensionName:    "minecraft:overworld",
		HashedSeed:       0,
		GameMode:         1,
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

	teleportID := nextTeleportID()
	if err := s.Send(&playpacket.SyncPlayerPosition{
		TeleportID: teleportID,
		X:          cfg.SpawnX,
		Y:          cfg.SpawnY,
		Z:          cfg.SpawnZ,
	}); err != nil {
		return err
	}

	chunkX := int32(cfg.SpawnX) >> 4
	chunkZ := int32(cfg.SpawnZ) >> 4
	if err := s.Send(&playpacket.SetCenterChunk{ChunkX: chunkX, ChunkZ: chunkZ}); err != nil {
		return err
	}

	if err := s.Send(&playpacket.UpdateHealth{Health: 20.0, Food: 20, FoodSaturation: 5.0}); err != nil {
		return err
	}

	if pm != nil {
		if op := pm.GetBySession(s); op != nil {
			op.X, op.Y, op.Z = cfg.SpawnX, cfg.SpawnY, cfg.SpawnZ
		}
	}

	if p, ok := s.Player().(*entity.Player); ok && p != nil {
		p.Living().Respawn()
		p.SetPosition(entity.Vec3{X: cfg.SpawnX, Y: cfg.SpawnY, Z: cfg.SpawnZ})
	}

	logger.Debug("respawned", "session", s.ID(), "x", cfg.SpawnX, "y", cfg.SpawnY, "z", cfg.SpawnZ)
	return nil
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
	if attackerLiving.IsDead() {
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

	var baseDamage float32 = 1.0
	attackerWeapon := getHeldItemName(pm, s)
	isSword := strings.HasSuffix(attackerWeapon, "_sword")
	var attackSpeedTicks int32 = 10
	if wp := equipment.GetWeapon(attackerWeapon); wp != nil {
		baseDamage = float32(wp.AttackDamage)
		if wp.AttackSpeed > 0 {
			attackSpeedTicks = int32(20.0 / wp.AttackSpeed)
		}
	}

	cooldownProgress := float32(1.0)
	if attackerLiving.AttackCooldown() > 0 {
		elapsed := attackSpeedTicks - attackerLiving.AttackCooldown()
		if elapsed < 0 {
			elapsed = 0
		}
		cooldownProgress = float32(elapsed) / float32(attackSpeedTicks)
		if cooldownProgress > 1 {
			cooldownProgress = 1
		}
	}
	baseDamage *= 0.2 + cooldownProgress*cooldownProgress*0.8

	critical := cooldownProgress > 0.9 && !attackerLiving.OnGround() && attackerLiving.FallDistance() > 0
	if critical {
		baseDamage *= 1.5
	}

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
				enchProtection += enchantment.FireProtectionFactor(el)
				enchProtection += enchantment.BlastProtectionFactor(el)
				enchProtection += enchantment.ProjectileProtectionFactor(el)
			}
		}
	}
	if enchProtection > 20 {
		enchProtection = 20
	}
	if attacker.Windows != nil {
		held := attacker.Windows.HeldItem()
		if !held.Empty() {
			elist := slotEnchantList(held)
			baseDamage += float32(enchantment.SharpnessDamage(elist))
		}
	}

	if totalDefense > 0 {
		baseDamage = float32(equipment.CalculateDamageReduction(float64(baseDamage), totalDefense, totalToughness))
	}
	if enchProtection > 0 {
		baseDamage *= float32(1.0 - enchProtection/25.0)
	}

	actual := targetLiving.Damage(baseDamage, "player")
	if actual <= 0 {
		return nil
	}

	attackerLiving.ResetAttackCooldown(attackSpeedTicks)

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
				sweepLiving.Damage(sweepDamage, "player_sweep")
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
	wa.SetBlock(bx, by, bz, 0)

	var drops []behavior.Drop
	if oldState > 0 && canHarvest {
		ctx := &behavior.Context{
			X: bx, Y: by, Z: bz,
			StateID: oldState, PlayerGM: gm,
		}
		drops = behavior.GetByState(oldState).OnBreak(ctx)
	}

	blockUpdate := &playpacket.BlockUpdate{Position: pos, BlockID: 0}
	if err := s.Send(blockUpdate); err != nil {
		return
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
