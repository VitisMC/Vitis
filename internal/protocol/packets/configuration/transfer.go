package configuration

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// Transfer tells the client to connect to a different server during configuration.
type Transfer struct {
	Host string
	Port int32
}

func NewTransfer() protocol.Packet { return &Transfer{} }

func (p *Transfer) ID() int32 {
	return int32(packetid.ClientboundConfigTransfer)
}

func (p *Transfer) Decode(buf *protocol.Buffer) error {
	var err error
	if p.Host, err = buf.ReadString(); err != nil {
		return err
	}
	p.Port, err = buf.ReadVarInt()
	return err
}

func (p *Transfer) Encode(buf *protocol.Buffer) error {
	buf.WriteString(p.Host)
	buf.WriteVarInt(p.Port)
	return nil
}
