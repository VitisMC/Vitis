package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// UpdateLight sends light data updates for a chunk to the client.
type UpdateLight struct {
	ChunkX int32
	ChunkZ int32
	Data   []byte
}

func NewUpdateLight() protocol.Packet { return &UpdateLight{} }

func (p *UpdateLight) ID() int32 {
	return int32(packetid.ClientboundUpdateLight)
}

func (p *UpdateLight) Decode(buf *protocol.Buffer) error {
	var err error
	if p.ChunkX, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.ChunkZ, err = buf.ReadVarInt(); err != nil {
		return err
	}
	remaining := buf.Remaining()
	if remaining > 0 {
		p.Data, err = buf.ReadBytes(remaining)
	}
	return err
}

func (p *UpdateLight) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.ChunkX)
	buf.WriteVarInt(p.ChunkZ)
	if len(p.Data) > 0 {
		buf.WriteBytes(p.Data)
	}
	return nil
}
