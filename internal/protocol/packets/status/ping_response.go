package status

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// PingResponse is a clientbound status-state pong response packet carrying an int64 timestamp.
type PingResponse struct {
	Payload int64
}

// NewPingResponse constructs an empty ping response packet.
func NewPingResponse() protocol.Packet {
	return &PingResponse{}
}

// ID returns the protocol packet id.
func (p *PingResponse) ID() int32 {
	return int32(packetid.ClientboundStatusPing)
}

// Decode reads PingResponse payload from buffer.
func (p *PingResponse) Decode(buf *protocol.Buffer) error {
	payload, err := buf.ReadInt64()
	if err != nil {
		return fmt.Errorf("decode ping payload: %w", err)
	}
	p.Payload = payload
	return nil
}

// Encode writes PingResponse payload to buffer.
func (p *PingResponse) Encode(buf *protocol.Buffer) error {
	buf.WriteInt64(p.Payload)
	return nil
}
