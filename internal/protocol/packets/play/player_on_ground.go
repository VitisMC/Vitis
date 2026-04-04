package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetPlayerOnGround is sent by the client when the player's on-ground status changes.
type SetPlayerOnGround struct {
	OnGround bool
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
	onGround, err := buf.ReadBool()
	if err != nil {
		return fmt.Errorf("decode set_player_on_ground on_ground: %w", err)
	}
	p.OnGround = onGround
	return nil
}

// Encode writes SetPlayerOnGround fields to buffer.
func (p *SetPlayerOnGround) Encode(buf *protocol.Buffer) error {
	buf.WriteBool(p.OnGround)
	return nil
}
