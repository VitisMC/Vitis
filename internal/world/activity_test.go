package world

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/world/chunk"
	worldtick "github.com/vitismc/vitis/internal/world/tick"
)

// TestSimulationDistanceValidation rejects unbounded simulation before allocating workers.
func TestSimulationDistanceValidation(t *testing.T) {
	for _, distance := range []int32{-1, 97} {
		if w, err := New(Config{Name: "invalid-simulation", SimulationDistance: distance}); err == nil || w != nil {
			t.Fatalf("accepted simulation distance %d", distance)
		}
	}
}

// TestPlayersOwnIndependentSimulationAreas retains the second player's tickets after the first leaves.
func TestPlayersOwnIndependentSimulationAreas(t *testing.T) {
	storage := chunk.NewStorage(50)
	loadTestNeighborhood(storage, 0, 0, 2)
	loadTestNeighborhood(storage, 8, 0, 2)
	w, err := New(Config{Name: "two-player-simulation", SimulationDistance: 1, ChunkManager: chunk.NewManager(chunk.ManagerConfig{Storage: storage})})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close(nil) })
	first := entity.NewPlayer(1, protocol.UUID{1}, "first", entity.Vec3{X: 8, Y: 64, Z: 8}, entity.Vec2{}, nil, 10)
	second := entity.NewPlayer(2, protocol.UUID{2}, "second", entity.Vec3{X: 136, Y: 64, Z: 8}, entity.Vec2{}, nil, 10)
	w.tracker.AddPlayer(first)
	w.tracker.AddPlayer(second)
	w.updateActivity()
	if !w.IsEntityTicking(0, 0) || !w.IsEntityTicking(8, 0) {
		t.Fatal("independent player centers did not activate")
	}
	w.tracker.RemovePlayer(first.ID())
	w.updateActivity()
	if w.IsEntityTicking(0, 0) || !w.IsEntityTicking(8, 0) || w.ChunkActivity(0, 0) != chunk.ActivityLoaded {
		t.Fatal("leaving player removed another player's simulation or visibility")
	}
}

// TestTickingWaitsForFullDependencies verifies the one- and two-ring readiness gates.
func TestTickingWaitsForFullDependencies(t *testing.T) {
	storage := chunk.NewStorage(25)
	storage.Set(chunk.New(0, 0))
	w, err := New(Config{Name: "dependencies", ChunkManager: chunk.NewManager(chunk.ManagerConfig{Storage: storage})})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close(nil) })
	if err := w.SetChunkTicket(chunk.Ticket{Kind: chunk.TicketForced}); err != nil {
		t.Fatal(err)
	}
	w.updateActivity()
	if w.ChunkActivity(0, 0) != chunk.ActivityLoaded {
		t.Fatal("chunk ticked before its dependencies loaded")
	}
	loadTestNeighborhood(storage, 0, 0, 1)
	if !w.IsBlockTicking(0, 0) || w.IsEntityTicking(0, 0) {
		t.Fatal("block and entity readiness are not independent")
	}
	loadTestNeighborhood(storage, 0, 0, 2)
	if !w.IsEntityTicking(0, 0) {
		t.Fatal("complete dependencies did not activate entity ticking")
	}
	storage.Delete(1, 0)
	if w.IsBlockTicking(0, 0) {
		t.Fatal("lost dependency did not suspend ticking")
	}
}

// activateTestChunk supplies an explicit forced ticket for isolated mechanic tests.
func activateTestChunk(t *testing.T, w *World, storage *chunk.Storage, x, z int32) {
	t.Helper()
	loadTestNeighborhood(storage, x, z, 2)
	if err := w.SetChunkTicket(chunk.Ticket{Kind: chunk.TicketForced, Owner: uint64(chunk.ChunkKey(x, z)), X: x, Z: z}); err != nil {
		t.Fatal(err)
	}
	w.updateActivity()
}

// loadTestNeighborhood publishes FULL dependencies for isolated simulation tests.
func loadTestNeighborhood(storage *chunk.Storage, x, z, radius int32) {
	for dx := -radius; dx <= radius; dx++ {
		for dz := -radius; dz <= radius; dz++ {
			if c, ok := storage.Get(x+dx, z+dz); !ok || c.State() != chunk.StateLoaded {
				storage.Set(chunk.New(x+dx, z+dz))
			}
		}
	}
}

// TestSimulationDistanceSeparatesLoadedChunksFromTicking checks moving players and inactive queues.
func TestSimulationDistanceSeparatesLoadedChunksFromTicking(t *testing.T) {
	storage := chunk.NewStorage(5)
	for _, x := range []int32{0, 1, 2, 3, 8} {
		storage.Set(chunk.New(x, 0))
	}
	w, err := New(Config{Name: "simulation", SimulationDistance: 1, ChunkManager: chunk.NewManager(chunk.ManagerConfig{Storage: storage})})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close(nil)
	loadTestNeighborhood(storage, 0, 0, 4)
	loadTestNeighborhood(storage, 8, 0, 3)
	p := entity.NewPlayer(1, protocol.UUID{1}, "test", entity.Vec3{Y: 64}, entity.Vec2{}, nil, 8)
	w.tracker.AddPlayer(p)
	w.updateActivity()
	if !w.IsEntityTicking(1, 0) || w.IsEntityTicking(2, 0) || !w.IsBlockTicking(2, 0) || w.IsBlockTicking(3, 0) || w.ChunkActivity(8, 0) != chunk.ActivityLoaded {
		t.Fatal("simulation and visibility distances overlap incorrectly")
	}
	state := block.DefaultStateID("minecraft:stone")
	w.tickScheduler.Schedule(128, 64, 0, w.CurrentTick(), 0, worldtick.PriorityNormal, worldtick.TickTypeBlock, state)
	w.processScheduledTicks()
	if w.tickScheduler.Pending() != 1 {
		t.Fatal("inactive scheduled tick was consumed")
	}
	p.SetPosition(entity.Vec3{X: 128, Y: 64})
	w.updateActivity()
	w.processScheduledTicks()
	if w.tickScheduler.Pending() != 0 || w.IsEntityTicking(0, 0) {
		t.Fatal("moving player did not change simulation ownership")
	}
}

// TestInactiveItemDoesNotAgeOrMerge checks entity manager gating independently of rendering.
func TestInactiveItemDoesNotAgeOrMerge(t *testing.T) {
	storage := chunk.NewStorage(1)
	storage.Set(chunk.New(0, 0))
	w, err := New(Config{Name: "inactive", ChunkManager: chunk.NewManager(chunk.ManagerConfig{Storage: storage})})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close(nil)
	item := entity.NewItemEntityWithVelocity(1, protocol.UUID{1}, entity.Vec3{X: 8, Y: 64, Z: 8}, inventory.NewSlot(1, 1), entity.Vec3{}, 10)
	w.items.Add(item)
	w.tickItems()
	if item.Position().Y != 64 {
		t.Fatal("inactive entity moved")
	}
	activateTestChunk(t, w, storage, 0, 0)
	w.tickItems()
	if item.Position().Y >= 64 {
		t.Fatal("activated entity did not tick")
	}
}
