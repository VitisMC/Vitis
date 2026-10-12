package session

import (
	"time"

	"github.com/vitismc/vitis/internal/enchantment"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/food"
	"github.com/vitismc/vitis/internal/logger"
	"github.com/vitismc/vitis/internal/mining"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

type digActionCommand struct {
	session    Session
	players    *PlayerManager
	world      WorldAccessor
	packet     playpacket.BlockDig
	mining     *mining.Tracker
	eating     *food.EatingTracker
	receivedAt time.Time
}

// Execute applies one ordered digging action on the authoritative world tick.
func (c *digActionCommand) Execute() {
	if c.session == nil || c.session.Context().Err() != nil {
		return
	}
	rememberPlayerActions(c.session, c.players, c.mining, c.eating)
	if err := c.run(); err != nil {
		logger.Warn("block dig response failed", "session", c.session.ID(), "error", err)
	}
}

func (c *digActionCommand) run() error {
	pkt := &c.packet
	player, ok := c.session.Player().(*entity.Player)
	if !ok || player == nil {
		return rejectBlockDig(c.session, c.world, pkt.Position, pkt.Sequence)
	}
	gameMode := player.Living().GameMode()
	x, y, z := int(pkt.Position.X), int(pkt.Position.Y), int(pkt.Position.Z)

	switch pkt.Status {
	case playpacket.DigStarted:
		if !blockWithinReach(player, pkt.Position, 6.0) {
			return rejectBlockDig(c.session, c.world, pkt.Position, pkt.Sequence)
		}
		stateID := c.world.GetBlock(x, y, z)
		if stateID <= 0 {
			return rejectBlockDig(c.session, c.world, pkt.Position, pkt.Sequence)
		}
		c.eating.Cancel(player.ID())
		if gameMode == 1 {
			breakBlock(c.session, c.players, c.world, pkt.Position, gameMode, true)
			return c.acknowledge()
		}
		var efficiency int32
		if c.players != nil {
			if online := c.players.GetBySession(c.session); online != nil && online.Windows != nil {
				held := online.Windows.HeldItem()
				if !held.Empty() {
					efficiency = enchantment.EfficiencyLevel(slotEnchantList(held))
				}
			}
		}
		result := mining.CalculateBreakTime(stateID, getHeldItemName(c.players, c.session), player.OnGround(), efficiency)
		if result.Instant {
			breakBlock(c.session, c.players, c.world, pkt.Position, gameMode, result.CanHarvest)
		} else if result.Ticks > 0 {
			c.mining.StartAt(player.ID(), x, y, z, stateID, result.Ticks, result.ProgressPerTick, result.CanHarvest, c.receivedAt)
		}
	case playpacket.DigCancelled:
		if c.mining.Cancel(player.ID()) != nil && c.players != nil {
			c.players.Broadcast(&playpacket.BlockBreakAnimation{
				EntityID: player.ID(), Position: pkt.Position, DestroyStage: -1,
			})
		}
	case playpacket.DigFinished:
		if gameMode != 1 {
			state := c.mining.Cancel(player.ID())
			stateID := c.world.GetBlock(x, y, z)
			if state == nil || !state.Matches(x, y, z, stateID) || !state.Ready(c.receivedAt) || !blockWithinReach(player, pkt.Position, 6.0) {
				return rejectBlockDig(c.session, c.world, pkt.Position, pkt.Sequence)
			}
			breakBlock(c.session, c.players, c.world, pkt.Position, gameMode, state.CanHarvest)
		}
	case playpacket.DigShootArrow:
		completeEating(c.session, c.players, player, c.eating)
	case playpacket.DigDropItem, playpacket.DigDropStack:
		if c.players != nil {
			if op := c.players.GetBySession(c.session); op != nil && op.Windows != nil {
				dropped := op.Windows.DropHeldItem(pkt.Status == playpacket.DigDropStack)
				if !dropped.Empty() {
					spawnInventoryItem(c.players, c.world, op.X, op.Y+1, op.Z, dropped)
					_ = sendWindowResync(c.session, op.Windows, 0)
				}
			}
		}
	}
	return c.acknowledge()
}

func (c *digActionCommand) acknowledge() error {
	return c.session.Send(&playpacket.AcknowledgeBlockChange{Sequence: c.packet.Sequence})
}
