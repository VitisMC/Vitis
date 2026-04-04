package status

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// StatusRequest is a serverbound status-state request packet with an empty payload.
type StatusRequest struct{}

// NewStatusRequest constructs an empty status request packet.
func NewStatusRequest() protocol.Packet {
	return &StatusRequest{}
}

// ID returns the protocol packet id.
func (p *StatusRequest) ID() int32 {
	return int32(packetid.ServerboundStatusPingStart)
}

// Decode reads StatusRequest payload from buffer.
func (p *StatusRequest) Decode(_ *protocol.Buffer) error {
	return nil
}

// Encode writes StatusRequest payload to buffer.
func (p *StatusRequest) Encode(_ *protocol.Buffer) error {
	return nil
}
