package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// UnloadChunk tells the client to unload a chunk column.
type UnloadChunk struct {
	ChunkX int32
	ChunkZ int32
}

// NewUnloadChunk creates a new UnloadChunk packet.
func NewUnloadChunk() protocol.Packet {
	return &UnloadChunk{}
}

// ID returns the packet identifier.
func (p *UnloadChunk) ID() int32 {
	return int32(packetid.ClientboundUnloadChunk)
}

// Decode reads packet fields from a protocol buffer.
func (p *UnloadChunk) Decode(buf *protocol.Buffer) error {
	position, err := buf.ReadInt64()
	if err != nil {
		return err
	}
	p.ChunkX = int32(uint32(position))
	p.ChunkZ = int32(uint32(uint64(position) >> 32))
	return nil
}

// Encode writes packet fields to a protocol buffer.
func (p *UnloadChunk) Encode(buf *protocol.Buffer) error {
	position := int64(uint64(uint32(p.ChunkX)) | uint64(uint32(p.ChunkZ))<<32)
	buf.WriteInt64(position)
	return nil
}
