package play

import (
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// TickEnd is sent by the client at the end of each tick.
type TickEnd struct{}

func NewTickEnd() protocol.Packet                  { return &TickEnd{} }
func (p *TickEnd) ID() int32                       { return int32(packetid.ServerboundTickEnd) }
func (p *TickEnd) Decode(_ *protocol.Buffer) error { return nil }
func (p *TickEnd) Encode(_ *protocol.Buffer) error { return nil }

// ServerboundPingRequest is sent by the client to measure latency during play.
type ServerboundPingRequest struct {
	Payload int64
}

func NewServerboundPingRequest() protocol.Packet { return &ServerboundPingRequest{} }
func (p *ServerboundPingRequest) ID() int32      { return int32(packetid.ServerboundPingRequest) }
func (p *ServerboundPingRequest) Decode(buf *protocol.Buffer) error {
	var err error
	p.Payload, err = buf.ReadInt64()
	return err
}
func (p *ServerboundPingRequest) Encode(buf *protocol.Buffer) error {
	buf.WriteInt64(p.Payload)
	return nil
}

// ServerboundPong is sent in response to a play ping.
type ServerboundPong struct {
	PongID int32
}

func NewServerboundPong() protocol.Packet { return &ServerboundPong{} }
func (p *ServerboundPong) ID() int32      { return int32(packetid.ServerboundPong) }
func (p *ServerboundPong) Decode(buf *protocol.Buffer) error {
	var err error
	p.PongID, err = buf.ReadInt32()
	return err
}
func (p *ServerboundPong) Encode(buf *protocol.Buffer) error {
	buf.WriteInt32(p.PongID)
	return nil
}

// ServerboundCustomPayload is a play-state plugin message from the client.
type ServerboundPlayCustomPayload struct {
	Channel string
	Data    []byte
}

func NewServerboundPlayCustomPayload() protocol.Packet { return &ServerboundPlayCustomPayload{} }
func (p *ServerboundPlayCustomPayload) ID() int32 {
	return int32(packetid.ServerboundCustomPayload)
}
func (p *ServerboundPlayCustomPayload) Decode(buf *protocol.Buffer) error {
	var err error
	if p.Channel, err = buf.ReadString(); err != nil {
		return err
	}
	remaining := buf.Remaining()
	if remaining > 0 {
		p.Data, err = buf.ReadBytes(remaining)
	}
	return err
}
func (p *ServerboundPlayCustomPayload) Encode(buf *protocol.Buffer) error {
	buf.WriteString(p.Channel)
	buf.WriteBytes(p.Data)
	return nil
}

// WindowClick is sent by the client when clicking in a container window.
type WindowClick struct {
	WindowID     int32
	StateID      int32
	Slot         int16
	Button       int32
	Mode         int32
	ChangedSlots map[int16]inventory.Slot
	CarriedItem  inventory.Slot
}

func NewWindowClick() protocol.Packet { return &WindowClick{} }
func (p *WindowClick) ID() int32      { return int32(packetid.ServerboundWindowClick) }
func (p *WindowClick) Decode(buf *protocol.Buffer) error {
	var err error
	if p.WindowID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.StateID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.Slot, err = buf.ReadInt16(); err != nil {
		return err
	}
	b, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.Button = int32(b)
	if p.Mode, err = buf.ReadVarInt(); err != nil {
		return err
	}
	remaining := buf.Remaining()
	if remaining > 0 {
		_, err = buf.ReadBytes(remaining)
	}
	return err
}
func (p *WindowClick) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.WindowID)
	buf.WriteVarInt(p.StateID)
	buf.WriteInt16(p.Slot)
	buf.WriteByte(byte(p.Button))
	buf.WriteVarInt(p.Mode)
	buf.WriteVarInt(int32(len(p.ChangedSlots)))
	for idx, s := range p.ChangedSlots {
		buf.WriteInt16(idx)
		inventory.EncodeSlot(buf, s)
	}
	inventory.EncodeSlot(buf, p.CarriedItem)
	return nil
}

// CloseWindow is sent when the client closes a container window.
type CloseWindow struct {
	WindowID int32
}

func NewCloseWindow() protocol.Packet { return &CloseWindow{} }
func (p *CloseWindow) ID() int32      { return int32(packetid.ServerboundCloseWindow) }
func (p *CloseWindow) Decode(buf *protocol.Buffer) error {
	var err error
	p.WindowID, err = buf.ReadVarInt()
	return err
}
func (p *CloseWindow) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.WindowID)
	return nil
}

// PlayerInput is sent for vehicle control (WASD + jump + sneak).
type PlayerInput struct {
	Flags byte
}

func NewPlayerInput() protocol.Packet { return &PlayerInput{} }
func (p *PlayerInput) ID() int32      { return int32(packetid.ServerboundPlayerInput) }
func (p *PlayerInput) Decode(buf *protocol.Buffer) error {
	var err error
	p.Flags, err = buf.ReadByte()
	return err
}
func (p *PlayerInput) Encode(buf *protocol.Buffer) error {
	buf.WriteByte(p.Flags)
	return nil
}
