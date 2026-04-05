package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ClientboundTransfer tells the client to connect to a different server during play.
type ClientboundTransfer struct {
	Host string
	Port int32
}

func NewClientboundTransfer() protocol.Packet { return &ClientboundTransfer{} }

func (p *ClientboundTransfer) ID() int32 {
	return int32(packetid.ClientboundTransfer)
}

func (p *ClientboundTransfer) Decode(buf *protocol.Buffer) error {
	var err error
	if p.Host, err = buf.ReadString(); err != nil {
		return err
	}
	p.Port, err = buf.ReadVarInt()
	return err
}

func (p *ClientboundTransfer) Encode(buf *protocol.Buffer) error {
	buf.WriteString(p.Host)
	buf.WriteVarInt(p.Port)
	return nil
}
