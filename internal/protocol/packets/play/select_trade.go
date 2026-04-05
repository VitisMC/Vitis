package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SelectTrade is sent when the client selects a villager trade offer.
type SelectTrade struct {
	SelectedSlot int32
}

func NewSelectTrade() protocol.Packet { return &SelectTrade{} }

func (p *SelectTrade) ID() int32 {
	return int32(packetid.ServerboundSelectTrade)
}

func (p *SelectTrade) Decode(buf *protocol.Buffer) error {
	var err error
	p.SelectedSlot, err = buf.ReadVarInt()
	return err
}

func (p *SelectTrade) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.SelectedSlot)
	return nil
}
