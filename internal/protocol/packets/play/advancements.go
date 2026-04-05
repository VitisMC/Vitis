package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// Advancements sends advancement progress data to the client.
type Advancements struct {
	Data []byte
}

func NewAdvancements() protocol.Packet { return &Advancements{} }

func (p *Advancements) ID() int32 {
	return int32(packetid.ClientboundAdvancements)
}

func (p *Advancements) Decode(buf *protocol.Buffer) error {
	remaining := buf.Remaining()
	if remaining > 0 {
		var err error
		p.Data, err = buf.ReadBytes(remaining)
		return err
	}
	return nil
}

func (p *Advancements) Encode(buf *protocol.Buffer) error {
	if len(p.Data) > 0 {
		buf.WriteBytes(p.Data)
	}
	return nil
}
