package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// UseItemOn is sent by the client when placing a block or using an item on a block.
// In 1.21.4, WorldBorderHit boolean was added between InsideBlock and Sequence.
type UseItemOn struct {
	Hand           int32
	Position       BlockPos
	Face           int32
	CursorX        float32
	CursorY        float32
	CursorZ        float32
	InsideBlock    bool
	WorldBorderHit bool
	Sequence       int32
}

func NewUseItemOn() protocol.Packet { return &UseItemOn{} }

func (p *UseItemOn) ID() int32 { return int32(packetid.ServerboundBlockPlace) }

func (p *UseItemOn) Decode(buf *protocol.Buffer) error {
	var err error
	if p.Hand, err = buf.ReadVarInt(); err != nil {
		return err
	}
	pos, err := buf.ReadInt64()
	if err != nil {
		return err
	}
	p.Position = UnpackBlockPos(pos)
	if p.Face, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.CursorX, err = buf.ReadFloat32(); err != nil {
		return err
	}
	if p.CursorY, err = buf.ReadFloat32(); err != nil {
		return err
	}
	if p.CursorZ, err = buf.ReadFloat32(); err != nil {
		return err
	}
	b, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.InsideBlock = b != 0
	wb, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.WorldBorderHit = wb != 0
	p.Sequence, err = buf.ReadVarInt()
	return err
}

func (p *UseItemOn) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.Hand)
	buf.WriteInt64(p.Position.Packed())
	buf.WriteVarInt(p.Face)
	buf.WriteFloat32(p.CursorX)
	buf.WriteFloat32(p.CursorY)
	buf.WriteFloat32(p.CursorZ)
	if p.InsideBlock {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	buf.WriteVarInt(p.Sequence)
	return nil
}
