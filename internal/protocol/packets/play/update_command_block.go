package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// UpdateCommandBlock is sent by the client to update a command block.
type UpdateCommandBlock struct {
	Position BlockPos
	Command  string
	Mode     int32
	Flags    byte
}

func NewUpdateCommandBlock() protocol.Packet { return &UpdateCommandBlock{} }

func (p *UpdateCommandBlock) ID() int32 {
	return int32(packetid.ServerboundUpdateCommandBlock)
}

func (p *UpdateCommandBlock) Decode(buf *protocol.Buffer) error {
	pos, err := buf.ReadInt64()
	if err != nil {
		return err
	}
	p.Position = UnpackBlockPos(pos)
	if p.Command, err = buf.ReadString(); err != nil {
		return err
	}
	if p.Mode, err = buf.ReadVarInt(); err != nil {
		return err
	}
	p.Flags, err = buf.ReadByte()
	return err
}

func (p *UpdateCommandBlock) Encode(buf *protocol.Buffer) error {
	buf.WriteInt64(p.Position.Packed())
	buf.WriteString(p.Command)
	buf.WriteVarInt(p.Mode)
	buf.WriteByte(p.Flags)
	return nil
}

// UpdateCommandBlockMinecart is sent by the client to update a command block minecart.
type UpdateCommandBlockMinecart struct {
	EntityID    int32
	Command     string
	TrackOutput bool
}

func NewUpdateCommandBlockMinecart() protocol.Packet { return &UpdateCommandBlockMinecart{} }

func (p *UpdateCommandBlockMinecart) ID() int32 {
	return int32(packetid.ServerboundUpdateCommandBlockMinecart)
}

func (p *UpdateCommandBlockMinecart) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.Command, err = buf.ReadString(); err != nil {
		return err
	}
	p.TrackOutput, err = buf.ReadBool()
	return err
}

func (p *UpdateCommandBlockMinecart) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteString(p.Command)
	buf.WriteBool(p.TrackOutput)
	return nil
}
