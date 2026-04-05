package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// PickItemFromBlock is sent when the client middle-clicks a block (pick block).
type PickItemFromBlock struct {
	X             int32
	Y             int32
	Z             int32
	IncludeData   bool
}

func NewPickItemFromBlock() protocol.Packet { return &PickItemFromBlock{} }

func (p *PickItemFromBlock) ID() int32 {
	return int32(packetid.ServerboundPickItemFromBlock)
}

func (p *PickItemFromBlock) Decode(buf *protocol.Buffer) error {
	pos, err := buf.ReadInt64()
	if err != nil {
		return err
	}
	bp := UnpackBlockPos(pos)
	p.X, p.Y, p.Z = bp.X, bp.Y, bp.Z
	p.IncludeData, err = buf.ReadBool()
	return err
}

func (p *PickItemFromBlock) Encode(buf *protocol.Buffer) error {
	bp := BlockPos{X: p.X, Y: p.Y, Z: p.Z}
	buf.WriteInt64(bp.Packed())
	buf.WriteBool(p.IncludeData)
	return nil
}

// PickItemFromEntity is sent when the client middle-clicks an entity.
type PickItemFromEntity struct {
	EntityID    int32
	IncludeData bool
}

func NewPickItemFromEntity() protocol.Packet { return &PickItemFromEntity{} }

func (p *PickItemFromEntity) ID() int32 {
	return int32(packetid.ServerboundPickItemFromEntity)
}

func (p *PickItemFromEntity) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	p.IncludeData, err = buf.ReadBool()
	return err
}

func (p *PickItemFromEntity) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteBool(p.IncludeData)
	return nil
}
