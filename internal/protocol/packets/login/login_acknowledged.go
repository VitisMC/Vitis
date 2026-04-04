package login

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// LoginAcknowledged is sent by the client to acknowledge Login Success, transitioning to Configuration.
type LoginAcknowledged struct{}

// NewLoginAcknowledged constructs an empty LoginAcknowledged packet.
func NewLoginAcknowledged() protocol.Packet {
	return &LoginAcknowledged{}
}

// ID returns the protocol packet id.
func (p *LoginAcknowledged) ID() int32 {
	return int32(packetid.ServerboundLoginLoginAcknowledged)
}

// Decode reads LoginAcknowledged payload from buffer.
func (p *LoginAcknowledged) Decode(_ *protocol.Buffer) error {
	return nil
}

// Encode writes LoginAcknowledged payload to buffer.
func (p *LoginAcknowledged) Encode(_ *protocol.Buffer) error {
	return nil
}

// InboundStateTransition transitions the session to Configuration after login acknowledgement.
func (p *LoginAcknowledged) InboundStateTransition() (protocol.State, bool) {
	return protocol.StateConfiguration, true
}
