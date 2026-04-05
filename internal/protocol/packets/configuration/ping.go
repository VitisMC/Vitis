package configuration

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// Ping is sent by the server to the client during configuration to measure latency.
type Ping struct {
	PingID int32
}

func NewPing() protocol.Packet { return &Ping{} }

func (p *Ping) ID() int32 {
	return int32(packetid.ClientboundConfigPing)
}

func (p *Ping) Decode(buf *protocol.Buffer) error {
	var err error
	p.PingID, err = buf.ReadInt32()
	return err
}

func (p *Ping) Encode(buf *protocol.Buffer) error {
	buf.WriteInt32(p.PingID)
	return nil
}

// Pong is sent by the client in response to a ping during configuration.
type Pong struct {
	PingID int32
}

func NewPong() protocol.Packet { return &Pong{} }

func (p *Pong) ID() int32 {
	return int32(packetid.ServerboundConfigPong)
}

func (p *Pong) Decode(buf *protocol.Buffer) error {
	var err error
	p.PingID, err = buf.ReadInt32()
	return err
}

func (p *Pong) Encode(buf *protocol.Buffer) error {
	buf.WriteInt32(p.PingID)
	return nil
}
