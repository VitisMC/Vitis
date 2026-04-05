package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ChunkBiomes sends biome data for chunks to the client.
type ChunkBiomes struct {
	Data []byte
}

func NewChunkBiomes() protocol.Packet { return &ChunkBiomes{} }

func (p *ChunkBiomes) ID() int32 {
	return int32(packetid.ClientboundChunkBiomes)
}

func (p *ChunkBiomes) Decode(buf *protocol.Buffer) error {
	remaining := buf.Remaining()
	if remaining > 0 {
		var err error
		p.Data, err = buf.ReadBytes(remaining)
		return err
	}
	return nil
}

func (p *ChunkBiomes) Encode(buf *protocol.Buffer) error {
	if len(p.Data) > 0 {
		buf.WriteBytes(p.Data)
	}
	return nil
}
