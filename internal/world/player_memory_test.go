package world

import (
	"errors"
	"testing"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/world/chunk"
)

// memoryPlayer creates an idle player without a network session.
func memoryPlayer(id int32) *entity.Player {
	return entity.NewPlayer(id, protocol.UUID{}, "memory", entity.Vec3{Y: 64}, entity.Vec2{}, nil, 2)
}

// TestLastPlayerDepartureReleasesCaches excludes departures while another player remains.
func TestLastPlayerDepartureReleasesCaches(t *testing.T) {
	w, g := newMemoryTestWorld(t)
	g.available.Store(true)
	first, last := memoryPlayer(1), memoryPlayer(2)
	w.tracker.AddPlayer(first)
	w.tracker.AddPlayer(last)
	w.RemovePlayer(first)
	if memoryReleaseDone(w.pregen) != nil {
		t.Fatal("cache reclamation started with a player still present")
	}
	w.RemovePlayer(last)
	awaitMemoryRelease(t, memoryReleaseDone(w.pregen))
	if g.releases.Load() != 1 {
		t.Fatal("last player departure did not release generation caches")
	}
	done := memoryReleaseDone(w.pregen)
	w.RemovePlayer(last)
	if memoryReleaseDone(w.pregen) != done {
		t.Fatal("repeated removal started another reclamation worker")
	}
}

// TestPlayerRejoinCancelsIdleReclamation protects the new player's generation state.
func TestPlayerRejoinCancelsIdleReclamation(t *testing.T) {
	w, g := newMemoryTestWorld(t)
	player := memoryPlayer(1)
	w.tracker.AddPlayer(player)
	w.RemovePlayer(player)
	awaitMemoryRelease(t, g.attempted)
	idle := memoryReleaseDone(w.pregen)
	w.AddPlayer(memoryPlayer(2))
	awaitMemoryRelease(t, idle)
	g.available.Store(true)
	if g.releases.Load() != 0 || w.pregen.idleChunksPending {
		t.Fatal("rejoined player's state was reclaimed")
	}
}

// TestPlayerDeparturePreservesRunningPregeneration retains Chunky's active caches.
func TestPlayerDeparturePreservesRunningPregeneration(t *testing.T) {
	w, g := newMemoryTestWorld(t)
	g.available.Store(true)
	if err := w.StartPregeneration("square", 0, 0, 16); err != nil {
		t.Fatal(err)
	}
	player := memoryPlayer(1)
	w.tracker.AddPlayer(player)
	w.RemovePlayer(player)
	if memoryReleaseDone(w.pregen) != nil {
		t.Fatal("player departure reclaimed running Chunky caches")
	}
	if err := w.CancelPregeneration(); err != nil {
		t.Fatal(err)
	}
	awaitMemoryRelease(t, memoryReleaseDone(w.pregen))
	if g.releases.Load() != 1 {
		t.Fatal("cancelled pregeneration retained its caches")
	}
}

// TestIdleMemoryWaitsForChunkUnloads defers final scavenging until runtime chunks leave.
func TestIdleMemoryWaitsForChunkUnloads(t *testing.T) {
	w, g := newMemoryTestWorld(t)
	g.available.Store(true)
	storage := chunk.NewStorage(1)
	var saveErr error
	manager := chunk.NewManager(chunk.ManagerConfig{Storage: storage, Generator: g, OnUnload: func(*chunk.Chunk) error { return saveErr }})
	defer func() {
		if err := manager.Close(nil); err != nil {
			t.Error(err)
		}
	}()
	// The fixture's original manager remains owned by World.Close.
	original := w.chunks
	w.chunks = manager
	defer func() { w.chunks = original }()
	c := chunk.New(17, -5)
	c.SetState(chunk.StateLoaded)
	storage.Set(c)
	player := memoryPlayer(1)
	w.tracker.AddPlayer(player)
	w.RemovePlayer(player)
	awaitMemoryRelease(t, memoryReleaseDone(w.pregen))
	first := memoryReleaseDone(w.pregen)
	w.pregen.tick()
	if memoryReleaseDone(w.pregen) != first || !w.pregen.idleChunksPending {
		t.Fatal("runtime chunks triggered premature final reclamation")
	}
	c.BeginSave()
	manager.MarkForUnload(c.X(), c.Z())
	if manager.ProcessUnloads(1) != 0 {
		t.Fatal("chunk with a pending save was unloaded")
	}
	c.FinishSave()
	c.MarkDirty()
	saveErr = errors.New("disk unavailable")
	manager.MarkForUnload(c.X(), c.Z())
	if manager.ProcessUnloads(1) != 0 {
		t.Fatal("chunk with a failed save was unloaded")
	}
	w.pregen.tick()
	if memoryReleaseDone(w.pregen) != first {
		t.Fatal("pending or failed saves triggered final reclamation")
	}
	saveErr = nil
	if !manager.MarkForUnload(c.X(), c.Z()) || manager.ProcessUnloads(1) != 1 {
		t.Fatal("unused clean chunk did not unload")
	}
	w.pregen.tick()
	awaitMemoryRelease(t, memoryReleaseDone(w.pregen))
	if g.releases.Load() != 2 || w.pregen.idleChunksPending {
		t.Fatal("unloaded chunk memory was not reclaimed")
	}
	w.pregen.tick()
	if g.releases.Load() != 2 {
		t.Fatal("idle world requested repeated garbage collections")
	}
}
