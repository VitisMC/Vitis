package session

import (
	"math"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/metadata"
	"github.com/vitismc/vitis/internal/food"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/logger"
	"github.com/vitismc/vitis/internal/mining"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// useItemCommand applies item use after authoritative hand and life checks.
type useItemCommand struct {
	session Session
	players *PlayerManager
	world   WorldAccessor
	request playpacket.UseItem
	config  PlayBootstrapConfig
	eating  *food.EatingTracker
	mining  *mining.Tracker
}

// Execute applies item-use effects and acknowledges the client sequence on tick.
func (c *useItemCommand) Execute() {
	s, pm, pkt, eatingTracker := c.session, c.players, &c.request, c.eating
	defer func() {
		if err := s.Send(&playpacket.AcknowledgeBlockChange{Sequence: pkt.Sequence}); err != nil {
			logger.Warn("acknowledge item use failed", "error", err)
		}
	}()
	if s.Context().Err() != nil || pm == nil {
		return
	}
	rememberPlayerActions(s, pm, c.mining, c.eating)
	op := pm.GetBySession(s)
	p, ok := s.Player().(*entity.Player)
	if op == nil || op.Windows == nil || !ok || p == nil || p.Living().IsDead() || op.GameMode == 3 || pkt.Hand < 0 || pkt.Hand > 1 {
		return
	}
	slot := inventory.HotbarStart + int(op.Windows.HeldSlot())
	if pkt.Hand == 1 {
		slot = inventory.OffhandSlot
	}
	held := op.Windows.Inventory().Get(slot)
	if !held.Empty() && item.NameByID(held.ItemID) == "minecraft:map" && c.config.CreateMap != nil {
		position := p.Position()
		filled, err := c.config.CreateMap(int32(math.Floor(position.X)), int32(math.Floor(position.Z)))
		if err != nil {
			logger.Warn("create map failed", "player", op.Name, "error", err)
			return
		}
		if op.GameMode != 1 {
			held.ItemCount--
			op.Windows.Inventory().Set(slot, held)
		}
		if held.Empty() {
			op.Windows.Inventory().Set(slot, filled)
		} else {
			indices := make([]int, 0, 36)
			for index := inventory.MainInventoryStart; index <= inventory.HotbarEnd; index++ {
				indices = append(indices, index)
			}
			remainder, _ := op.Windows.Inventory().Insert(filled, filled.MaxStackSize(), indices)
			if !remainder.Empty() {
				spawnInventoryItem(pm, c.world, position.X, position.Y, position.Z, remainder)
			}
		}
		if err := sendWindowResync(s, op.Windows, 0); err != nil {
			logger.Warn("resync map use failed", "error", err)
		}
		return
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

					// Shield blocking: check main hand or offhand
					var shieldName string
					if pkt.Hand == 0 {
						h := op.Windows.HeldItem()
						if !h.Empty() {
							shieldName = item.NameByID(h.ItemID)
						}
					} else {
						oh := op.Windows.Inventory().Get(inventory.OffhandSlot)
						if !oh.Empty() {
							shieldName = item.NameByID(oh.ItemID)
						}
					}
					if shieldName == "minecraft:shield" && living.ShieldCooldown() <= 0 {
						living.SetBlocking(true)
						smd := metadata.New()
						smd.SetByte(metadata.IndexLivingHandStates, 0x01)
						pm.Broadcast(&playpacket.SetEntityMetadata{
							EntityID: op.EntityID,
							Metadata: smd.Encode(),
						})
					}
				}
			}
		}
	}

}
