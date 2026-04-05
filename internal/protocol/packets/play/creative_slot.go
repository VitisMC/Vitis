package play

import (
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetCreativeSlot is sent by the client in creative mode when picking/placing items.
type SetCreativeSlot struct {
	SlotIndex int16
	Item      inventory.Slot
}

func NewSetCreativeSlot() protocol.Packet { return &SetCreativeSlot{} }

func (p *SetCreativeSlot) ID() int32 { return int32(packetid.ServerboundSetCreativeSlot) }

func (p *SetCreativeSlot) Decode(buf *protocol.Buffer) error {
	var err error
	if p.SlotIndex, err = buf.ReadInt16(); err != nil {
		return err
	}
	p.Item, err = inventory.DecodeSlot(buf)
	return err
}

func (p *SetCreativeSlot) Encode(buf *protocol.Buffer) error {
	buf.WriteInt16(p.SlotIndex)
	inventory.EncodeSlot(buf, p.Item)
	return nil
}

// UseItem is sent by the client when using an item (eating, drinking, etc).
type UseItem struct {
	Hand     int32
	Sequence int32
	Yaw      float32
	Pitch    float32
}

func NewUseItem() protocol.Packet { return &UseItem{} }

func (p *UseItem) ID() int32 { return int32(packetid.ServerboundUseItem) }

func (p *UseItem) Decode(buf *protocol.Buffer) error {
	var err error
	if p.Hand, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.Sequence, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.Yaw, err = buf.ReadFloat32(); err != nil {
		return err
	}
	p.Pitch, err = buf.ReadFloat32()
	return err
}

func (p *UseItem) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.Hand)
	buf.WriteVarInt(p.Sequence)
	buf.WriteFloat32(p.Yaw)
	buf.WriteFloat32(p.Pitch)
	return nil
}
