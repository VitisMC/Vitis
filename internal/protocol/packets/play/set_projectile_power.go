package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetProjectilePower sets the power of a projectile entity.
type SetProjectilePower struct {
	EntityID int32
	PowerX   float64
	PowerY   float64
	PowerZ   float64
}

func NewSetProjectilePower() protocol.Packet { return &SetProjectilePower{} }

func (p *SetProjectilePower) ID() int32 {
	return int32(packetid.ClientboundSetProjectilePower)
}

func (p *SetProjectilePower) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.PowerX, err = buf.ReadFloat64(); err != nil {
		return err
	}
	if p.PowerY, err = buf.ReadFloat64(); err != nil {
		return err
	}
	p.PowerZ, err = buf.ReadFloat64()
	return err
}

func (p *SetProjectilePower) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteFloat64(p.PowerX)
	buf.WriteFloat64(p.PowerY)
	buf.WriteFloat64(p.PowerZ)
	return nil
}
