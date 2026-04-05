package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ClientboundPlayerRotation sets the player's rotation from the server.
type ClientboundPlayerRotation struct {
	Yaw   float32
	Pitch float32
}

func NewClientboundPlayerRotation() protocol.Packet { return &ClientboundPlayerRotation{} }

func (p *ClientboundPlayerRotation) ID() int32 {
	return int32(packetid.ClientboundPlayerRotation)
}

func (p *ClientboundPlayerRotation) Decode(buf *protocol.Buffer) error {
	var err error
	if p.Yaw, err = buf.ReadFloat32(); err != nil {
		return err
	}
	p.Pitch, err = buf.ReadFloat32()
	return err
}

func (p *ClientboundPlayerRotation) Encode(buf *protocol.Buffer) error {
	buf.WriteFloat32(p.Yaw)
	buf.WriteFloat32(p.Pitch)
	return nil
}
