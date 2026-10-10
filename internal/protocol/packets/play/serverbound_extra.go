package play

import (
	"fmt"
	"strings"

	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

const maxPlayCustomPayloadLength = 32767

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
	if p.Channel, err = buf.ReadStringLimit(256); err != nil {
		return err
	}
	if !validPayloadChannel(p.Channel) {
		return fmt.Errorf("invalid custom payload channel %q", p.Channel)
	}
	remaining := buf.Remaining()
	if remaining > maxPlayCustomPayloadLength {
		return fmt.Errorf("custom payload exceeds %d byte limit", maxPlayCustomPayloadLength)
	}
	if remaining > 0 {
		p.Data, err = buf.ReadBytes(remaining)
	}
	return err
}

func validPayloadChannel(channel string) bool {
	if channel == "" || strings.Count(channel, ":") != 1 {
		return false
	}
	for _, part := range strings.Split(channel, ":") {
		if part == "" {
			return false
		}
		for _, r := range part {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' || r == '/' {
				continue
			}
			return false
		}
	}
	return true
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
	changedCount, err := buf.ReadVarInt()
	if err != nil {
		return err
	}
	if changedCount < 0 || changedCount > 128 {
		return fmt.Errorf("window click changed slot count out of range: %d", changedCount)
	}
	p.ChangedSlots = make(map[int16]inventory.Slot, changedCount)
	for index := int32(0); index < changedCount; index++ {
		slotIndex, readErr := buf.ReadInt16()
		if readErr != nil {
			return readErr
		}
		stack, readErr := inventory.DecodeSlotWithoutComponents(buf)
		if readErr != nil {
			return fmt.Errorf("decode changed slot %d: %w", slotIndex, readErr)
		}
		p.ChangedSlots[slotIndex] = stack
	}
	p.CarriedItem, err = inventory.DecodeSlotWithoutComponents(buf)
	if err != nil {
		return fmt.Errorf("decode carried item: %w", err)
	}
	if buf.Remaining() != 0 {
		return fmt.Errorf("window click trailing bytes: %d", buf.Remaining())
	}
	return nil
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
		if err := inventory.EncodeItemStack(buf, s); err != nil {
			return err
		}
	}
	return inventory.EncodeItemStack(buf, p.CarriedItem)
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

// PlayerInput contains the player's movement key state.
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
