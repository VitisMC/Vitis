package play

import (
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetCursorItem tells the client what item is on the cursor.
type SetCursorItem struct {
	SlotData inventory.Slot
}

func NewSetCursorItem() protocol.Packet { return &SetCursorItem{} }
func (p *SetCursorItem) ID() int32 {
	return int32(packetid.ClientboundSetCursorItem)
}
func (p *SetCursorItem) Decode(_ *protocol.Buffer) error { return nil }
func (p *SetCursorItem) Encode(buf *protocol.Buffer) error {
	inventory.EncodeSlot(buf, p.SlotData)
	return nil
}

// SetPlayerInventorySlot updates a single slot in the player inventory.
type SetPlayerInventorySlot struct {
	SlotIdx  int32
	SlotData inventory.Slot
}

func NewSetPlayerInventorySlot() protocol.Packet { return &SetPlayerInventorySlot{} }
func (p *SetPlayerInventorySlot) ID() int32 {
	return int32(packetid.ClientboundSetPlayerInventory)
}
func (p *SetPlayerInventorySlot) Decode(_ *protocol.Buffer) error { return nil }
func (p *SetPlayerInventorySlot) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.SlotIdx)
	inventory.EncodeSlot(buf, p.SlotData)
	return nil
}

// ClientboundHeldItemSlot tells the client which hotbar slot is selected.
type ClientboundHeldItemSlot struct {
	Slot int32
}

func NewClientboundHeldItemSlot() protocol.Packet { return &ClientboundHeldItemSlot{} }
func (p *ClientboundHeldItemSlot) ID() int32 {
	return int32(packetid.ClientboundHeldItemSlot)
}
func (p *ClientboundHeldItemSlot) Decode(_ *protocol.Buffer) error { return nil }
func (p *ClientboundHeldItemSlot) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.Slot)
	return nil
}
