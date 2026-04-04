package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

type WorldParticles struct {
	ParticleID    int32
	LongDistance  bool
	AlwaysShow    bool
	X             float64
	Y             float64
	Z             float64
	OffsetX       float32
	OffsetY       float32
	OffsetZ       float32
	MaxSpeed      float32
	ParticleCount int32
	Data          []byte
}

func NewWorldParticles() protocol.Packet { return &WorldParticles{} }

func (p *WorldParticles) ID() int32 { return int32(packetid.ClientboundWorldParticles) }

func (p *WorldParticles) Decode(_ *protocol.Buffer) error { return nil }

func (p *WorldParticles) Encode(buf *protocol.Buffer) error {
	if p.LongDistance {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	if p.AlwaysShow {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	buf.WriteFloat64(p.X)
	buf.WriteFloat64(p.Y)
	buf.WriteFloat64(p.Z)
	buf.WriteFloat32(p.OffsetX)
	buf.WriteFloat32(p.OffsetY)
	buf.WriteFloat32(p.OffsetZ)
	buf.WriteFloat32(p.MaxSpeed)
	buf.WriteInt32(p.ParticleCount)
	buf.WriteVarInt(p.ParticleID)
	if len(p.Data) > 0 {
		buf.WriteBytes(p.Data)
	}
	return nil
}
