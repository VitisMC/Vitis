package spawning

import (
	"testing"

	"github.com/vitismc/vitis/internal/entity"
)

// inactiveSpawnWorld rejects all simulation positions and counts block queries.
type inactiveSpawnWorld struct{ reads int }

// IsEntityTicking rejects candidate positions before terrain inspection.
func (w *inactiveSpawnWorld) IsEntityTicking(int32, int32) bool { return false }

// GetBlock records an unexpected inactive terrain query.
func (w *inactiveSpawnWorld) GetBlock(int, int, int) int32 {
	w.reads++
	return 0
}

// TestSpawnerDoesNotSpawnOrDespawnOutsideSimulation preserves paused mobs and avoids unloaded queries.
func TestSpawnerDoesNotSpawnOrDespawnOutsideSimulation(t *testing.T) {
	entities := entity.NewManager()
	mobs := entity.NewMobManager(entities, entities.AllocateID)
	mob := mobs.SpawnMob("zombie", entity.Vec3{X: 1000, Y: 64})
	if mob == nil {
		t.Fatal("missing zombie type")
	}
	world := &inactiveSpawnWorld{}
	requests := NewSpawner(DefaultOverworldSpawns, 1).Tick(mobs, []PlayerPos{{Y: 64}}, world).Requests
	if len(requests) != 0 || world.reads != 0 || mob.Removed() {
		t.Fatal("inactive spawning or despawning changed world state")
	}
}
