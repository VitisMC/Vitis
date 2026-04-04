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
	var err error
	if p.ChunkX, err = buf.ReadInt32(); err != nil {
		return err
	}
	if p.ChunkZ, err = buf.ReadInt32(); err != nil {
		return err
	}
	return nil
}

// Encode writes packet fields to a protocol buffer.
func (p *UnloadChunk) Encode(buf *protocol.Buffer) error {
	buf.WriteInt32(p.ChunkX)
	buf.WriteInt32(p.ChunkZ)
	return nil
}
