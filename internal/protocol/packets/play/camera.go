package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// Camera sets the camera entity for the client (e.g. spectator mode).
type Camera struct {
	CameraID int32
}

func NewCamera() protocol.Packet { return &Camera{} }

func (p *Camera) ID() int32 {
	return int32(packetid.ClientboundCamera)
}

func (p *Camera) Decode(buf *protocol.Buffer) error {
	var err error
	p.CameraID, err = buf.ReadVarInt()
	return err
}

func (p *Camera) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.CameraID)
	return nil
}
