package session

import (
	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/block/behavior"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/food"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/logger"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

type heldSlotCommand struct {
	session Session
	players *PlayerManager
	eating  *food.EatingTracker
	slot    int32
}

// Execute changes the selected hotbar slot on the authoritative world tick.
func (c *heldSlotCommand) Execute() {
	if c.session == nil || c.session.Context().Err() != nil {
		return
	}
	if c.players == nil {
		return
	}
	op := c.players.GetBySession(c.session)
	if op == nil || op.Windows == nil {
		return
	}
	op.Windows.SetHeldSlot(c.slot)
	if player, ok := c.session.Player().(*entity.Player); ok && player != nil {
		c.eating.Cancel(player.ID())
	}
	held := op.Windows.HeldItem()
	c.players.BroadcastExcept(op.UUID, &playpacket.EntityEquipment{
		EntityID: op.EntityID,
		Slots: []playpacket.EquipmentSlot{{
			SlotID: 0, Stack: held.Clone(), Empty: held.Empty(), ItemID: held.ItemID, Count: held.ItemCount,
		}},
	})
}

type creativeSlotCommand struct {
	session Session
	players *PlayerManager
	slot    int16
	item    inventory.ItemStack
}

type closeWindowCommand struct {
	session  Session
	players  *PlayerManager
	world    WorldAccessor
	windowID int32
}

type openBlockWindowCommand struct {
	session  Session
	players  *PlayerManager
	world    WorldAccessor
	x, y, z  int32
	sequence int32
}

// Execute opens the current block's window from the authoritative world tick.
func (c *openBlockWindowCommand) Execute() {
	if c.session == nil || c.session.Context().Err() != nil {
		return
	}
	player, ok := c.session.Player().(*entity.Player)
	if !ok || player == nil || !blockWithinReach(player, playpacket.BlockPos{X: c.x, Y: c.y, Z: c.z}, 6.0) {
		_ = c.session.Send(&playpacket.AcknowledgeBlockChange{Sequence: c.sequence})
		return
	}
	state := c.world.GetBlock(int(c.x), int(c.y), int(c.z))
	_, _, _, isContainer := behavior.IsContainer(state)
	_, _, _, isFurnace := behavior.IsFurnace(state)
	if c.players != nil && (isContainer || isFurnace) {
		if online := c.players.GetBySession(c.session); online != nil && online.Windows != nil {
			if active := online.Windows.ActiveWindow(); active != nil {
				(&closeWindowCommand{session: c.session, players: c.players, world: c.world, windowID: active.ID}).Execute()
			}
		}
	}
	var err error
	if furnaceType, windowType, title, ok := behavior.IsFurnace(state); ok {
		err = handleOpenFurnace(c.session, c.players, c.world, furnaceType, windowType, title, c.x, c.y, c.z, c.sequence)
	} else if windowType, title, slots, ok := behavior.IsContainer(state); ok {
		err = handleOpenContainer(c.session, c.players, c.world, state, windowType, title, slots, int(c.x), int(c.y), int(c.z), c.sequence)
	} else {
		err = c.session.Send(&playpacket.AcknowledgeBlockChange{Sequence: c.sequence})
	}
	if err != nil {
		logger.Warn("open block window failed", "session", c.session.ID(), "error", err)
	}
}

// Execute closes only the window identified by the client's ordered action.
func (c *closeWindowCommand) Execute() {
	if c.session == nil || c.session.Context().Err() != nil {
		return
	}
	if c.players == nil {
		return
	}
	op := c.players.GetBySession(c.session)
	if op == nil || op.Windows == nil {
		return
	}
	active := op.Windows.ActiveWindow()
	if active == nil || active.ID != c.windowID {
		return
	}
	drops := op.Windows.CloseWindowWithDrops()
	if c.world != nil {
		state := c.world.GetBlock(active.BlockX, active.BlockY, active.BlockZ)
		if behavior.IsChest(state) {
			viewerCount := byte(0)
			for _, viewer := range c.players.Players() {
				if viewer != nil && viewer.Windows != nil && sameOpenBlockContainer(viewer.Windows.ActiveWindow(), active) && viewerCount < 255 {
					viewerCount++
				}
			}
			c.players.Broadcast(&playpacket.BlockAction{
				Position: playpacket.BlockPos{X: int32(active.BlockX), Y: int32(active.BlockY), Z: int32(active.BlockZ)},
				ActionID: 1, ActionParameter: viewerCount, BlockType: block.BlockIDFromState(state),
			})
			if active.DoubleChest {
				otherState := c.world.GetBlock(active.OtherX, active.OtherY, active.OtherZ)
				c.players.Broadcast(&playpacket.BlockAction{
					Position: playpacket.BlockPos{X: int32(active.OtherX), Y: int32(active.OtherY), Z: int32(active.OtherZ)},
					ActionID: 1, ActionParameter: viewerCount, BlockType: block.BlockIDFromState(otherState),
				})
			}
		}
	}
	for _, stack := range drops {
		spawnInventoryItem(c.players, c.world, op.X, op.Y+1, op.Z, stack)
	}
	_ = sendWindowResync(c.session, op.Windows, 0)
}

// Execute applies a creative inventory update only while the player still has creative capability.
func (c *creativeSlotCommand) Execute() {
	if c.session == nil || c.session.Context().Err() != nil {
		return
	}
	if c.players == nil {
		return
	}
	op := c.players.GetBySession(c.session)
	if op == nil || op.Windows == nil {
		return
	}
	player, ok := c.session.Player().(*entity.Player)
	if !ok || player == nil || player.Living().GameMode() != 1 {
		_ = sendWindowResync(c.session, op.Windows, 0)
		return
	}
	op.Windows.HandleCreativeSet(c.slot, c.item)
	if IsEquipmentSlot(c.slot) || int(c.slot) == inventory.HotbarStart+int(op.Windows.HeldSlot()) {
		UpdateArmorStats(op)
		if packet := BuildFullEquipmentPacket(op.EntityID, op.Windows); packet != nil {
			c.players.BroadcastExcept(op.UUID, packet)
		}
	}
	// Creative inventory edits are client-predicted. Echoing the whole menu here
	// overwrites the client's carried stack between consecutive slot updates.
}
