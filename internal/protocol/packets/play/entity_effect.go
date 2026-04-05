package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// EntityEffect applies a potion/status effect to an entity.
type EntityEffect struct {
	EntityID  int32
	EffectID  int32
	Amplifier int32
	Duration  int32
	Flags     byte
}

func NewEntityEffect() protocol.Packet { return &EntityEffect{} }

func (p *EntityEffect) ID() int32 {
	return int32(packetid.ClientboundEntityEffect)
}

func (p *EntityEffect) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.EffectID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.Amplifier, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.Duration, err = buf.ReadVarInt(); err != nil {
		return err
	}
	p.Flags, err = buf.ReadByte()
	return err
}

func (p *EntityEffect) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteVarInt(p.EffectID)
	buf.WriteVarInt(p.Amplifier)
	buf.WriteVarInt(p.Duration)
	buf.WriteByte(p.Flags)
	return nil
}

// RemoveEntityEffect removes a potion/status effect from an entity.
type RemoveEntityEffect struct {
	EntityID int32
	EffectID int32
}

func NewRemoveEntityEffect() protocol.Packet { return &RemoveEntityEffect{} }

func (p *RemoveEntityEffect) ID() int32 {
	return int32(packetid.ClientboundRemoveEntityEffect)
}

func (p *RemoveEntityEffect) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	p.EffectID, err = buf.ReadVarInt()
	return err
}

func (p *RemoveEntityEffect) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteVarInt(p.EffectID)
	return nil
}
