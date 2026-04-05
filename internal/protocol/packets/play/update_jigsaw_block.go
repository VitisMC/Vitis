package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// UpdateJigsawBlock is sent by the client to update a jigsaw block.
type UpdateJigsawBlock struct {
	Position    BlockPos
	Name        string
	Target      string
	Pool        string
	FinalState  string
	JointType   string
	SelectionPriority int32
	PlacementPriority int32
}

func NewUpdateJigsawBlock() protocol.Packet { return &UpdateJigsawBlock{} }

func (p *UpdateJigsawBlock) ID() int32 {
	return int32(packetid.ServerboundUpdateJigsawBlock)
}

func (p *UpdateJigsawBlock) Decode(buf *protocol.Buffer) error {
	pos, err := buf.ReadInt64()
	if err != nil {
		return err
	}
	p.Position = UnpackBlockPos(pos)
	if p.Name, err = buf.ReadString(); err != nil {
		return err
	}
	if p.Target, err = buf.ReadString(); err != nil {
		return err
	}
	if p.Pool, err = buf.ReadString(); err != nil {
		return err
	}
	if p.FinalState, err = buf.ReadString(); err != nil {
		return err
	}
	if p.JointType, err = buf.ReadString(); err != nil {
		return err
	}
	if p.SelectionPriority, err = buf.ReadVarInt(); err != nil {
		return err
	}
	p.PlacementPriority, err = buf.ReadVarInt()
	return err
}

func (p *UpdateJigsawBlock) Encode(buf *protocol.Buffer) error {
	buf.WriteInt64(p.Position.Packed())
	buf.WriteString(p.Name)
	buf.WriteString(p.Target)
	buf.WriteString(p.Pool)
	buf.WriteString(p.FinalState)
	buf.WriteString(p.JointType)
	buf.WriteVarInt(p.SelectionPriority)
	buf.WriteVarInt(p.PlacementPriority)
	return nil
}
