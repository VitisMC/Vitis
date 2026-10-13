package main

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/world"
	"github.com/vitismc/vitis/internal/world/chunk"
)

// loginSpawnWorld provides loaded collision dependencies without active player tickets.
func loginSpawnWorld(t *testing.T, queueCapacity int) *world.World {
	t.Helper()
	storage := chunk.NewStorage(25)
	for x := int32(-2); x <= 2; x++ {
		for z := int32(-2); z <= 2; z++ {
			storage.Set(chunk.New(x, z))
		}
	}
	w, err := world.New(world.Config{Name: "login-lease", Dimension: "minecraft:the_nether", SchedulerQueueCapacity: queueCapacity, ChunkManager: chunk.NewManager(chunk.ManagerConfig{Storage: storage})})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := w.Close(nil); err != nil {
			t.Error(err)
		}
	})
	return w
}

// waitLoginSpawnTask waits for ingress without accessing game state concurrently.
func waitLoginSpawnTask(t *testing.T, w *world.World) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for w.Scheduler().Pending() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("login preparation was not queued")
		}
		time.Sleep(time.Millisecond)
	}
}

// TestLoginSpawnLeaseRetainsUntilAdmission verifies game-thread acquisition and ordered release.
func TestLoginSpawnLeaseRetainsUntilAdmission(t *testing.T) {
	w := loginSpawnWorld(t, 8)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	position := entity.Vec3{X: 8.5, Y: 64, Z: 8.5}
	result := make(chan error, 1)
	go func() { result <- retainLoginSpawn(ctx, w, 1, position) }()
	waitLoginSpawnTask(t, w)
	if w.ChunkActivity(0, 0) != chunk.ActivityUnloaded {
		t.Fatal("login preparation mutated tickets outside the tick")
	}
	w.Tick()
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	for x := int32(-1); x <= 1; x++ {
		for z := int32(-1); z <= 1; z++ {
			if w.ChunkActivity(x, z) != chunk.ActivityLoaded {
				t.Fatal("login dependencies were not retained independently of simulation")
			}
		}
	}
	if w.ChunkActivity(2, 0) != chunk.ActivityUnloaded {
		t.Fatal("login retention exceeded its collision dependencies")
	}
	player := entity.NewPlayer(1, protocol.UUID{1}, "login", position, entity.Vec2{}, nil, 2)
	if !w.Submit(func() { w.AddPlayer(player) }) {
		t.Fatal("admission queue rejected player")
	}
	releaseLoginSpawn(w, 1)
	w.Tick()
	if w.GetEntity(1) != player.Entity || !w.IsEntityTicking(0, 0) {
		t.Fatal("temporary retention release removed player simulation ownership")
	}
	w.RemovePlayer(player)
	w.Tick()
	if w.ChunkActivity(0, 0) != chunk.ActivityUnloaded {
		t.Fatal("login ticket survived admission and disconnect")
	}
}

// TestLoginSpawnCancellationDoesNotAcquireTickets cancels an accepted task before execution.
func TestLoginSpawnCancellationDoesNotAcquireTickets(t *testing.T) {
	w := loginSpawnWorld(t, 8)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- retainLoginSpawn(ctx, w, 1, entity.Vec3{}) }()
	waitLoginSpawnTask(t, w)
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled preparation error = %v", err)
	}
	releaseLoginSpawn(w, 1)
	w.Tick()
	if w.ChunkActivity(0, 0) != chunk.ActivityUnloaded {
		t.Fatal("cancelled login activated or retained chunks")
	}
}

// TestLoginSpawnRejectsMalformedCoordinatesAndFullIngress verifies bounded admission failure.
func TestLoginSpawnRejectsMalformedCoordinatesAndFullIngress(t *testing.T) {
	w := loginSpawnWorld(t, 1)
	for _, position := range []entity.Vec3{{X: math.NaN()}, {Z: math.Inf(1)}, {X: 30000000}, {Z: -30000001}} {
		if err := retainLoginSpawn(context.Background(), w, 1, position); err == nil {
			t.Fatal("invalid coordinates accepted")
		}
	}
	if w.Scheduler().Pending() != 0 {
		t.Fatal("malformed login entered the game queue")
	}
	if !w.Submit(func() {}) {
		t.Fatal("failed to saturate ingress")
	}
	if err := retainLoginSpawn(context.Background(), w, 1, entity.Vec3{}); err == nil {
		t.Fatal("saturated ingress accepted a login preparation")
	}
	w.Tick()
	if w.ChunkActivity(0, 0) != chunk.ActivityUnloaded {
		t.Fatal("rejected preparation retained chunks")
	}
}
