package session

import (
	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/portal"
)

type ignitionCommand struct {
	session Session
	players *PlayerManager
	world   WorldAccessor
	packet  playpacket.UseItemOn
}

// Execute applies flint-and-steel or fire-charge use on the world tick.
func (c *ignitionCommand) Execute() {
	if c.session == nil || c.session.Context().Err() != nil {
		return
	}
	pkt := c.packet
	ack := func() { _ = c.session.Send(&playpacket.AcknowledgeBlockChange{Sequence: pkt.Sequence}) }
	player, ok := c.session.Player().(*entity.Player)
	if !ok || player == nil || player.Living().GameMode() == 3 || !blockWithinReach(player, pkt.Position, 6) {
		ack()
		return
	}
	op := c.players.GetBySession(c.session)
	if op == nil || op.Windows == nil {
		ack()
		return
	}
	slotIndex := inventory.HotbarStart + int(op.Windows.HeldSlot())
	if pkt.Hand == 1 {
		slotIndex = inventory.OffhandSlot
	}
	held := op.Windows.GetSlot(slotIndex)
	itemName := item.NameByID(held.ItemID)
	if held.Empty() || (itemName != "minecraft:flint_and_steel" && itemName != "minecraft:fire_charge") {
		ack()
		return
	}
	x, y, z := int(pkt.Position.X), int(pkt.Position.Y), int(pkt.Position.Z)
	if block.NameFromState(c.world.GetBlock(x, y, z)) == "minecraft:tnt" {
		if c.world.IgniteTNT(x, y, z, player.ID()) {
			c.consume(op, slotIndex, held, itemName)
		}
		ack()
		return
	}
	dx, dy, dz := BlockFaceOffset(pkt.Face)
	tx, ty, tz := x+int(dx), y+int(dy), z+int(dz)
	if _, loaded := c.world.GetChunk(chunkCoord(float64(tx)), chunkCoord(float64(tz))); !loaded || !isAirForPlacement(c.world.GetBlock(tx, ty, tz)) {
		_ = resyncBlock(c.session, c.world, playpacket.BlockPos{X: int32(tx), Y: int32(ty), Z: int32(tz)})
		ack()
		return
	}
	if isAirForPlacement(c.world.GetBlock(x, y, z)) {
		ack()
		return
	}
	fire := block.DefaultStateID("minecraft:fire")
	if fire < 0 {
		ack()
		return
	}
	c.world.SetBlock(tx, ty, tz, fire)
	if frame, found := portal.FindFrame(tx, ty, tz, c.world.GetBlock); found {
		portalState := block.StateID("minecraft:nether_portal", map[string]string{"axis": frame.Axis})
		if portalState >= 0 {
			for _, position := range frame.Interior {
				c.world.SetBlock(position.X, position.Y, position.Z, portalState)
				c.players.Broadcast(&playpacket.BlockUpdate{Position: playpacket.BlockPos{X: int32(position.X), Y: int32(position.Y), Z: int32(position.Z)}, BlockID: portalState})
			}
			c.consume(op, slotIndex, held, itemName)
			ack()
			return
		}
	}
	c.world.ScheduleTicksForBlock(tx, ty, tz, fire)
	c.world.NotifyNeighbors(tx, ty, tz)
	c.players.Broadcast(&playpacket.BlockUpdate{Position: playpacket.BlockPos{X: int32(tx), Y: int32(ty), Z: int32(tz)}, BlockID: fire})
	c.consume(op, slotIndex, held, itemName)
	ack()
}

func (c *ignitionCommand) consume(op *OnlinePlayer, slotIndex int, held inventory.Slot, itemName string) {
	if player, ok := c.session.Player().(*entity.Player); ok && player.Living().GameMode() == 1 {
		return
	}
	if itemName == "minecraft:flint_and_steel" {
		if item.ShouldConsumeDurability(held.EnchantLevel(item.EnchantUnbreakingID), false) {
			damage, broken := item.ApplyDamage(held.ItemID, held.Damage, 1)
			if broken {
				held = inventory.EmptySlot()
			} else {
				held.SetDamage(damage)
			}
		}
	} else {
		held.ItemCount--
		if held.Empty() {
			held = inventory.EmptySlot()
		}
	}
	op.Windows.Inventory().Set(slotIndex, held)
	_ = sendWindowResync(c.session, op.Windows, 0)
}
