package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// TeleportEntity is the clientbound absolute position teleport packet.
type TeleportEntity struct {
	EntityID  int32
	X         float64
	Y         float64
	Z         float64
	VelocityX float64
	VelocityY float64
	VelocityZ float64
	Yaw       float32
	Pitch     float32
	Flags     int32
	OnGround  bool
}

// NewTeleportEntity constructs an empty TeleportEntity packet.
func NewTeleportEntity() protocol.Packet {
	return &TeleportEntity{}
}

// ID returns the protocol packet id.
func (p *TeleportEntity) ID() int32 {
	return int32(packetid.ClientboundEntityTeleport)
}

// Decode reads TeleportEntity fields from buffer.
func (p *TeleportEntity) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return fmt.Errorf("decode teleport entity id: %w", err)
	}
	if p.X, err = buf.ReadFloat64(); err != nil {
		return fmt.Errorf("decode teleport entity x: %w", err)
	}
	if p.Y, err = buf.ReadFloat64(); err != nil {
		return fmt.Errorf("decode teleport entity y: %w", err)
	}
	if p.Z, err = buf.ReadFloat64(); err != nil {
		return fmt.Errorf("decode teleport entity z: %w", err)
	}
	if p.VelocityX, err = buf.ReadFloat64(); err != nil {
		return fmt.Errorf("decode teleport entity velocity x: %w", err)
	}
	if p.VelocityY, err = buf.ReadFloat64(); err != nil {
		return fmt.Errorf("decode teleport entity velocity y: %w", err)
	}
	if p.VelocityZ, err = buf.ReadFloat64(); err != nil {
		return fmt.Errorf("decode teleport entity velocity z: %w", err)
	}
	if p.Yaw, err = buf.ReadFloat32(); err != nil {
		return fmt.Errorf("decode teleport entity yaw: %w", err)
	}
	if p.Pitch, err = buf.ReadFloat32(); err != nil {
		return fmt.Errorf("decode teleport entity pitch: %w", err)
	}
	if p.Flags, err = buf.ReadInt32(); err != nil {
		return fmt.Errorf("decode teleport entity flags: %w", err)
	}
	if p.OnGround, err = buf.ReadBool(); err != nil {
		return fmt.Errorf("decode teleport entity on_ground: %w", err)
	}
	return nil
}

// Encode writes TeleportEntity fields to buffer.
func (p *TeleportEntity) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteFloat64(p.X)
	buf.WriteFloat64(p.Y)
	buf.WriteFloat64(p.Z)
	buf.WriteFloat64(p.VelocityX)
	buf.WriteFloat64(p.VelocityY)
	buf.WriteFloat64(p.VelocityZ)
	buf.WriteFloat32(p.Yaw)
	buf.WriteFloat32(p.Pitch)
	buf.WriteInt32(p.Flags)
	buf.WriteBool(p.OnGround)
	return nil
}
