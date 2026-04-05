package configuration

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ClientboundKeepAlive is sent by the server to check if the client is still connected during configuration.
type ClientboundKeepAlive struct {
	KeepAliveID int64
}

func NewClientboundKeepAlive() protocol.Packet { return &ClientboundKeepAlive{} }

func (p *ClientboundKeepAlive) ID() int32 {
	return int32(packetid.ClientboundConfigKeepAlive)
}

func (p *ClientboundKeepAlive) Decode(buf *protocol.Buffer) error {
	var err error
	p.KeepAliveID, err = buf.ReadInt64()
	return err
}

func (p *ClientboundKeepAlive) Encode(buf *protocol.Buffer) error {
	buf.WriteInt64(p.KeepAliveID)
	return nil
}

// ServerboundKeepAlive is sent by the client in response to a keep alive during configuration.
type ServerboundKeepAlive struct {
	KeepAliveID int64
}

func NewServerboundKeepAlive() protocol.Packet { return &ServerboundKeepAlive{} }

func (p *ServerboundKeepAlive) ID() int32 {
	return int32(packetid.ServerboundConfigKeepAlive)
}

func (p *ServerboundKeepAlive) Decode(buf *protocol.Buffer) error {
	var err error
	p.KeepAliveID, err = buf.ReadInt64()
	return err
}

func (p *ServerboundKeepAlive) Encode(buf *protocol.Buffer) error {
	buf.WriteInt64(p.KeepAliveID)
	return nil
}
