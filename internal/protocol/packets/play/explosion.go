package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// Explosion notifies the client that an explosion has occurred.
type Explosion struct {
	X                    float64
	Y                    float64
	Z                    float64
	Strength             float32
	RecordCount          int32
	Records              []byte
	PlayerMotionX        float64
	PlayerMotionY        float64
	PlayerMotionZ        float64
	HasPlayerMotion      bool
	BlockInteraction     int32
	SmallExplosionParticle int32
	SmallParticleData    []byte
	LargeExplosionParticle int32
	LargeParticleData    []byte
	SoundName            string
	HasFixedRange        bool
	FixedRange           float32
}

func NewExplosion() protocol.Packet { return &Explosion{} }

func (p *Explosion) ID() int32 {
	return int32(packetid.ClientboundExplosion)
}

func (p *Explosion) Decode(_ *protocol.Buffer) error { return nil }

func (p *Explosion) Encode(buf *protocol.Buffer) error {
	buf.WriteFloat64(p.X)
	buf.WriteFloat64(p.Y)
	buf.WriteFloat64(p.Z)
	buf.WriteFloat32(p.Strength)
	buf.WriteVarInt(p.RecordCount)
	if len(p.Records) > 0 {
		buf.WriteBytes(p.Records)
	}
	if p.HasPlayerMotion {
		buf.WriteByte(1)
		buf.WriteFloat64(p.PlayerMotionX)
		buf.WriteFloat64(p.PlayerMotionY)
		buf.WriteFloat64(p.PlayerMotionZ)
	} else {
		buf.WriteByte(0)
	}
	buf.WriteVarInt(p.BlockInteraction)
	buf.WriteVarInt(p.SmallExplosionParticle)
	if len(p.SmallParticleData) > 0 {
		buf.WriteBytes(p.SmallParticleData)
	}
	buf.WriteVarInt(p.LargeExplosionParticle)
	if len(p.LargeParticleData) > 0 {
		buf.WriteBytes(p.LargeParticleData)
	}
	buf.WriteVarInt(0)
	buf.WriteString(p.SoundName)
	if p.HasFixedRange {
		buf.WriteByte(1)
		buf.WriteFloat32(p.FixedRange)
	} else {
		buf.WriteByte(0)
	}
	return nil
}
