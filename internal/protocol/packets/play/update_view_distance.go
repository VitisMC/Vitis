package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// UpdateViewDistance sets the view distance for the client.
type UpdateViewDistance struct {
	ViewDistance int32
}

func NewUpdateViewDistance() protocol.Packet { return &UpdateViewDistance{} }

func (p *UpdateViewDistance) ID() int32 {
	return int32(packetid.ClientboundUpdateViewDistance)
}

func (p *UpdateViewDistance) Decode(buf *protocol.Buffer) error {
	var err error
	p.ViewDistance, err = buf.ReadVarInt()
	return err
}

func (p *UpdateViewDistance) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.ViewDistance)
	return nil
}
