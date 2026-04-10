package session

import (
	"github.com/vitismc/vitis/internal/enchantment"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/equipment"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
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

	var totalDefense, totalToughness, enchProt float64
	inv := op.Windows.Inventory()
	for slot := inventory.ArmorStart; slot <= inventory.ArmorEnd; slot++ {
		as := inv.Get(slot)
		if as.Empty() {
			continue
		}
		armorName := item.NameByID(as.ItemID)
		if ap := equipment.GetArmor(armorName); ap != nil {
			totalDefense += ap.Defense
			totalToughness += ap.Toughness
		}
		el := slotEnchantList(as)
		enchProt += enchantment.ProtectionFactor(el)
		enchProt += enchantment.FireProtectionFactor(el)
		enchProt += enchantment.BlastProtectionFactor(el)
		enchProt += enchantment.ProjectileProtectionFactor(el)
	}
	if enchProt > 20 {
		enchProt = 20
	}

	p.Living().SetArmorStats(totalDefense, totalToughness, enchProt)
}

// IsEquipmentSlot returns true if the given player inventory slot index
// is an armor slot or the offhand slot.
func IsEquipmentSlot(slot int16) bool {
	return (slot >= int16(inventory.ArmorStart) && slot <= int16(inventory.ArmorEnd)) ||
		slot == int16(inventory.OffhandSlot)
}

// damageArmorSlots reduces durability by 1 on each equipped armor piece,
// respecting the Unbreaking enchantment. Broken pieces are cleared.
func damageArmorSlots(op *OnlinePlayer) {
	if op == nil || op.Windows == nil {
		return
	}
	inv := op.Windows.Inventory()
	changed := false
	for slot := inventory.ArmorStart; slot <= inventory.ArmorEnd; slot++ {
		as := inv.Get(slot)
		if as.Empty() || !item.IsDurable(as.ItemID) {
			continue
		}
		unbreaking := as.EnchantLevel(item.EnchantUnbreakingID)
		if !item.ShouldConsumeDurability(unbreaking, true) {
			continue
		}
		newDmg, broken := item.ApplyDamage(as.ItemID, as.Damage, 1)
		if broken {
			inv.Clear(slot)
			_ = op.Session.Send(&playpacket.SetPlayerInventorySlot{
				SlotIdx:  int32(slot),
				SlotData: inventory.EmptySlot(),
			})
		} else {
			as.Damage = newDmg
			inv.Set(slot, as)
			_ = op.Session.Send(&playpacket.SetPlayerInventorySlot{
				SlotIdx:  int32(slot),
				SlotData: as,
			})
		}
		changed = true
	}
	if changed {
		UpdateArmorStats(op)
	}
}
