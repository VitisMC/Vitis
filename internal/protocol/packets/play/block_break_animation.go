package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// BlockBreakAnimation shows mining progress overlay on a block to other players.
type BlockBreakAnimation struct {
	EntityID     int32
	Position     BlockPos
	DestroyStage int8
}

func NewBlockBreakAnimation() protocol.Packet { return &BlockBreakAnimation{} }

func (p *BlockBreakAnimation) ID() int32 {
	return int32(packetid.ClientboundBlockBreakAnimation)
}

func (p *BlockBreakAnimation) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	pos, err := buf.ReadInt64()
	if err != nil {
		return err
	}
	p.Position = UnpackBlockPos(pos)
	b, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.DestroyStage = int8(b)
	return nil
}

func (p *BlockBreakAnimation) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteInt64(p.Position.Packed())
	buf.WriteByte(byte(p.DestroyStage))
	return nil
}
