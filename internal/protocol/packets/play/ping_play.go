package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ClientboundPing sends a ping to the client during play state.
type ClientboundPing struct {
	PingID int32
}

func NewClientboundPing() protocol.Packet { return &ClientboundPing{} }

func (p *ClientboundPing) ID() int32 {
	return int32(packetid.ClientboundPing)
}

func (p *ClientboundPing) Decode(buf *protocol.Buffer) error {
	var err error
	p.PingID, err = buf.ReadInt32()
	return err
}

func (p *ClientboundPing) Encode(buf *protocol.Buffer) error {
	buf.WriteInt32(p.PingID)
	return nil
}

// ClientboundPingResponse sends a ping response to the client during play state.
type ClientboundPingResponse struct {
	Payload int64
}

func NewClientboundPingResponse() protocol.Packet { return &ClientboundPingResponse{} }

func (p *ClientboundPingResponse) ID() int32 {
	return int32(packetid.ClientboundPingResponse)
}

func (p *ClientboundPingResponse) Decode(buf *protocol.Buffer) error {
	var err error
	p.Payload, err = buf.ReadInt64()
	return err
}

func (p *ClientboundPingResponse) Encode(buf *protocol.Buffer) error {
	buf.WriteInt64(p.Payload)
	return nil
}
