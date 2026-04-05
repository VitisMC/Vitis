package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// GenerateStructure is sent by the client to generate a structure from a jigsaw block.
type GenerateStructure struct {
	Position  BlockPos
	Levels    int32
	KeepJigsaws bool
}

func NewGenerateStructure() protocol.Packet { return &GenerateStructure{} }

func (p *GenerateStructure) ID() int32 {
	return int32(packetid.ServerboundGenerateStructure)
}

func (p *GenerateStructure) Decode(buf *protocol.Buffer) error {
	pos, err := buf.ReadInt64()
	if err != nil {
		return err
	}
	p.Position = UnpackBlockPos(pos)
	if p.Levels, err = buf.ReadVarInt(); err != nil {
		return err
	}
	p.KeepJigsaws, err = buf.ReadBool()
	return err
}

func (p *GenerateStructure) Encode(buf *protocol.Buffer) error {
	buf.WriteInt64(p.Position.Packed())
	buf.WriteVarInt(p.Levels)
	buf.WriteBool(p.KeepJigsaws)
	return nil
}
