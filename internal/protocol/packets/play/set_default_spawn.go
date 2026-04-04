package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetDefaultSpawnPosition is sent by the server to set the world spawn point and compass target.
type SetDefaultSpawnPosition struct {
	X     int32
	Y     int32
	Z     int32
	Angle float32
}

// NewSetDefaultSpawnPosition constructs an empty SetDefaultSpawnPosition packet.
func NewSetDefaultSpawnPosition() protocol.Packet {
	return &SetDefaultSpawnPosition{}
}

// ID returns the protocol packet id.
func (p *SetDefaultSpawnPosition) ID() int32 {
	return int32(packetid.ClientboundSpawnPosition)
}

// Decode reads SetDefaultSpawnPosition fields from buffer.
func (p *SetDefaultSpawnPosition) Decode(buf *protocol.Buffer) error {
	posVal, err := buf.ReadInt64()
	if err != nil {
		return fmt.Errorf("decode set_default_spawn location: %w", err)
	}
	p.X = int32(posVal >> 38)
	p.Y = int32(posVal << 52 >> 52)
	p.Z = int32(posVal << 26 >> 38)

	angle, err := buf.ReadFloat32()
	if err != nil {
		return fmt.Errorf("decode set_default_spawn angle: %w", err)
	}
	p.Angle = angle
	return nil
}

// Encode writes SetDefaultSpawnPosition fields to buffer.
func (p *SetDefaultSpawnPosition) Encode(buf *protocol.Buffer) error {
	posVal := (int64(p.X&0x3FFFFFF) << 38) |
		(int64(p.Z&0x3FFFFFF) << 12) |
		int64(p.Y&0xFFF)
	buf.WriteInt64(posVal)
	buf.WriteFloat32(p.Angle)
	return nil
}
