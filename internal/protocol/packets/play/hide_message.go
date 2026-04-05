package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// HideMessage tells the client to hide a previously received chat message.
type HideMessage struct {
	Data []byte
}

func NewHideMessage() protocol.Packet { return &HideMessage{} }

func (p *HideMessage) ID() int32 {
	return int32(packetid.ClientboundHideMessage)
}

func (p *HideMessage) Decode(buf *protocol.Buffer) error {
	remaining := buf.Remaining()
	if remaining > 0 {
		var err error
		p.Data, err = buf.ReadBytes(remaining)
		return err
	}
	return nil
}

func (p *HideMessage) Encode(buf *protocol.Buffer) error {
	if len(p.Data) > 0 {
		buf.WriteBytes(p.Data)
	}
	return nil
}
