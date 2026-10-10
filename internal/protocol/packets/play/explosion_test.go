package play

import (
	"testing"

	"github.com/vitismc/vitis/internal/data/generated/particle"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestExplosionWireFormat checks every field in the 1.21.4 client decoder order.
func TestExplosionWireFormat(t *testing.T) {
	for _, tc := range []struct {
		name       string
		packet     Explosion
		particleID int32
	}{
		{"small", Explosion{X: 1.5, Y: 64, Z: -2}, particle.ParticleExplosion},
		{"large with knockback", Explosion{X: 1.5, Y: 64, Z: -2, HasPlayerMotion: true, PlayerMotionX: 0.25, PlayerMotionY: 0.5, PlayerMotionZ: -0.25, Large: true}, particle.ParticleExplosionEmitter},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buf := protocol.NewBuffer(96)
			if err := tc.packet.Encode(buf); err != nil {
				t.Fatal(err)
			}
			for _, want := range []float64{tc.packet.X, tc.packet.Y, tc.packet.Z} {
				got, err := buf.ReadFloat64()
				if err != nil || got != want {
					t.Fatalf("center: got %v, want %v: %v", got, want, err)
				}
			}
			hasMotion, err := buf.ReadBool()
			if err != nil || hasMotion != tc.packet.HasPlayerMotion {
				t.Fatalf("knockback presence: got %v: %v", hasMotion, err)
			}
			if hasMotion {
				for _, want := range []float64{tc.packet.PlayerMotionX, tc.packet.PlayerMotionY, tc.packet.PlayerMotionZ} {
					got, err := buf.ReadFloat64()
					if err != nil || got != want {
						t.Fatalf("knockback: got %v, want %v: %v", got, want, err)
					}
				}
			}
			particleID, err := buf.ReadVarInt()
			if err != nil || particleID != tc.particleID {
				t.Fatalf("particle: got %d, want %d: %v", particleID, tc.particleID, err)
			}
			holderID, err := buf.ReadVarInt()
			if err != nil || holderID != 0 {
				t.Fatalf("sound holder: got %d: %v", holderID, err)
			}
			sound, err := buf.ReadString()
			if err != nil || sound != "minecraft:entity.generic.explode" {
				t.Fatalf("sound: got %q: %v", sound, err)
			}
			fixedRange, err := buf.ReadBool()
			if err != nil || fixedRange {
				t.Fatalf("fixed range: got %v: %v", fixedRange, err)
			}
			if buf.Remaining() != 0 {
				t.Fatalf("unexpected trailing bytes: %d", buf.Remaining())
			}
		})
	}
}
