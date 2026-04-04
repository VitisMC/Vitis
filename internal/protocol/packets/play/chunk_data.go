package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ChunkDataAndUpdateLight sends chunk column data and lighting to the client.
// Payload contains everything after ChunkX/ChunkZ pre-serialized:
// heightmaps NBT + VarInt(section_data_len) + section_data +
// VarInt(num_block_entities) + block_entities + light data.
type ChunkDataAndUpdateLight struct {
	ChunkX  int32
	ChunkZ  int32
	Payload []byte
}

// NewChunkDataAndUpdateLight creates a new ChunkDataAndUpdateLight packet.
func NewChunkDataAndUpdateLight() protocol.Packet {
	return &ChunkDataAndUpdateLight{}
}

// ID returns the packet identifier.
func (p *ChunkDataAndUpdateLight) ID() int32 {
	return int32(packetid.ClientboundMapChunk)
}

// Decode reads packet fields from a protocol buffer.
func (p *ChunkDataAndUpdateLight) Decode(buf *protocol.Buffer) error {
	var err error
	if p.ChunkX, err = buf.ReadInt32(); err != nil {
		return err
	}
	if p.ChunkZ, err = buf.ReadInt32(); err != nil {
		return err
	}
	remaining := buf.Remaining()
	if remaining > 0 {
		raw, readErr := buf.ReadBytes(remaining)
		if readErr != nil {
			return readErr
		}
		p.Payload = make([]byte, len(raw))
		copy(p.Payload, raw)
	}
	return nil
}

// Encode writes packet fields to a protocol buffer.
func (p *ChunkDataAndUpdateLight) Encode(buf *protocol.Buffer) error {
	buf.WriteInt32(p.ChunkX)
	buf.WriteInt32(p.ChunkZ)
	buf.WriteBytes(p.Payload)
	return nil
}
