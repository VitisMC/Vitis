package status

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// PingRequest is a serverbound status-state ping packet carrying an int64 timestamp.
type PingRequest struct {
	Payload int64
}

// NewPingRequest constructs an empty ping request packet.
func NewPingRequest() protocol.Packet {
	return &PingRequest{}
}

// ID returns the protocol packet id.
func (p *PingRequest) ID() int32 {
	return int32(packetid.ServerboundStatusPing)
}

// Decode reads PingRequest payload from buffer.
func (p *PingRequest) Decode(buf *protocol.Buffer) error {
	payload, err := buf.ReadInt64()
	if err != nil {
		return fmt.Errorf("decode ping payload: %w", err)
	}
	p.Payload = payload
	return nil
}

// Encode writes PingRequest payload to buffer.
func (p *PingRequest) Encode(buf *protocol.Buffer) error {
	buf.WriteInt64(p.Payload)
	return nil
}
