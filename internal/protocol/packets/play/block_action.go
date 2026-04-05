package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// BlockAction triggers a block event such as chest open/close, noteblock play, or piston extend.
type BlockAction struct {
	Position        BlockPos
	ActionID        byte
	ActionParameter byte
	BlockType       int32
}

func NewBlockAction() protocol.Packet { return &BlockAction{} }

func (p *BlockAction) ID() int32 {
	return int32(packetid.ClientboundBlockAction)
}

func (p *BlockAction) Decode(buf *protocol.Buffer) error {
	pos, err := buf.ReadInt64()
	if err != nil {
		return err
	}
	p.Position = UnpackBlockPos(pos)
	if p.ActionID, err = buf.ReadByte(); err != nil {
		return err
	}
	if p.ActionParameter, err = buf.ReadByte(); err != nil {
		return err
	}
	p.BlockType, err = buf.ReadVarInt()
	return err
}

func (p *BlockAction) Encode(buf *protocol.Buffer) error {
	buf.WriteInt64(p.Position.Packed())
	buf.WriteByte(p.ActionID)
	buf.WriteByte(p.ActionParameter)
	buf.WriteVarInt(p.BlockType)
	return nil
}
