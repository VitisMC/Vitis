package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// OpenSignEntity tells the client to open the sign editor for a specific sign.
type OpenSignEntity struct {
	Position    BlockPos
	IsFrontText bool
}

func NewOpenSignEntity() protocol.Packet { return &OpenSignEntity{} }

func (p *OpenSignEntity) ID() int32 {
	return int32(packetid.ClientboundOpenSignEntity)
}

func (p *OpenSignEntity) Decode(buf *protocol.Buffer) error {
	pos, err := buf.ReadInt64()
	if err != nil {
		return err
	}
	p.Position = UnpackBlockPos(pos)
	p.IsFrontText, err = buf.ReadBool()
	return err
}

func (p *OpenSignEntity) Encode(buf *protocol.Buffer) error {
	buf.WriteInt64(p.Position.Packed())
	buf.WriteBool(p.IsFrontText)
	return nil
}
