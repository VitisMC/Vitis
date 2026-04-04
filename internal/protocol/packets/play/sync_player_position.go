package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SyncPlayerPosition is sent by the server to teleport the client to a position.
type SyncPlayerPosition struct {
	TeleportID int32
	X          float64
	Y          float64
	Z          float64
	VelocityX  float64
	VelocityY  float64
	VelocityZ  float64
	Yaw        float32
	Pitch      float32
	Flags      int32
}

// NewSyncPlayerPosition constructs an empty SyncPlayerPosition packet.
func NewSyncPlayerPosition() protocol.Packet {
	return &SyncPlayerPosition{}
}

// ID returns the protocol packet id.
func (p *SyncPlayerPosition) ID() int32 {
	return int32(packetid.ClientboundPosition)
}

// Decode reads SyncPlayerPosition fields from buffer.
func (p *SyncPlayerPosition) Decode(buf *protocol.Buffer) error {
	teleportID, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode sync_player_position teleport_id: %w", err)
	}
	p.TeleportID = teleportID

	x, err := buf.ReadFloat64()
	if err != nil {
		return fmt.Errorf("decode sync_player_position x: %w", err)
	}
	p.X = x

	y, err := buf.ReadFloat64()
	if err != nil {
		return fmt.Errorf("decode sync_player_position y: %w", err)
	}
	p.Y = y

	z, err := buf.ReadFloat64()
	if err != nil {
		return fmt.Errorf("decode sync_player_position z: %w", err)
	}
	p.Z = z

	vx, err := buf.ReadFloat64()
	if err != nil {
		return fmt.Errorf("decode sync_player_position velocity_x: %w", err)
	}
	p.VelocityX = vx

	vy, err := buf.ReadFloat64()
	if err != nil {
		return fmt.Errorf("decode sync_player_position velocity_y: %w", err)
	}
	p.VelocityY = vy

	vz, err := buf.ReadFloat64()
	if err != nil {
		return fmt.Errorf("decode sync_player_position velocity_z: %w", err)
	}
	p.VelocityZ = vz

	yaw, err := buf.ReadFloat32()
	if err != nil {
		return fmt.Errorf("decode sync_player_position yaw: %w", err)
	}
	p.Yaw = yaw

	pitch, err := buf.ReadFloat32()
	if err != nil {
		return fmt.Errorf("decode sync_player_position pitch: %w", err)
	}
	p.Pitch = pitch

	flags, err := buf.ReadInt32()
	if err != nil {
		return fmt.Errorf("decode sync_player_position flags: %w", err)
	}
	p.Flags = flags

	return nil
}

// Encode writes SyncPlayerPosition fields to buffer.
func (p *SyncPlayerPosition) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.TeleportID)
	buf.WriteFloat64(p.X)
	buf.WriteFloat64(p.Y)
	buf.WriteFloat64(p.Z)
	buf.WriteFloat64(p.VelocityX)
	buf.WriteFloat64(p.VelocityY)
	buf.WriteFloat64(p.VelocityZ)
	buf.WriteFloat32(p.Yaw)
	buf.WriteFloat32(p.Pitch)
	buf.WriteInt32(p.Flags)
	return nil
}
