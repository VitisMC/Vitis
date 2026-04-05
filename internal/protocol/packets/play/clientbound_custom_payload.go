package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ClientboundCustomPayload sends plugin channel data to the client.
type ClientboundCustomPayload struct {
	Channel string
	Data    []byte
}

func NewClientboundCustomPayload() protocol.Packet { return &ClientboundCustomPayload{} }

func (p *ClientboundCustomPayload) ID() int32 {
	return int32(packetid.ClientboundCustomPayload)
}

func (p *ClientboundCustomPayload) Decode(buf *protocol.Buffer) error {
	var err error
	if p.Channel, err = buf.ReadString(); err != nil {
		return err
	}
	remaining := buf.Remaining()
	if remaining > 0 {
		p.Data, err = buf.ReadBytes(remaining)
	}
	return err
}

func (p *ClientboundCustomPayload) Encode(buf *protocol.Buffer) error {
	buf.WriteString(p.Channel)
	if len(p.Data) > 0 {
		buf.WriteBytes(p.Data)
	}
	return nil
}
