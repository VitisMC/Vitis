package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetPlayerRotation is sent by the client when the player rotates without moving.
type SetPlayerRotation struct {
	Yaw                 float32
	Pitch               float32
	OnGround            bool
	HorizontalCollision bool
}

// NewSetPlayerRotation constructs an empty SetPlayerRotation packet.
func NewSetPlayerRotation() protocol.Packet {
	return &SetPlayerRotation{}
}

// ID returns the protocol packet id.
func (p *SetPlayerRotation) ID() int32 {
	return int32(packetid.ServerboundLook)
}

// Decode reads SetPlayerRotation fields from buffer.
func (p *SetPlayerRotation) Decode(buf *protocol.Buffer) error {
	yaw, err := buf.ReadFloat32()
	if err != nil {
		return fmt.Errorf("decode set_player_rotation yaw: %w", err)
	}
	pitch, err := buf.ReadFloat32()
	if err != nil {
		return fmt.Errorf("decode set_player_rotation pitch: %w", err)
	}
	flags, err := buf.ReadByte()
	if err != nil {
		return fmt.Errorf("decode set_player_rotation flags: %w", err)
	}
	if flags&^byte(0x03) != 0 {
		return fmt.Errorf("decode set_player_rotation flags: invalid flags 0x%02X", flags)
	}
	p.Yaw = yaw
	p.Pitch = pitch
	p.OnGround = flags&0x01 != 0
	p.HorizontalCollision = flags&0x02 != 0
	return nil
}

// Encode writes SetPlayerRotation fields to buffer.
func (p *SetPlayerRotation) Encode(buf *protocol.Buffer) error {
	buf.WriteFloat32(p.Yaw)
	buf.WriteFloat32(p.Pitch)
	buf.WriteByte(movementFlags(p.OnGround, p.HorizontalCollision))
	return nil
}
