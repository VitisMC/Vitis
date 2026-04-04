package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetPlayerPositionAndRotation is sent by the client when the player moves and rotates.
type SetPlayerPositionAndRotation struct {
	X        float64
	Y        float64
	Z        float64
	Yaw      float32
	Pitch    float32
	OnGround bool
}

// NewSetPlayerPositionAndRotation constructs an empty SetPlayerPositionAndRotation packet.
func NewSetPlayerPositionAndRotation() protocol.Packet {
	return &SetPlayerPositionAndRotation{}
}

// ID returns the protocol packet id.
func (p *SetPlayerPositionAndRotation) ID() int32 {
	return int32(packetid.ServerboundPositionLook)
}

// Decode reads SetPlayerPositionAndRotation fields from buffer.
func (p *SetPlayerPositionAndRotation) Decode(buf *protocol.Buffer) error {
	x, err := buf.ReadFloat64()
	if err != nil {
		return fmt.Errorf("decode set_player_position_and_rotation x: %w", err)
	}
	y, err := buf.ReadFloat64()
	if err != nil {
		return fmt.Errorf("decode set_player_position_and_rotation y: %w", err)
	}
	z, err := buf.ReadFloat64()
	if err != nil {
		return fmt.Errorf("decode set_player_position_and_rotation z: %w", err)
	}
	yaw, err := buf.ReadFloat32()
	if err != nil {
		return fmt.Errorf("decode set_player_position_and_rotation yaw: %w", err)
	}
	pitch, err := buf.ReadFloat32()
	if err != nil {
		return fmt.Errorf("decode set_player_position_and_rotation pitch: %w", err)
	}
	onGround, err := buf.ReadBool()
	if err != nil {
		return fmt.Errorf("decode set_player_position_and_rotation on_ground: %w", err)
	}
	p.X = x
	p.Y = y
	p.Z = z
	p.Yaw = yaw
	p.Pitch = pitch
	p.OnGround = onGround
	return nil
}

// Encode writes SetPlayerPositionAndRotation fields to buffer.
func (p *SetPlayerPositionAndRotation) Encode(buf *protocol.Buffer) error {
	buf.WriteFloat64(p.X)
	buf.WriteFloat64(p.Y)
	buf.WriteFloat64(p.Z)
	buf.WriteFloat32(p.Yaw)
	buf.WriteFloat32(p.Pitch)
	buf.WriteBool(p.OnGround)
	return nil
}
