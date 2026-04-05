package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SpawnEntityExperienceOrb spawns an experience orb entity.
type SpawnEntityExperienceOrb struct {
	EntityID int32
	X        float64
	Y        float64
	Z        float64
	Count    int16
}

func NewSpawnEntityExperienceOrb() protocol.Packet { return &SpawnEntityExperienceOrb{} }

func (p *SpawnEntityExperienceOrb) ID() int32 {
	return int32(packetid.ClientboundSpawnEntityExperienceOrb)
}

func (p *SpawnEntityExperienceOrb) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.X, err = buf.ReadFloat64(); err != nil {
		return err
	}
	if p.Y, err = buf.ReadFloat64(); err != nil {
		return err
	}
	if p.Z, err = buf.ReadFloat64(); err != nil {
		return err
	}
	p.Count, err = buf.ReadInt16()
	return err
}

func (p *SpawnEntityExperienceOrb) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteFloat64(p.X)
	buf.WriteFloat64(p.Y)
	buf.WriteFloat64(p.Z)
	buf.WriteInt16(p.Count)
	return nil
}
