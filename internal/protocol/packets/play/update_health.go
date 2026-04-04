package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// UpdateHealth sends health, food, and saturation to the client.
type UpdateHealth struct {
	Health         float32
	Food           int32
	FoodSaturation float32
}

func NewUpdateHealth() protocol.Packet { return &UpdateHealth{} }

func (p *UpdateHealth) ID() int32 { return int32(packetid.ClientboundUpdateHealth) }

func (p *UpdateHealth) Decode(buf *protocol.Buffer) error {
	var err error
	if p.Health, err = buf.ReadFloat32(); err != nil {
		return err
	}
	if p.Food, err = buf.ReadVarInt(); err != nil {
		return err
	}
	p.FoodSaturation, err = buf.ReadFloat32()
	return err
}

func (p *UpdateHealth) Encode(buf *protocol.Buffer) error {
	buf.WriteFloat32(p.Health)
	buf.WriteVarInt(p.Food)
	buf.WriteFloat32(p.FoodSaturation)
	return nil
}
