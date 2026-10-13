package level

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestPlayerWriterBackpressureAndOrdering checks full queues and stale captures.
func TestPlayerWriterBackpressureAndOrdering(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var lock sync.Mutex
	var positions []float64
	writer := NewPlayerWriter(1, func(data *PlayerData) error {
		if data.PosX == 1 {
			close(started)
			<-release
		}
		lock.Lock()
		positions = append(positions, data.PosX)
		lock.Unlock()
		return nil
	})
	snapshot := func(seq uint64) PlayerWrite {
		return PlayerWrite{Data: &PlayerData{UUID: "test", PosX: float64(seq)}, Sequence: seq}
	}
	if !writer.Offer(snapshot(1)) {
		t.Fatal("initial offer rejected")
	}
	<-started
	if !writer.Offer(snapshot(3)) || writer.Offer(snapshot(2)) {
		t.Fatal("queue capacity was ignored")
	}
	close(release)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := writer.Enqueue(ctx, snapshot(2)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(ctx); err != nil {
		t.Fatal(err)
	}
	lock.Lock()
	defer lock.Unlock()
	if len(positions) != 2 || positions[0] != 1 || positions[1] != 3 {
		t.Fatalf("write order=%v", positions)
	}
}

// TestPlayerWriterRetriesAndFlushes verifies errors do not discard accepted saves.
func TestPlayerWriterRetriesAndFlushes(t *testing.T) {
	var attempts atomic.Int32
	writer := NewPlayerWriter(1, func(*PlayerData) error {
		if attempts.Add(1) < 3 {
			return errors.New("disk failure")
		}
		return nil
	})
	if !writer.Offer(PlayerWrite{Data: &PlayerData{UUID: "test"}, Sequence: 1}) {
		t.Fatal("snapshot rejected")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := writer.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 3 {
		t.Fatal("failed write was not retried")
	}
}

// TestPlayerWriterShutdownFailure reports an outstanding storage failure at timeout.
func TestPlayerWriterShutdownFailure(t *testing.T) {
	diskError := errors.New("read-only disk")
	writer := NewPlayerWriter(1, func(*PlayerData) error { return diskError })
	writer.Offer(PlayerWrite{Data: &PlayerData{UUID: "test"}, Sequence: 1})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := writer.Close(ctx); !errors.Is(err, diskError) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shutdown error=%v", err)
	}
	if writer.Offer(PlayerWrite{Data: &PlayerData{UUID: "late"}, Sequence: 2}) {
		t.Fatal("accepted a snapshot after shutdown")
	}
}

// TestPlayerDataRecipeAndSpawnStateRoundTrip verifies durable identifiers and spawn flags.
func TestPlayerDataRecipeAndSpawnStateRoundTrip(t *testing.T) {
	pd := DefaultPlayerData("00000000-0000-0000-0000-000000000001", 1, 64, 1, 0)
	pd.HasSpawnPoint = true
	pd.SpawnX, pd.SpawnY, pd.SpawnZ = 8, 70, -9
	pd.SpawnDimension, pd.SpawnAngle, pd.SpawnForced = "minecraft:overworld", 75, true
	pd.RecipeBook.Recipes = []string{"minecraft:stick"}
	pd.RecipeBook.Highlights = []string{"minecraft:stick"}
	pd.RecipeBook.Settings[0].Open = true
	pd.RecipeProgress.Criteria = map[string]map[string]bool{"minecraft:recipes/stick": {"has_planks": true}}
	pd.RecipeProgress.Rewarded = map[string]bool{"minecraft:recipes/stick": true}
	dir := t.TempDir()
	if err := pd.Save(dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadPlayerData(dir, pd.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SpawnAngle != 75 || !loaded.SpawnForced || loaded.SpawnZ != -9 || len(loaded.RecipeBook.Recipes) != 1 || !loaded.RecipeProgress.Rewarded["minecraft:recipes/stick"] {
		t.Fatalf("persistent state lost: %+v", loaded)
	}
	clone := loaded.Clone()
	clone.RecipeBook.Recipes[0] = "changed"
	clone.RecipeProgress.Criteria["minecraft:recipes/stick"]["has_planks"] = false
	if loaded.RecipeBook.Recipes[0] != "minecraft:stick" || !loaded.RecipeProgress.Criteria["minecraft:recipes/stick"]["has_planks"] {
		t.Fatal("snapshot aliased runtime data")
	}
}

// TestPlayerWriterRejectsInvalidFinalSnapshots avoids waiting forever on bad input.
func TestPlayerWriterRejectsInvalidFinalSnapshots(t *testing.T) {
	writer := NewPlayerWriter(1, func(*PlayerData) error { return nil })
	if err := writer.Enqueue(context.Background(), PlayerWrite{}); err == nil {
		t.Fatal("empty snapshot accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := writer.Enqueue(ctx, PlayerWrite{Data: &PlayerData{UUID: "test"}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled enqueue=%v", err)
	}
	if err := writer.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// TestPlayerWriterPendingSnapshotPreventsStaleReconnects retains the newest queued state.
func TestPlayerWriterPendingSnapshotPreventsStaleReconnects(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	writer := NewPlayerWriter(2, func(data *PlayerData) error {
		if data.PosX == 1 {
			close(started)
			<-release
		}
		return nil
	})
	writer.Offer(PlayerWrite{Data: &PlayerData{UUID: "test", PosX: 1}, Sequence: 1})
	<-started
	writer.Offer(PlayerWrite{Data: &PlayerData{UUID: "test", PosX: 3}, Sequence: 3})
	writer.Offer(PlayerWrite{Data: &PlayerData{UUID: "test", PosX: 2}, Sequence: 2})
	pending := writer.Pending("test")
	if pending == nil || pending.PosX != 3 {
		t.Fatal("older disk state can be used on reconnect")
	}
	pending.PosX = 99
	if writer.Pending("test").PosX != 3 {
		t.Fatal("pending snapshot shared mutable state")
	}
	close(release)
	if err := writer.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if writer.Pending("test") != nil {
		t.Fatal("completed write retained pending state")
	}
}
