package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// PlayerChat sends a signed player chat message to the client.
type PlayerChat struct {
	Data []byte
}

func NewPlayerChat() protocol.Packet { return &PlayerChat{} }

func (p *PlayerChat) ID() int32 {
	return int32(packetid.ClientboundPlayerChat)
}

func (p *PlayerChat) Decode(buf *protocol.Buffer) error {
	remaining := buf.Remaining()
	if remaining > 0 {
		var err error
		p.Data, err = buf.ReadBytes(remaining)
		return err
	}
	return nil
}

func (p *PlayerChat) Encode(buf *protocol.Buffer) error {
	if len(p.Data) > 0 {
		buf.WriteBytes(p.Data)
	}
	return nil
}
