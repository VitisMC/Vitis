package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// UpdateStructureBlock is sent by the client to update a structure block.
type UpdateStructureBlock struct {
	Position  BlockPos
	Action    int32
	Mode      int32
	Name      string
	OffsetX   int8
	OffsetY   int8
	OffsetZ   int8
	SizeX     int8
	SizeY     int8
	SizeZ     int8
	Mirror    int32
	Rotation  int32
	Metadata  string
	Integrity float32
	Seed      int64
	Flags     byte
}

func NewUpdateStructureBlock() protocol.Packet { return &UpdateStructureBlock{} }

func (p *UpdateStructureBlock) ID() int32 {
	return int32(packetid.ServerboundUpdateStructureBlock)
}

func (p *UpdateStructureBlock) Decode(buf *protocol.Buffer) error {
	pos, err := buf.ReadInt64()
	if err != nil {
		return err
	}
	p.Position = UnpackBlockPos(pos)
	if p.Action, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.Mode, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.Name, err = buf.ReadString(); err != nil {
		return err
	}
	b, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.OffsetX = int8(b)
	if b, err = buf.ReadByte(); err != nil {
		return err
	}
	p.OffsetY = int8(b)
	if b, err = buf.ReadByte(); err != nil {
		return err
	}
	p.OffsetZ = int8(b)
	if b, err = buf.ReadByte(); err != nil {
		return err
	}
	p.SizeX = int8(b)
	if b, err = buf.ReadByte(); err != nil {
		return err
	}
	p.SizeY = int8(b)
	if b, err = buf.ReadByte(); err != nil {
		return err
	}
	p.SizeZ = int8(b)
	if p.Mirror, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.Rotation, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.Metadata, err = buf.ReadString(); err != nil {
		return err
	}
	if p.Integrity, err = buf.ReadFloat32(); err != nil {
		return err
	}
	if p.Seed, err = buf.ReadInt64(); err != nil {
		return err
	}
	p.Flags, err = buf.ReadByte()
	return err
}

func (p *UpdateStructureBlock) Encode(buf *protocol.Buffer) error {
	buf.WriteInt64(p.Position.Packed())
	buf.WriteVarInt(p.Action)
	buf.WriteVarInt(p.Mode)
	buf.WriteString(p.Name)
	buf.WriteByte(byte(p.OffsetX))
	buf.WriteByte(byte(p.OffsetY))
	buf.WriteByte(byte(p.OffsetZ))
	buf.WriteByte(byte(p.SizeX))
	buf.WriteByte(byte(p.SizeY))
	buf.WriteByte(byte(p.SizeZ))
	buf.WriteVarInt(p.Mirror)
	buf.WriteVarInt(p.Rotation)
	buf.WriteString(p.Metadata)
	buf.WriteFloat32(p.Integrity)
	buf.WriteInt64(p.Seed)
	buf.WriteByte(p.Flags)
	return nil
}
