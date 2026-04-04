package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetEntityMetadata is the clientbound entity metadata packet.
// Metadata is stored as raw bytes; full metadata parsing is not yet implemented.
type SetEntityMetadata struct {
	EntityID int32
	Metadata []byte
}

// NewSetEntityMetadata constructs an empty SetEntityMetadata packet.
func NewSetEntityMetadata() protocol.Packet {
	return &SetEntityMetadata{}
}

// ID returns the protocol packet id.
func (p *SetEntityMetadata) ID() int32 {
	return int32(packetid.ClientboundEntityMetadata)
}

// Decode reads SetEntityMetadata fields from buffer.
func (p *SetEntityMetadata) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return fmt.Errorf("decode entity metadata id: %w", err)
	}
	remaining := buf.Remaining()
	if remaining > 0 {
		raw, readErr := buf.ReadBytes(remaining)
		if readErr != nil {
			return fmt.Errorf("decode entity metadata payload: %w", readErr)
		}
		p.Metadata = make([]byte, len(raw))
		copy(p.Metadata, raw)
	}
	return nil
}

// Encode writes SetEntityMetadata fields to buffer.
func (p *SetEntityMetadata) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	if len(p.Metadata) > 0 {
		buf.WriteBytes(p.Metadata)
	} else {
		_ = buf.WriteByte(0xFF)
	}
	return nil
}
