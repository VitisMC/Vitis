package projectile

import (
	"testing"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol"
)

func TestThrownProjectileRemainsAvailableUntilFinalized(t *testing.T) {
	manager := NewManager()
	thrown := NewThrownEntity(5, protocol.UUID{}, ThrownEnderPearl, entity.Vec3{}, entity.Vec3{}, 1, protocol.UUID{})
	thrown.Entity.Remove()
	manager.AddThrown(thrown)

	result := manager.TickAll(nil, nil)
	if len(result.RemovedIDs) != 1 || result.RemovedIDs[0] != thrown.ID() {
		t.Fatalf("removed IDs = %v", result.RemovedIDs)
	}
	if got := manager.GetThrown(thrown.ID()); got != thrown {
		t.Fatal("thrown projectile was removed before effect processing")
	}
	manager.FinalizeRemovals(result.RemovedIDs)
	if got := manager.GetThrown(thrown.ID()); got != nil {
		t.Fatal("thrown projectile remained after finalization")
	}
}
