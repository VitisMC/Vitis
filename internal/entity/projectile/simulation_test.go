package projectile

import (
	"testing"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol"
)

// projectileSimulationWorld supplies empty block data and a mutable activity policy.
type projectileSimulationWorld struct{ active bool }

// IsEntityTicking returns the authoritative test gate.
func (w *projectileSimulationWorld) IsEntityTicking(int32, int32) bool { return w.active }

// GetBlockStateAt supplies air for projectile movement.
func (w *projectileSimulationWorld) GetBlockStateAt(int, int, int) int32 { return 0 }

// TestProjectileActivityPreservesLifeAndProcessesRemoval checks the connected manager gate.
func TestProjectileActivityPreservesLifeAndProcessesRemoval(t *testing.T) {
	world := &projectileSimulationWorld{}
	m := NewManager()
	pos := entity.Vec3{X: 8, Y: 64, Z: 8}
	arrow := NewArrow(1, protocol.UUID{1}, pos, entity.Vec3{X: 1}, 0, protocol.UUID{})
	m.AddArrow(arrow)
	m.TickAll(world, nil)
	if arrow.Life() != 0 || arrow.Position() != pos {
		t.Fatal("inactive projectile advanced")
	}
	world.active = true
	m.TickAll(world, nil)
	if arrow.Life() != 1 || arrow.Position().X <= pos.X {
		t.Fatal("reactivated projectile remained frozen")
	}
	world.active = false
	arrow.Remove()
	result := m.TickAll(world, nil)
	if len(result.RemovedIDs) != 1 || result.RemovedIDs[0] != arrow.ID() {
		t.Fatal("inactive removed projectile was retained")
	}
}
