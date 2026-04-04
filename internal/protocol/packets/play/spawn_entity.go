package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SpawnEntity is the clientbound packet that spawns an entity for the client.
type SpawnEntity struct {
	EntityID   int32
	EntityUUID protocol.UUID
	Type       int32
	X          float64
	Y          float64
	Z          float64
	Pitch      byte
	Yaw        byte
	HeadYaw    byte
	Data       int32
	VelocityX  int16
	VelocityY  int16
	VelocityZ  int16
}

// NewSpawnEntity constructs an empty SpawnEntity packet.
func NewSpawnEntity() protocol.Packet {
	return &SpawnEntity{}
}

// ID returns the protocol packet id.
func (p *SpawnEntity) ID() int32 {
	return int32(packetid.ClientboundSpawnEntity)
}

// Decode reads SpawnEntity fields from buffer.
func (p *SpawnEntity) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return fmt.Errorf("decode spawn entity id: %w", err)
	}
	if p.EntityUUID, err = buf.ReadUUID(); err != nil {
		return fmt.Errorf("decode spawn entity uuid: %w", err)
	}
	if p.Type, err = buf.ReadVarInt(); err != nil {
		return fmt.Errorf("decode spawn entity type: %w", err)
	}
	if p.X, err = buf.ReadFloat64(); err != nil {
		return fmt.Errorf("decode spawn entity x: %w", err)
	}
	if p.Y, err = buf.ReadFloat64(); err != nil {
		return fmt.Errorf("decode spawn entity y: %w", err)
	}
	if p.Z, err = buf.ReadFloat64(); err != nil {
		return fmt.Errorf("decode spawn entity z: %w", err)
	}
	if p.Pitch, err = buf.ReadByte(); err != nil {
		return fmt.Errorf("decode spawn entity pitch: %w", err)
	}
	if p.Yaw, err = buf.ReadByte(); err != nil {
		return fmt.Errorf("decode spawn entity yaw: %w", err)
	}
	if p.HeadYaw, err = buf.ReadByte(); err != nil {
		return fmt.Errorf("decode spawn entity head yaw: %w", err)
	}
	if p.Data, err = buf.ReadVarInt(); err != nil {
		return fmt.Errorf("decode spawn entity data: %w", err)
	}
	if p.VelocityX, err = buf.ReadInt16(); err != nil {
		return fmt.Errorf("decode spawn entity velocity x: %w", err)
	}
	if p.VelocityY, err = buf.ReadInt16(); err != nil {
		return fmt.Errorf("decode spawn entity velocity y: %w", err)
	}
	if p.VelocityZ, err = buf.ReadInt16(); err != nil {
		return fmt.Errorf("decode spawn entity velocity z: %w", err)
	}
	return nil
}

// Encode writes SpawnEntity fields to buffer.
func (p *SpawnEntity) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteUUID(p.EntityUUID)
	buf.WriteVarInt(p.Type)
	buf.WriteFloat64(p.X)
	buf.WriteFloat64(p.Y)
	buf.WriteFloat64(p.Z)
	_ = buf.WriteByte(p.Pitch)
	_ = buf.WriteByte(p.Yaw)
	_ = buf.WriteByte(p.HeadYaw)
	buf.WriteVarInt(p.Data)
	buf.WriteInt16(p.VelocityX)
	buf.WriteInt16(p.VelocityY)
	buf.WriteInt16(p.VelocityZ)
	return nil
}
