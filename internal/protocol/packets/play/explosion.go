package play

import (
	"github.com/vitismc/vitis/internal/data/generated/particle"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// Explosion notifies a 1.21.4 client of an explosion's effects.
type Explosion struct {
	X, Y, Z                                     float64
	PlayerMotionX, PlayerMotionY, PlayerMotionZ float64
	HasPlayerMotion                             bool
	Large                                       bool
}

// NewExplosion creates a clientbound explosion packet.
func NewExplosion() protocol.Packet { return &Explosion{} }

// ID returns the clientbound explosion packet ID.
func (p *Explosion) ID() int32 { return int32(packetid.ClientboundExplosion) }

// Decode is unused for clientbound packets.
func (p *Explosion) Decode(_ *protocol.Buffer) error { return nil }

// Encode writes the 1.21.4 center, optional knockback, particle and sound.
func (p *Explosion) Encode(buf *protocol.Buffer) error {
	buf.WriteFloat64(p.X)
	buf.WriteFloat64(p.Y)
	buf.WriteFloat64(p.Z)
	buf.WriteBool(p.HasPlayerMotion)
	if p.HasPlayerMotion {
		buf.WriteFloat64(p.PlayerMotionX)
		buf.WriteFloat64(p.PlayerMotionY)
		buf.WriteFloat64(p.PlayerMotionZ)
	}
	particleID := particle.ParticleExplosion
	if p.Large {
		particleID = particle.ParticleExplosionEmitter
	}
	buf.WriteVarInt(particleID)
	// A direct sound holder avoids depending on registry ID ordering.
	buf.WriteVarInt(0)
	if err := buf.WriteString("minecraft:entity.generic.explode"); err != nil {
		return err
	}
	buf.WriteBool(false)
	return nil
}
