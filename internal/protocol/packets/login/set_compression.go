package login

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetCompression is sent by the server to enable protocol compression.
type SetCompression struct {
	Threshold int32
}

// NewSetCompression constructs an empty SetCompression packet.
func NewSetCompression() protocol.Packet {
	return &SetCompression{}
}

// ID returns the protocol packet id.
func (p *SetCompression) ID() int32 {
	return int32(packetid.ClientboundLoginCompress)
}

// Decode reads SetCompression fields from buffer.
func (p *SetCompression) Decode(buf *protocol.Buffer) error {
	threshold, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode set_compression threshold: %w", err)
	}
	p.Threshold = threshold
	return nil
}

// Encode writes SetCompression fields to buffer.
func (p *SetCompression) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.Threshold)
	return nil
}
