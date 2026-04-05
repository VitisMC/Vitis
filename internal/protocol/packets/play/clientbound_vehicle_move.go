package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ClientboundVehicleMove sends the server-authoritative vehicle position to the client.
type ClientboundVehicleMove struct {
	X     float64
	Y     float64
	Z     float64
	Yaw   float32
	Pitch float32
}

func NewClientboundVehicleMove() protocol.Packet { return &ClientboundVehicleMove{} }

func (p *ClientboundVehicleMove) ID() int32 {
	return int32(packetid.ClientboundVehicleMove)
}

func (p *ClientboundVehicleMove) Decode(buf *protocol.Buffer) error {
	var err error
	if p.X, err = buf.ReadFloat64(); err != nil {
		return err
	}
	if p.Y, err = buf.ReadFloat64(); err != nil {
		return err
	}
	if p.Z, err = buf.ReadFloat64(); err != nil {
		return err
	}
	if p.Yaw, err = buf.ReadFloat32(); err != nil {
		return err
	}
	p.Pitch, err = buf.ReadFloat32()
	return err
}

func (p *ClientboundVehicleMove) Encode(buf *protocol.Buffer) error {
	buf.WriteFloat64(p.X)
	buf.WriteFloat64(p.Y)
	buf.WriteFloat64(p.Z)
	buf.WriteFloat32(p.Yaw)
	buf.WriteFloat32(p.Pitch)
	return nil
}
