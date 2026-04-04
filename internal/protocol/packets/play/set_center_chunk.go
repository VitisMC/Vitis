package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetCenterChunk is sent by the server to set the center of the client's chunk loading area.
type SetCenterChunk struct {
	ChunkX int32
	ChunkZ int32
}

// NewSetCenterChunk constructs an empty SetCenterChunk packet.
func NewSetCenterChunk() protocol.Packet {
	return &SetCenterChunk{}
}

// ID returns the protocol packet id.
func (p *SetCenterChunk) ID() int32 {
	return int32(packetid.ClientboundUpdateViewPosition)
}

// Decode reads SetCenterChunk fields from buffer.
func (p *SetCenterChunk) Decode(buf *protocol.Buffer) error {
	x, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode set_center_chunk x: %w", err)
	}
	p.ChunkX = x

	z, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode set_center_chunk z: %w", err)
	}
	p.ChunkZ = z
	return nil
}

// Encode writes SetCenterChunk fields to buffer.
func (p *SetCenterChunk) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.ChunkX)
	buf.WriteVarInt(p.ChunkZ)
	return nil
}
