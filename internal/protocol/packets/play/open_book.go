package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// OpenBook tells the client to open a written book.
type OpenBook struct {
	Hand int32
}

func NewOpenBook() protocol.Packet { return &OpenBook{} }

func (p *OpenBook) ID() int32 {
	return int32(packetid.ClientboundOpenBook)
}

func (p *OpenBook) Decode(buf *protocol.Buffer) error {
	var err error
	p.Hand, err = buf.ReadVarInt()
	return err
}

func (p *OpenBook) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.Hand)
	return nil
}
