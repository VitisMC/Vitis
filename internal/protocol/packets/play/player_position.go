package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetPlayerPosition is sent by the client when the player moves without rotating.
type SetPlayerPosition struct {
	X        float64
	Y        float64
	Z        float64
	OnGround bool
}

// NewSetPlayerPosition constructs an empty SetPlayerPosition packet.
func NewSetPlayerPosition() protocol.Packet {
	return &SetPlayerPosition{}
}

// ID returns the protocol packet id.
func (p *SetPlayerPosition) ID() int32 {
	return int32(packetid.ServerboundPosition)
}

// Decode reads SetPlayerPosition fields from buffer.
func (p *SetPlayerPosition) Decode(buf *protocol.Buffer) error {
	x, err := buf.ReadFloat64()
	if err != nil {
		return fmt.Errorf("decode set_player_position x: %w", err)
	}
	y, err := buf.ReadFloat64()
	if err != nil {
		return fmt.Errorf("decode set_player_position y: %w", err)
	}
	z, err := buf.ReadFloat64()
	if err != nil {
		return fmt.Errorf("decode set_player_position z: %w", err)
	}
	onGround, err := buf.ReadBool()
	if err != nil {
		return fmt.Errorf("decode set_player_position on_ground: %w", err)
	}
	p.X = x
	p.Y = y
	p.Z = z
	p.OnGround = onGround
	return nil
}

// Encode writes SetPlayerPosition fields to buffer.
func (p *SetPlayerPosition) Encode(buf *protocol.Buffer) error {
	buf.WriteFloat64(p.X)
	buf.WriteFloat64(p.Y)
	buf.WriteFloat64(p.Z)
	buf.WriteBool(p.OnGround)
	return nil
}
