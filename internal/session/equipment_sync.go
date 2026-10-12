package session

import (
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/equipment"
	"github.com/vitismc/vitis/internal/inventory"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// Equipment slot IDs as defined by the Minecraft protocol.
const (
	ProtoSlotMainHand   byte = 0
	ProtoSlotOffHand    byte = 1
	ProtoSlotBoots      byte = 2
	ProtoSlotLeggings   byte = 3
	ProtoSlotChestplate byte = 4
	ProtoSlotHelmet     byte = 5
)

// BuildFullEquipmentPacket creates an EntityEquipment packet with all six
// equipment slots (main hand, off hand, boots, leggings, chestplate, helmet)
// from the player's inventory.
func BuildFullEquipmentPacket(entityID int32, wm *inventory.WindowManager) *playpacket.EntityEquipment {
	if wm == nil {
		return nil
	}
	inv := wm.Inventory()
	held := wm.HeldItem()

	slots := make([]playpacket.EquipmentSlot, 6)

	// Main hand
	slots[0] = makeEquipSlot(ProtoSlotMainHand, held)
	// Off hand
	slots[1] = makeEquipSlot(ProtoSlotOffHand, inv.Get(inventory.OffhandSlot))
	// Boots (inventory slot 8)
	slots[2] = makeEquipSlot(ProtoSlotBoots, inv.Get(inventory.ArmorEnd))
	// Leggings (inventory slot 7)
	slots[3] = makeEquipSlot(ProtoSlotLeggings, inv.Get(inventory.ArmorEnd-1))
	// Chestplate (inventory slot 6)
	slots[4] = makeEquipSlot(ProtoSlotChestplate, inv.Get(inventory.ArmorStart+1))
	// Helmet (inventory slot 5)
	slots[5] = makeEquipSlot(ProtoSlotHelmet, inv.Get(inventory.ArmorStart))

	return &playpacket.EntityEquipment{
		EntityID: entityID,
		Slots:    slots,
	}
}

func makeEquipSlot(protoSlot byte, s inventory.Slot) playpacket.EquipmentSlot {
	return playpacket.EquipmentSlot{
		Stack:  s.Clone(),
		SlotID: protoSlot,
		Empty:  s.Empty(),
		ItemID: s.ItemID,
		Count:  int32(s.ItemCount),
	}
}

// UpdateArmorStats recomputes and caches armor defense, toughness, and
// enchantment protection on the player's LivingEntity from their inventory.
func UpdateArmorStats(op *OnlinePlayer) {
	if op == nil || op.Windows == nil {
		return
	}
	p, ok := op.Session.Player().(*entity.Player)
	if !ok || p == nil {
		return
	}

	stats := equipment.PlayerArmorStats(op.Windows.Inventory())
	p.Living().SetArmorStats(stats.Defense, stats.Toughness, stats.Protection)
	p.Living().SetArmorDamageProtection(stats.Fire, stats.Blast, stats.Projectile, stats.Fall)
}

// IsEquipmentSlot returns true if the given player inventory slot index
// is an armor slot or the offhand slot.
func IsEquipmentSlot(slot int16) bool {
	return (slot >= int16(inventory.ArmorStart) && slot <= int16(inventory.ArmorEnd)) ||
		slot == int16(inventory.OffhandSlot)
}
