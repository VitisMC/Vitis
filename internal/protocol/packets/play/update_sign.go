package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// UpdateSign is sent by the client when finishing editing a sign.
type UpdateSign struct {
	Position    BlockPos
	IsFrontText bool
	Line1       string
	Line2       string
	Line3       string
	Line4       string
}

func NewUpdateSign() protocol.Packet { return &UpdateSign{} }

func (p *UpdateSign) ID() int32 {
	return int32(packetid.ServerboundUpdateSign)
}

func (p *UpdateSign) Decode(buf *protocol.Buffer) error {
	var err error
	pos, err := buf.ReadInt64()
	if err != nil {
		return err
	}
	p.Position = UnpackBlockPos(pos)
	if p.IsFrontText, err = buf.ReadBool(); err != nil {
		return err
	}
	if p.Line1, err = buf.ReadString(); err != nil {
		return err
	}
	if p.Line2, err = buf.ReadString(); err != nil {
		return err
	}
	if p.Line3, err = buf.ReadString(); err != nil {
		return err
	}
	p.Line4, err = buf.ReadString()
	return err
}

func (p *UpdateSign) Encode(buf *protocol.Buffer) error {
	buf.WriteInt64(p.Position.Packed())
	buf.WriteBool(p.IsFrontText)
	buf.WriteString(p.Line1)
	buf.WriteString(p.Line2)
	buf.WriteString(p.Line3)
	buf.WriteString(p.Line4)
	return nil
}
