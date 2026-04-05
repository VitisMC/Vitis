package configuration

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// Disconnect is sent by the server to disconnect the client during configuration.
type Disconnect struct {
	Reason string
}

func NewDisconnect() protocol.Packet { return &Disconnect{} }

func (p *Disconnect) ID() int32 {
	return int32(packetid.ClientboundConfigDisconnect)
}

func (p *Disconnect) Decode(buf *protocol.Buffer) error {
	var err error
	p.Reason, err = buf.ReadString()
	return err
}

func (p *Disconnect) Encode(buf *protocol.Buffer) error {
	buf.WriteString(p.Reason)
	return nil
}
