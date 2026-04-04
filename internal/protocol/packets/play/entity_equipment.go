package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// EntityEquipment sends an entity's equipment (armor, held items).
// Simplified: sends empty slots for now.
type EntityEquipment struct {
	EntityID int32
	Slots    []EquipmentSlot
}

// EquipmentSlot is one slot in the equipment packet.
type EquipmentSlot struct {
	SlotID byte
	Empty  bool
	ItemID int32
	Count  int32
}

func NewEntityEquipment() protocol.Packet { return &EntityEquipment{} }

func (p *EntityEquipment) ID() int32 { return int32(packetid.ClientboundEntityEquipment) }

func (p *EntityEquipment) Decode(_ *protocol.Buffer) error { return nil }

func (p *EntityEquipment) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	for i, slot := range p.Slots {
		slotByte := slot.SlotID
		if i < len(p.Slots)-1 {
			slotByte |= 0x80
		}
		buf.WriteByte(slotByte)
		if slot.Empty {
			buf.WriteVarInt(0)
		} else {
			buf.WriteVarInt(slot.Count)
			buf.WriteVarInt(slot.ItemID)
			buf.WriteVarInt(0)
			buf.WriteVarInt(0)
		}
	}
	return nil
}
