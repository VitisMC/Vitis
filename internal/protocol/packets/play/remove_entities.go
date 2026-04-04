package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// RemoveEntities is the clientbound packet that despawns entities for the client.
type RemoveEntities struct {
	EntityIDs []int32
}

// NewRemoveEntities constructs an empty RemoveEntities packet.
func NewRemoveEntities() protocol.Packet {
	return &RemoveEntities{}
}

// ID returns the protocol packet id.
func (p *RemoveEntities) ID() int32 {
	return int32(packetid.ClientboundEntityDestroy)
}

// Decode reads RemoveEntities fields from buffer.
func (p *RemoveEntities) Decode(buf *protocol.Buffer) error {
	count, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode remove entities count: %w", err)
	}
	if count < 0 {
		return fmt.Errorf("decode remove entities: negative count %d", count)
	}
	p.EntityIDs = make([]int32, count)
	for i := int32(0); i < count; i++ {
		if p.EntityIDs[i], err = buf.ReadVarInt(); err != nil {
			return fmt.Errorf("decode remove entities id[%d]: %w", i, err)
		}
	}
	return nil
}

// Encode writes RemoveEntities fields to buffer.
func (p *RemoveEntities) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(int32(len(p.EntityIDs)))
	for _, id := range p.EntityIDs {
		buf.WriteVarInt(id)
	}
	return nil
}
