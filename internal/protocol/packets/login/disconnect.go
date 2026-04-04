package login

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// Disconnect is sent by the server to kick a client during the login phase.
type Disconnect struct {
	Reason string
}

// NewDisconnect constructs an empty login Disconnect packet.
func NewDisconnect() protocol.Packet {
	return &Disconnect{}
}

// ID returns the protocol packet id.
func (p *Disconnect) ID() int32 {
	return int32(packetid.ClientboundLoginDisconnect)
}

// Decode reads Disconnect fields from buffer.
func (p *Disconnect) Decode(buf *protocol.Buffer) error {
	reason, err := buf.ReadString()
	if err != nil {
		return fmt.Errorf("decode disconnect reason: %w", err)
	}
	p.Reason = reason
	return nil
}

// Encode writes Disconnect fields to buffer.
func (p *Disconnect) Encode(buf *protocol.Buffer) error {
	if err := buf.WriteString(p.Reason); err != nil {
		return fmt.Errorf("encode disconnect reason: %w", err)
	}
	return nil
}
