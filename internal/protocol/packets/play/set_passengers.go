package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetPassengers (clientbound) sets the passengers of an entity (vehicle).
type SetPassengers struct {
	EntityID   int32
	Passengers []int32
}

func NewSetPassengers() protocol.Packet { return &SetPassengers{} }

func (p *SetPassengers) ID() int32 {
	return int32(packetid.ClientboundSetPassengers)
}

func (p *SetPassengers) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return fmt.Errorf("decode set passengers entity: %w", err)
	}
	count, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode set passengers count: %w", err)
	}
	p.Passengers = make([]int32, count)
	for i := int32(0); i < count; i++ {
		if p.Passengers[i], err = buf.ReadVarInt(); err != nil {
			return fmt.Errorf("decode set passengers[%d]: %w", i, err)
		}
	}
	return nil
}

func (p *SetPassengers) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteVarInt(int32(len(p.Passengers)))
	for _, id := range p.Passengers {
		buf.WriteVarInt(id)
	}
	return nil
}
