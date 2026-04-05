package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ClientboundStoreCookie tells the client to store a cookie during play.
type ClientboundStoreCookie struct {
	Key  string
	Data []byte
}

func NewClientboundStoreCookie() protocol.Packet { return &ClientboundStoreCookie{} }

func (p *ClientboundStoreCookie) ID() int32 {
	return int32(packetid.ClientboundStoreCookie)
}

func (p *ClientboundStoreCookie) Decode(buf *protocol.Buffer) error {
	var err error
	if p.Key, err = buf.ReadString(); err != nil {
		return err
	}
	length, err := buf.ReadVarInt()
	if err != nil {
		return err
	}
	p.Data, err = buf.ReadBytes(int(length))
	return err
}

func (p *ClientboundStoreCookie) Encode(buf *protocol.Buffer) error {
	buf.WriteString(p.Key)
	buf.WriteVarInt(int32(len(p.Data)))
	buf.WriteBytes(p.Data)
	return nil
}
