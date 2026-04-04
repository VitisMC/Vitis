package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ArmAnimation is sent when the player swings their arm.
type ArmAnimation struct {
	Hand int32
}

func NewArmAnimation() protocol.Packet { return &ArmAnimation{} }
func (p *ArmAnimation) ID() int32      { return int32(packetid.ServerboundArmAnimation) }
func (p *ArmAnimation) Decode(buf *protocol.Buffer) error {
	var err error
	p.Hand, err = buf.ReadVarInt()
	return err
}
func (p *ArmAnimation) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.Hand)
	return nil
}

// EntityAction is sent for sneak, sprint, jump with horse, etc.
type EntityAction struct {
	EntityID int32
	ActionID int32
	JumpBoost int32
}

func NewEntityAction() protocol.Packet { return &EntityAction{} }
func (p *EntityAction) ID() int32      { return int32(packetid.ServerboundEntityAction) }
func (p *EntityAction) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.ActionID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	p.JumpBoost, err = buf.ReadVarInt()
	return err
}
func (p *EntityAction) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteVarInt(p.ActionID)
	buf.WriteVarInt(p.JumpBoost)
	return nil
}

// ServerboundHeldItemSlot is sent when the player changes the selected hotbar slot.
type ServerboundHeldItemSlot struct {
	Slot int16
}

func NewServerboundHeldItemSlot() protocol.Packet { return &ServerboundHeldItemSlot{} }
func (p *ServerboundHeldItemSlot) ID() int32 {
	return int32(packetid.ServerboundHeldItemSlot)
}
func (p *ServerboundHeldItemSlot) Decode(buf *protocol.Buffer) error {
	var err error
	p.Slot, err = buf.ReadInt16()
	return err
}
func (p *ServerboundHeldItemSlot) Encode(buf *protocol.Buffer) error {
	buf.WriteInt16(p.Slot)
	return nil
}

// ClientCommand is sent for respawn or request stats.
type ClientCommand struct {
	ActionID int32
}

func NewClientCommand() protocol.Packet { return &ClientCommand{} }
func (p *ClientCommand) ID() int32      { return int32(packetid.ServerboundClientCommand) }
func (p *ClientCommand) Decode(buf *protocol.Buffer) error {
	var err error
	p.ActionID, err = buf.ReadVarInt()
	return err
}
func (p *ClientCommand) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.ActionID)
	return nil
}

// ServerboundSettings is sent by the client with its settings during play.
type ServerboundSettings struct {
	Locale       string
	ViewDistance  int8
	ChatMode     int32
	ChatColors   bool
	SkinParts    byte
	MainHand     int32
	TextFiltering bool
	AllowListing  bool
}

func NewServerboundSettings() protocol.Packet { return &ServerboundSettings{} }
func (p *ServerboundSettings) ID() int32      { return int32(packetid.ServerboundSettings) }
func (p *ServerboundSettings) Decode(buf *protocol.Buffer) error {
	var err error
	if p.Locale, err = buf.ReadString(); err != nil {
		return err
	}
	b, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.ViewDistance = int8(b)
	if p.ChatMode, err = buf.ReadVarInt(); err != nil {
		return err
	}
	cb, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.ChatColors = cb != 0
	sb, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.SkinParts = sb
	if p.MainHand, err = buf.ReadVarInt(); err != nil {
		return err
	}
	tf, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.TextFiltering = tf != 0
	al, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.AllowListing = al != 0
	return nil
}
func (p *ServerboundSettings) Encode(buf *protocol.Buffer) error {
	buf.WriteString(p.Locale)
	buf.WriteByte(byte(p.ViewDistance))
	buf.WriteVarInt(p.ChatMode)
	if p.ChatColors {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	buf.WriteByte(p.SkinParts)
	buf.WriteVarInt(p.MainHand)
	if p.TextFiltering {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	if p.AllowListing {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	return nil
}
