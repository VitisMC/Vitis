package configuration

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ClientboundPluginMessage is sent by the server to deliver plugin channel data during configuration.
type ClientboundPluginMessage struct {
	Channel string
	Data    []byte
}

// NewClientboundPluginMessage constructs an empty ClientboundPluginMessage packet.
func NewClientboundPluginMessage() protocol.Packet {
	return &ClientboundPluginMessage{}
}

// ID returns the protocol packet id.
func (p *ClientboundPluginMessage) ID() int32 {
	return int32(packetid.ClientboundConfigCustomPayload)
}

// Decode reads ClientboundPluginMessage fields from buffer.
func (p *ClientboundPluginMessage) Decode(buf *protocol.Buffer) error {
	channel, err := buf.ReadString()
	if err != nil {
		return fmt.Errorf("decode plugin_message channel: %w", err)
	}
	p.Channel = channel
	remaining := buf.Remaining()
	if remaining > 0 {
		data, err := buf.ReadBytes(remaining)
		if err != nil {
			return fmt.Errorf("decode plugin_message data: %w", err)
		}
		p.Data = data
	}
	return nil
}

// Encode writes ClientboundPluginMessage fields to buffer.
func (p *ClientboundPluginMessage) Encode(buf *protocol.Buffer) error {
	if err := buf.WriteString(p.Channel); err != nil {
		return fmt.Errorf("encode plugin_message channel: %w", err)
	}
	buf.WriteBytes(p.Data)
	return nil
}

// ServerboundPluginMessage is sent by the client to deliver plugin channel data during configuration.
type ServerboundPluginMessage struct {
	Channel string
	Data    []byte
}

// NewServerboundPluginMessage constructs an empty ServerboundPluginMessage packet.
func NewServerboundPluginMessage() protocol.Packet {
	return &ServerboundPluginMessage{}
}

// ID returns the protocol packet id.
func (p *ServerboundPluginMessage) ID() int32 {
	return int32(packetid.ServerboundConfigCustomPayload)
}

// Decode reads ServerboundPluginMessage fields from buffer.
func (p *ServerboundPluginMessage) Decode(buf *protocol.Buffer) error {
	channel, err := buf.ReadString()
	if err != nil {
		return fmt.Errorf("decode plugin_message channel: %w", err)
	}
	p.Channel = channel
	remaining := buf.Remaining()
	if remaining > 0 {
		data, err := buf.ReadBytes(remaining)
		if err != nil {
			return fmt.Errorf("decode plugin_message data: %w", err)
		}
		p.Data = data
	}
	return nil
}

// Encode writes ServerboundPluginMessage fields to buffer.
func (p *ServerboundPluginMessage) Encode(buf *protocol.Buffer) error {
	if err := buf.WriteString(p.Channel); err != nil {
		return fmt.Errorf("encode plugin_message channel: %w", err)
	}
	buf.WriteBytes(p.Data)
	return nil
}
