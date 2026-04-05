package configuration

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// StoreCookie tells the client to store a cookie during configuration.
type StoreCookie struct {
	Key  string
	Data []byte
}

func NewStoreCookie() protocol.Packet { return &StoreCookie{} }

func (p *StoreCookie) ID() int32 {
	return int32(packetid.ClientboundConfigStoreCookie)
}

func (p *StoreCookie) Decode(buf *protocol.Buffer) error {
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

func (p *StoreCookie) Encode(buf *protocol.Buffer) error {
	buf.WriteString(p.Key)
	buf.WriteVarInt(int32(len(p.Data)))
	buf.WriteBytes(p.Data)
	return nil
}
