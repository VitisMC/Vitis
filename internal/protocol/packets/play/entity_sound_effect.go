package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// EntitySoundEffect plays a sound attached to an entity.
type EntitySoundEffect struct {
	SoundID       int32
	SoundCategory int32
	EntityID      int32
	Volume        float32
	Pitch         float32
	Seed          int64
}

func NewEntitySoundEffect() protocol.Packet { return &EntitySoundEffect{} }

func (p *EntitySoundEffect) ID() int32 {
	return int32(packetid.ClientboundEntitySoundEffect)
}

func (p *EntitySoundEffect) Decode(buf *protocol.Buffer) error {
	var err error
	if p.SoundID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.SoundCategory, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.Volume, err = buf.ReadFloat32(); err != nil {
		return err
	}
	if p.Pitch, err = buf.ReadFloat32(); err != nil {
		return err
	}
	p.Seed, err = buf.ReadInt64()
	return err
}

func (p *EntitySoundEffect) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.SoundID)
	buf.WriteVarInt(p.SoundCategory)
	buf.WriteVarInt(p.EntityID)
	buf.WriteFloat32(p.Volume)
	buf.WriteFloat32(p.Pitch)
	buf.WriteInt64(p.Seed)
	return nil
}
