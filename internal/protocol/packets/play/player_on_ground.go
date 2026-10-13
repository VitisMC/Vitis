package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetPlayerOnGround is sent by the client when the player's on-ground status changes.
type SetPlayerOnGround struct {
	OnGround            bool
	HorizontalCollision bool
}

// NewSetPlayerOnGround constructs an empty SetPlayerOnGround packet.
func NewSetPlayerOnGround() protocol.Packet {
	return &SetPlayerOnGround{}
}

// ID returns the protocol packet id.
func (p *SetPlayerOnGround) ID() int32 {
	return int32(packetid.ServerboundFlying)
}

// Decode reads SetPlayerOnGround fields from buffer.
func (p *SetPlayerOnGround) Decode(buf *protocol.Buffer) error {
	flags, err := buf.ReadByte()
	if err != nil {
		return fmt.Errorf("decode set_player_on_ground flags: %w", err)
	}
	if flags&^byte(0x03) != 0 {
		return fmt.Errorf("decode set_player_on_ground flags: invalid flags 0x%02X", flags)
	}
	p.OnGround = flags&0x01 != 0
	p.HorizontalCollision = flags&0x02 != 0
	return nil
}

// Encode writes SetPlayerOnGround fields to buffer.
func (p *SetPlayerOnGround) Encode(buf *protocol.Buffer) error {
	buf.WriteByte(movementFlags(p.OnGround, p.HorizontalCollision))
	return nil
}

// movementFlags encodes the Minecraft 1.21.4 movement state bit field.
func movementFlags(onGround, horizontalCollision bool) byte {
	var flags byte
	if onGround {
		flags |= 0x01
	}
	if horizontalCollision {
		flags |= 0x02
	}
	return flags
}
