package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// MultiBlockChange updates multiple blocks within a single 16x16x16 chunk section.
type MultiBlockChange struct {
	ChunkSectionPosition int64
	Blocks               []int64
}

func NewMultiBlockChange() protocol.Packet { return &MultiBlockChange{} }

func (p *MultiBlockChange) ID() int32 {
	return int32(packetid.ClientboundMultiBlockChange)
}

func (p *MultiBlockChange) Decode(buf *protocol.Buffer) error {
	var err error
	if p.ChunkSectionPosition, err = buf.ReadInt64(); err != nil {
		return err
	}
	count, err := buf.ReadVarInt()
	if err != nil {
		return err
	}
	p.Blocks = make([]int64, count)
	for i := int32(0); i < count; i++ {
		if p.Blocks[i], err = buf.ReadVarLong(); err != nil {
			return err
		}
	}
	return nil
}

func (p *MultiBlockChange) Encode(buf *protocol.Buffer) error {
	buf.WriteInt64(p.ChunkSectionPosition)
	buf.WriteVarInt(int32(len(p.Blocks)))
	for _, b := range p.Blocks {
		buf.WriteVarLong(b)
	}
	return nil
}
