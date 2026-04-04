package configuration

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// AcknowledgeFinishConfiguration is sent by the client to confirm configuration is complete.
type AcknowledgeFinishConfiguration struct{}

// NewAcknowledgeFinishConfiguration constructs an empty AcknowledgeFinishConfiguration packet.
func NewAcknowledgeFinishConfiguration() protocol.Packet {
	return &AcknowledgeFinishConfiguration{}
}

// ID returns the protocol packet id.
func (p *AcknowledgeFinishConfiguration) ID() int32 {
	return int32(packetid.ServerboundConfigFinishConfiguration)
}

// Decode reads AcknowledgeFinishConfiguration fields from buffer (no fields).
func (p *AcknowledgeFinishConfiguration) Decode(_ *protocol.Buffer) error {
	return nil
}

// Encode writes AcknowledgeFinishConfiguration fields to buffer (no fields).
func (p *AcknowledgeFinishConfiguration) Encode(_ *protocol.Buffer) error {
	return nil
}

// InboundStateTransition transitions to play state after receiving this packet.
func (p *AcknowledgeFinishConfiguration) InboundStateTransition() (protocol.State, bool) {
	return protocol.StatePlay, true
}
