package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ConfirmTeleportation is sent by the client to confirm a server-issued teleport.
type ConfirmTeleportation struct {
	TeleportID int32
}

// NewConfirmTeleportation constructs an empty ConfirmTeleportation packet.
func NewConfirmTeleportation() protocol.Packet {
	return &ConfirmTeleportation{}
}

// ID returns the protocol packet id.
func (p *ConfirmTeleportation) ID() int32 {
	return int32(packetid.ServerboundTeleportConfirm)
}

// Decode reads ConfirmTeleportation fields from buffer.
func (p *ConfirmTeleportation) Decode(buf *protocol.Buffer) error {
	id, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode confirm_teleportation teleport_id: %w", err)
	}
	p.TeleportID = id
	return nil
}

// Encode writes ConfirmTeleportation fields to buffer.
func (p *ConfirmTeleportation) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.TeleportID)
	return nil
}
