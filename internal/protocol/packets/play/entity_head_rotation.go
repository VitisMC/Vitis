package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetHeadRotation is the clientbound head yaw update packet.
type SetHeadRotation struct {
	EntityID int32
	HeadYaw  byte
}

// NewSetHeadRotation constructs an empty SetHeadRotation packet.
func NewSetHeadRotation() protocol.Packet {
	return &SetHeadRotation{}
}

// ID returns the protocol packet id.
func (p *SetHeadRotation) ID() int32 {
	return int32(packetid.ClientboundEntityHeadRotation)
}

// Decode reads SetHeadRotation fields from buffer.
func (p *SetHeadRotation) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return fmt.Errorf("decode head rotation id: %w", err)
	}
	if p.HeadYaw, err = buf.ReadByte(); err != nil {
		return fmt.Errorf("decode head rotation yaw: %w", err)
	}
	return nil
}

// Encode writes SetHeadRotation fields to buffer.
func (p *SetHeadRotation) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	_ = buf.WriteByte(p.HeadYaw)
	return nil
}
