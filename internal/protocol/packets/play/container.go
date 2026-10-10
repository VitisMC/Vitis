package play

import (
	"fmt"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetContainerContent sends the full contents of a container window.
type SetContainerContent struct {
	WindowID int32
	StateID  int32
	Slots    []inventory.Slot
	Carried  inventory.Slot
}

func NewSetContainerContent() protocol.Packet { return &SetContainerContent{} }
func (p *SetContainerContent) ID() int32 {
	return int32(packetid.ClientboundWindowItems)
}
func (p *SetContainerContent) Decode(_ *protocol.Buffer) error { return nil }
func (p *SetContainerContent) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.WindowID)
	buf.WriteVarInt(p.StateID)
	buf.WriteVarInt(int32(len(p.Slots)))
	for _, s := range p.Slots {
		if err := inventory.EncodeItemStack(buf, s); err != nil {
			return err
		}
	}
	return inventory.EncodeItemStack(buf, p.Carried)
}

// SetContainerSlot updates a single slot in a container window.
type SetContainerSlot struct {
	WindowID int32
	StateID  int32
	SlotIdx  int16
	SlotData inventory.Slot
}

func NewSetContainerSlot() protocol.Packet { return &SetContainerSlot{} }
func (p *SetContainerSlot) ID() int32 {
	return int32(packetid.ClientboundSetSlot)
}
func (p *SetContainerSlot) Decode(_ *protocol.Buffer) error { return nil }
func (p *SetContainerSlot) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.WindowID)
	buf.WriteVarInt(p.StateID)
	buf.WriteInt16(p.SlotIdx)
	return inventory.EncodeItemStack(buf, p.SlotData)
}

// ClientboundCloseContainer tells the client to close a container window.
type ClientboundCloseContainer struct {
	WindowID int32
}

func NewClientboundCloseContainer() protocol.Packet { return &ClientboundCloseContainer{} }
func (p *ClientboundCloseContainer) ID() int32 {
	return int32(packetid.ClientboundCloseWindow)
}
func (p *ClientboundCloseContainer) Decode(_ *protocol.Buffer) error { return nil }
func (p *ClientboundCloseContainer) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.WindowID)
	return nil
}

// SetContainerProperty updates a property in a container window (e.g., furnace progress).
// Property indices for furnace-like blocks:
// 0: Fire icon (fuel left) - current fuel burn time remaining
// 1: Maximum fuel burn time
// 2: Progress arrow - current cooking progress
// 3: Maximum progress (always 200 for vanilla furnaces)
type SetContainerProperty struct {
	WindowID int32
	Property int16
	Value    int16
}

func NewSetContainerProperty() protocol.Packet { return &SetContainerProperty{} }
func (p *SetContainerProperty) ID() int32 {
	return int32(packetid.ClientboundCraftProgressBar)
}
func (p *SetContainerProperty) Decode(buf *protocol.Buffer) error {
	var err error
	p.WindowID, err = buf.ReadVarInt()
	if err != nil {
		return err
	}
	if p.WindowID < 0 {
		return fmt.Errorf("invalid container ID")
	}
	p.Property, err = buf.ReadInt16()
	if err != nil {
		return err
	}
	p.Value, err = buf.ReadInt16()
	return err
}
func (p *SetContainerProperty) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(int32(p.WindowID))
	buf.WriteInt16(p.Property)
	buf.WriteInt16(p.Value)
	return nil
}

// OpenScreen tells the client to open a container window.
type OpenScreen struct {
	WindowID   int32
	WindowType int32
	Title      string
}

func NewOpenScreen() protocol.Packet { return &OpenScreen{} }
func (p *OpenScreen) ID() int32 {
	return int32(packetid.ClientboundOpenWindow)
}
func (p *OpenScreen) Decode(_ *protocol.Buffer) error { return nil }
func (p *OpenScreen) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.WindowID)
	buf.WriteVarInt(p.WindowType)
	return writeNBTTextComponent(buf, p.Title)
}
