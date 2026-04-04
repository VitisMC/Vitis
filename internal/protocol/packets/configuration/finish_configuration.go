package configuration

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// FinishConfiguration is sent by the server to tell the client configuration is complete.
type FinishConfiguration struct{}

// NewFinishConfiguration constructs an empty FinishConfiguration packet.
func NewFinishConfiguration() protocol.Packet {
	return &FinishConfiguration{}
}

// ID returns the protocol packet id.
func (p *FinishConfiguration) ID() int32 {
	return int32(packetid.ClientboundConfigFinishConfiguration)
}

// Decode reads FinishConfiguration fields from buffer (no fields).
func (p *FinishConfiguration) Decode(_ *protocol.Buffer) error {
	return nil
}

// Encode writes FinishConfiguration fields to buffer (no fields).
func (p *FinishConfiguration) Encode(_ *protocol.Buffer) error {
	return nil
}
