package projectile

import (
	"testing"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestArrowDamageScalesWithSpeed guards the damage of real ranged attacks.
func TestArrowDamageScalesWithSpeed(t *testing.T) {
	for _, test := range []struct{ speed, damage float64 }{{0, 0}, {.25, 1}, {1.6, 4}, {3, 6}} {
		arrow := NewArrow(1, protocol.UUID{}, entity.Vec3{}, entity.Vec3{X: test.speed}, 0, protocol.UUID{})
		if actual := arrow.ComputeHitDamage(); actual != test.damage {
			t.Fatalf("speed %v: damage %v, want %v", test.speed, actual, test.damage)
		}
	}
}

// unavailableProjectileWorld prevents projectiles from crossing unloaded terrain.
type unavailableProjectileWorld struct{}

// GetBlockStateAt returns unavailable states past the resident boundary.
func (unavailableProjectileWorld) GetBlockStateAt(x, y, z int) int32 {
	if x >= 1 {
		return -1
	}
	return 0
}

// TestProjectileStopsAtUnavailableTerrain avoids damaging targets through unknown chunks.
func TestProjectileStopsAtUnavailableTerrain(t *testing.T) {
	arrow := NewArrow(1, protocol.UUID{}, entity.Vec3{X: .5, Y: 64, Z: .5}, entity.Vec3{X: 3}, 0, protocol.UUID{})
	arrow.Tick(unavailableProjectileWorld{}, nil)
	hit := arrow.LastHitResult()
	if hit == nil || !hit.HitBlock || !arrow.InGround() || arrow.Position().X > 1.01 {
		t.Fatal("projectile crossed an unavailable chunk")
	}
}
