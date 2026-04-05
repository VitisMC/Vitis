package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ClientboundTags sends tag data to the client during play state.
type ClientboundTags struct {
	Data []byte
}

func NewClientboundTags() protocol.Packet { return &ClientboundTags{} }

func (p *ClientboundTags) ID() int32 {
	return int32(packetid.ClientboundTags)
}

func (p *ClientboundTags) Decode(buf *protocol.Buffer) error {
	remaining := buf.Remaining()
	if remaining > 0 {
		var err error
		p.Data, err = buf.ReadBytes(remaining)
		return err
	}
	return nil
}

func (p *ClientboundTags) Encode(buf *protocol.Buffer) error {
	if len(p.Data) > 0 {
		buf.WriteBytes(p.Data)
	}
	return nil
}
