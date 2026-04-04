package login

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// LoginStart is sent by the client to initiate login.
type LoginStart struct {
	Name       string
	PlayerUUID protocol.UUID
}

// NewLoginStart constructs an empty LoginStart packet.
func NewLoginStart() protocol.Packet {
	return &LoginStart{}
}

// ID returns the protocol packet id.
func (p *LoginStart) ID() int32 {
	return int32(packetid.ServerboundLoginLoginStart)
}

// Decode reads LoginStart fields from buffer.
func (p *LoginStart) Decode(buf *protocol.Buffer) error {
	name, err := buf.ReadString()
	if err != nil {
		return fmt.Errorf("decode username: %w", err)
	}
	uuid, err := buf.ReadUUID()
	if err != nil {
		return fmt.Errorf("decode player uuid: %w", err)
	}
	p.Name = name
	p.PlayerUUID = uuid
	return nil
}

// Encode writes LoginStart fields to buffer.
func (p *LoginStart) Encode(buf *protocol.Buffer) error {
	if err := buf.WriteString(p.Name); err != nil {
		return fmt.Errorf("encode username: %w", err)
	}
	buf.WriteUUID(p.PlayerUUID)
	return nil
}
