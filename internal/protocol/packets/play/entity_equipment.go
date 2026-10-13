package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// EntityEquipment sends an entity's equipment (armor, held items).
type EntityEquipment struct {
	EntityID int32
	Slots    []EquipmentSlot
}

// EquipmentSlot is one slot in the equipment packet.
type EquipmentSlot struct {
	Stack  inventory.ItemStack
	SlotID byte
	Empty  bool
	ItemID int32
	Count  int32
}

func NewEntityEquipment() protocol.Packet { return &EntityEquipment{} }

func (p *EntityEquipment) ID() int32 { return int32(packetid.ClientboundEntityEquipment) }

// Decode reads a bounded equipment list and preserves complete item stacks.
func (p *EntityEquipment) Decode(buf *protocol.Buffer) error {
	entityID, err := buf.ReadVarInt()
	if err != nil {
		return err
	}
	slots := make([]EquipmentSlot, 0, 7)
	var seen byte
	for {
		slotByte, err := buf.ReadByte()
		if err != nil {
			return err
		}
		id := slotByte & 0x7f
		if id > 6 || seen&(1<<id) != 0 {
			return fmt.Errorf("invalid or duplicate equipment slot %d", id)
		}
		seen |= 1 << id
		stack, err := inventory.DecodeItemStack(buf)
		if err != nil {
			return err
		}
		slots = append(slots, EquipmentSlot{SlotID: id, Stack: stack, Empty: stack.Empty(), ItemID: stack.ItemID, Count: stack.ItemCount})
		if slotByte&0x80 == 0 {
			p.EntityID, p.Slots = entityID, slots
			return nil
		}
		if len(slots) == 7 {
			return fmt.Errorf("equipment slots exceed limit")
		}
	}
}

func (p *EntityEquipment) Encode(buf *protocol.Buffer) error {
	if len(p.Slots) == 0 || len(p.Slots) > 7 {
		return fmt.Errorf("invalid equipment slots")
	}
	buf.WriteVarInt(p.EntityID)
	var seen byte
	for i, slot := range p.Slots {
		if slot.SlotID > 6 || seen&(1<<slot.SlotID) != 0 {
			return fmt.Errorf("invalid or duplicate equipment slot")
		}
		seen |= 1 << slot.SlotID
		slotByte := slot.SlotID
		if i < len(p.Slots)-1 {
			slotByte |= 0x80
		}
		buf.WriteByte(slotByte)
		if !slot.Stack.Empty() {
			if err := inventory.EncodeItemStack(buf, slot.Stack); err != nil {
				return err
			}
		} else if slot.Empty {
			buf.WriteVarInt(0)
		} else {
			if err := inventory.EncodeItemStack(buf, inventory.NewSlot(slot.ItemID, slot.Count)); err != nil {
				return err
			}
		}
	}
	return nil
}
