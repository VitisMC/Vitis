package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// KeepAliveServerbound is sent by client to acknowledge keepalive challenge.
type KeepAliveServerbound struct {
	Value int64
}

// NewKeepAliveServerbound constructs an empty serverbound keepalive packet.
func NewKeepAliveServerbound() protocol.Packet {
	return &KeepAliveServerbound{}
}

// ID returns the protocol packet id.
func (p *KeepAliveServerbound) ID() int32 {
	return int32(packetid.ServerboundKeepAlive)
}

// Decode reads KeepAliveServerbound fields from buffer.
func (p *KeepAliveServerbound) Decode(buf *protocol.Buffer) error {
	value, err := buf.ReadInt64()
	if err != nil {
		return fmt.Errorf("decode keepalive value: %w", err)
	}
	p.Value = value
	return nil
}

// Encode writes KeepAliveServerbound fields to buffer.
func (p *KeepAliveServerbound) Encode(buf *protocol.Buffer) error {
	buf.WriteInt64(p.Value)
	return nil
}

// KeepAliveClientbound is sent by server to probe client liveness.
type KeepAliveClientbound struct {
	Value int64
}

// NewKeepAliveClientbound constructs an empty clientbound keepalive packet.
func NewKeepAliveClientbound() protocol.Packet {
	return &KeepAliveClientbound{}
}

// ID returns the protocol packet id.
func (p *KeepAliveClientbound) ID() int32 {
	return int32(packetid.ClientboundKeepAlive)
}

// Decode reads KeepAliveClientbound fields from buffer.
func (p *KeepAliveClientbound) Decode(buf *protocol.Buffer) error {
	value, err := buf.ReadInt64()
	if err != nil {
		return fmt.Errorf("decode keepalive value: %w", err)
	}
	p.Value = value
	return nil
}

// Encode writes KeepAliveClientbound fields to buffer.
func (p *KeepAliveClientbound) Encode(buf *protocol.Buffer) error {
	buf.WriteInt64(p.Value)
	return nil
}
