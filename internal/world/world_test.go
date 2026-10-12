package world

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/vitismc/vitis/internal/block"
	blockentitystate "github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/level"
	"github.com/vitismc/vitis/internal/world/persistence"
)

type countingGenerator struct {
	calls int
}

func (g *countingGenerator) Generate(x, z int32) (*chunk.Chunk, error) {
	g.calls++
	return chunk.New(x, z), nil
}

func TestCustomGeneratorLoadsPersistedChunkFirst(t *testing.T) {
	store := persistence.NewChunkStore(t.TempDir())
	persisted := chunk.New(3, -2)
	persisted.Data().SetBlock(1, 64, 1, 1)
	encoded, err := chunk.EncodeChunkNBT(persisted)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WriteChunkNBT(3, -2, encoded); err != nil {
		t.Fatal(err)
	}

	generator := &countingGenerator{}
	instance, err := New(Config{Name: "persistent_custom", Generator: generator, ChunkStore: store, ChunkLoadWorkers: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close(context.Background())
	if !instance.chunks.RequestLoad(3, -2) {
		t.Fatal("persisted chunk load request was rejected")
	}
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		instance.chunks.PumpLoadRequests()
		instance.chunks.ApplyLoadCompletions(1)
		if loaded, ok := instance.chunks.GetChunk(3, -2); ok {
			if got := loaded.Data().GetBlock(1, 64, 1); got != 1 {
				t.Fatalf("persisted block = %d, want 1", got)
			}
			if generator.calls != 0 {
				t.Fatalf("fallback generator called %d times", generator.calls)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("persisted chunk did not load")
}

// TestWorldClosePersistsLevelTime checks metadata survives shutdown.
func TestWorldClosePersistsLevelTime(t *testing.T) {
	worldDir := filepath.Join(t.TempDir(), "world")
	metadata := level.New("world", 0, 64, 0)
	metadata.Seed = 123
	if err := metadata.Save(worldDir); err != nil {
		t.Fatal(err)
	}
	instance, err := New(Config{
		Name: "world", WorldDir: worldDir, Seed: 123,
		Difficulty: 2, InitialWorldAge: 100, InitialTimeOfDay: 23000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if instance.Seed() != 123 || instance.WorldAge() != 100 || instance.TimeOfDay() != 23000 {
		t.Fatalf("incorrect initial metadata: seed=%d age=%d day=%d", instance.Seed(), instance.WorldAge(), instance.TimeOfDay())
	}
	instance.Tick()
	if err := instance.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	loaded, err := level.Open(worldDir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Time != instance.WorldAge() || loaded.DayTime != instance.TimeOfDay() || loaded.Difficulty != 2 {
		t.Fatalf("persisted metadata differs: age=%d day=%d difficulty=%d", loaded.Time, loaded.DayTime, loaded.Difficulty)
	}
}

func TestWorldTimeSnapshotIsConcurrentSafe(t *testing.T) {
	instance, err := New(Config{Name: "time_snapshot"})
	if err != nil {
		t.Fatalf("create world: %v", err)
	}
	defer instance.Close(nil)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := int64(0); i < 10_000; i++ {
			instance.SetTimeOfDay(i)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 10_000; i++ {
			if got := instance.TimeOfDay(); got < 0 || got >= 24_000 {
				t.Errorf("invalid time of day: %d", got)
				return
			}
			_ = instance.WorldAge()
		}
	}()
	wg.Wait()
}

// TestPlayerDeathHandlerRunsOncePerDeath prevents duplicated death drops.
func TestPlayerDeathHandlerRunsOncePerDeath(t *testing.T) {
	w, err := New(Config{Name: "death_lifecycle", Difficulty: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close(nil)
	player := entity.NewPlayer(10, protocol.UUID{1}, "test", entity.Vec3{Y: 64}, entity.Vec2{}, nil, 2)
	player.SetClientLoaded(true)
	player.Living().SetHealth(0)
	w.AddPlayer(player)
	deaths := 0
	w.SetPlayerDeathHandler(func(*entity.Player) { deaths++ })
	w.Tick()
	w.Tick()
	if deaths != 1 {
		t.Fatalf("death callback ran %d times, want one", deaths)
	}
	player.Living().Respawn()
	player.Living().Damage(20, "test")
	w.Tick()
	if deaths != 2 {
		t.Fatalf("death callback did not run after respawn: %d", deaths)
	}
}

func TestWorldDifficultyState(t *testing.T) {
	w, err := New(Config{Name: "difficulty", Difficulty: 2})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := w.Difficulty(); got != 2 {
		t.Fatalf("difficulty = %d, want 2", got)
	}
	w.SetDifficulty(3)
	if got := w.Difficulty(); got != 3 {
		t.Fatalf("difficulty = %d, want 3", got)
	}
	w.SetDifficulty(9)
	if got := w.Difficulty(); got != 3 {
		t.Fatalf("invalid update changed difficulty to %d", got)
	}
}

func TestItemPickupKeepsUnacceptedRemainder(t *testing.T) {
	storage := chunk.NewStorage(1)
	storage.Set(chunk.New(0, 0))
	w, err := New(Config{Name: "item_pickup", ChunkManager: chunk.NewManager(chunk.ManagerConfig{Storage: storage})})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close(nil)
	player := entity.NewPlayer(10, protocol.UUID{1}, "test", entity.Vec3{Y: 64}, entity.Vec2{}, nil, 2)
	w.tracker.AddPlayer(player)
	loadTestNeighborhood(storage, 0, 0, 2)
	w.updateActivity()
	dropped := entity.NewItemEntityWithVelocity(11, protocol.UUID{2}, entity.Vec3{Y: 64}, inventory.NewSlot(1, 5), entity.Vec3{}, 0)
	w.items.Add(dropped)
	w.SetItemPickupHandler(func(_ *entity.Player, stack inventory.Slot) int32 {
		if stack.ItemCount != 5 {
			t.Fatalf("pickup stack count = %d, want 5", stack.ItemCount)
		}
		return 3
	})
	w.tickItems()
	if got := w.items.Get(11); got == nil || got.Stack().ItemCount != 2 {
		t.Fatalf("partial pickup remainder = %+v, want 2", got)
	}
}

func TestDaylightCycleRuleFreezesClock(t *testing.T) {
	w, err := New(Config{Name: "daylight_rule"})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close(nil)
	w.SetTimeOfDay(12000)
	w.SetDaylightCycle(false)
	w.Tick()
	if got := w.TimeOfDay(); got != 12000 {
		t.Fatalf("frozen time = %d, want 12000", got)
	}
	w.SetDaylightCycle(true)
	w.Tick()
	if got := w.TimeOfDay(); got != 12001 {
		t.Fatalf("resumed time = %d, want 12001", got)
	}
}

func TestFireExtinguishesWithoutSupportAndFreezesWithRule(t *testing.T) {
	storage := chunk.NewStorage(1)
	section := chunk.New(0, 0)
	storage.Set(section)
	manager := chunk.NewManager(chunk.ManagerConfig{Storage: storage})
	w, err := New(Config{Name: "fire_rule", ChunkManager: manager})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close(nil)
	fire := block.DefaultStateID("minecraft:fire")
	if fire < 0 {
		t.Fatal("fire state unavailable")
	}
	section.SetBlock(1, 64, 1, fire)
	w.tickFire(1, 64, 1, fire)
	if got := section.GetBlock(1, 64, 1); got != 0 {
		t.Fatalf("unsupported fire remained: %d", got)
	}
	section.SetBlock(1, 64, 1, fire)
	w.SetFireTick(false)
	w.tickFire(1, 64, 1, fire)
	if got := section.GetBlock(1, 64, 1); got != fire {
		t.Fatalf("disabled fire tick changed block: %d", got)
	}
}

func TestIgniteTNTReplacesBlockWithPrimedEntity(t *testing.T) {
	storage := chunk.NewStorage(1)
	section := chunk.New(0, 0)
	storage.Set(section)
	w, err := New(Config{Name: "tnt_ignition", ChunkManager: chunk.NewManager(chunk.ManagerConfig{Storage: storage})})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close(nil)
	activateTestChunk(t, w, storage, 0, 0)
	section.SetBlock(2, 64, 2, block.DefaultStateID("minecraft:tnt"))
	if !w.IgniteTNT(2, 64, 2, 7) {
		t.Fatal("TNT ignition rejected")
	}
	if !block.IsAir(section.GetBlock(2, 64, 2)) || len(w.tnts.All()) != 1 {
		t.Fatalf("TNT ignition state: block=%d entities=%d", section.GetBlock(2, 64, 2), len(w.tnts.All()))
	}
	if w.IgniteTNT(2, 64, 2, 7) {
		t.Fatal("already primed TNT ignited twice")
	}
	section.SetBlock(2, 63, 2, block.DefaultStateID("minecraft:stone"))
	section.SetBlock(3, 64, 2, block.DefaultStateID("minecraft:stone"))
	for i := 0; i < 80; i++ {
		w.tickTNT()
	}
	if len(w.tnts.All()) != 0 || !block.IsAir(section.GetBlock(3, 64, 2)) {
		t.Fatalf("fused TNT did not explode: remaining=%d neighbor=%d", len(w.tnts.All()), section.GetBlock(3, 64, 2))
	}
}

func TestWorldTickRunsFurnaceAndLightsBlock(t *testing.T) {
	storage := chunk.NewStorage(1)
	owner := chunk.New(0, 0)
	owner.SetBlock(1, 64, 2, block.DefaultStateID("minecraft:furnace"))
	furnace := blockentitystate.NewFurnaceBlockEntity(1, 64, 2)
	furnace.Container().Set(0, inventory.NewSlot(item.IDByName("minecraft:sand"), 1))
	furnace.Container().Set(1, inventory.NewSlot(item.IDByName("minecraft:coal"), 1))
	owner.Data().AddBlockEntity(furnace)
	storage.Set(owner)
	w, err := New(Config{Name: "furnace_tick", ChunkManager: chunk.NewManager(chunk.ManagerConfig{Storage: storage})})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close(nil)
	activateTestChunk(t, w, storage, 0, 0)
	updates := 0
	w.SetFurnaceUpdateHandler(func(_, _, _ int32, _ []inventory.Slot, _ [4]int16) { updates++ })
	w.tickBlockEntities()
	if !furnace.IsBurning() || updates != 1 || !owner.Dirty() {
		t.Fatalf("furnace did not tick: burning=%v updates=%d dirty=%v", furnace.IsBurning(), updates, owner.Dirty())
	}
	if props := block.PropertiesFromState(owner.GetBlock(1, 64, 2)); props["lit"] != "true" {
		t.Fatalf("furnace block not lit: %+v", props)
	}
}

func TestWorldTickAppliesAsyncChunkCompletions(t *testing.T) {
	instance, err := New(Config{Name: "test_world"})
	if err != nil {
		t.Fatalf("create world failed: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = instance.ChunkManager().Close(ctx)
	}()

	instance.LoadChunk(12, 34)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		instance.Tick()
		loaded, ok := instance.GetChunk(12, 34)
		if !ok || loaded == nil {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		if loaded.X() != 12 || loaded.Z() != 34 {
			t.Fatalf("unexpected loaded chunk coordinates x=%d z=%d", loaded.X(), loaded.Z())
		}
		return
	}

	t.Fatalf("timed out waiting for chunk load completion")
}

func TestWorldUnloadChunkRemovesData(t *testing.T) {
	instance, err := New(Config{Name: "unload_world"})
	if err != nil {
		t.Fatalf("create world failed: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = instance.ChunkManager().Close(ctx)
	}()

	instance.LoadChunk(2, 2)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		instance.Tick()
		if _, ok := instance.GetChunk(2, 2); ok {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if _, ok := instance.GetChunk(2, 2); !ok {
		t.Fatalf("expected loaded chunk before unload")
	}

	instance.UnloadChunk(2, 2)
	instance.Tick()

	if _, ok := instance.GetChunk(2, 2); ok {
		t.Fatalf("expected chunk to be removed after unload")
	}
}

func TestRemovedItemLeavesAllWorldIndexes(t *testing.T) {
	instance, err := New(Config{Name: "item_lifecycle"})
	if err != nil {
		t.Fatalf("create world: %v", err)
	}
	defer instance.Close(nil)

	item := entity.NewItemEntity(1, protocol.UUID{1, 2}, entity.Vec3{X: 0, Y: 64, Z: 0}, inventory.NewSlot(1, 1))
	instance.SpawnItemEntity(item)
	item.Remove()
	instance.Tick()

	if instance.ItemManager().Count() != 0 {
		t.Fatalf("item manager retained removed item")
	}
	if instance.EntityManager().Get(item.ID()) != nil {
		t.Fatalf("entity manager retained removed item")
	}
}

func TestDuplicateEntityIDDoesNotEnterSpecializedManager(t *testing.T) {
	instance, err := New(Config{Name: "duplicate_entity"})
	if err != nil {
		t.Fatalf("create world: %v", err)
	}
	defer instance.Close(nil)

	first := entity.NewItemEntity(1, protocol.UUID{1, 1}, entity.Vec3{}, inventory.NewSlot(1, 1))
	second := entity.NewItemEntity(1, protocol.UUID{2, 2}, entity.Vec3{}, inventory.NewSlot(2, 1))
	instance.SpawnItemEntity(first)
	instance.SpawnItemEntity(second)

	if instance.ItemManager().Count() != 1 {
		t.Fatalf("item manager count = %d, want 1", instance.ItemManager().Count())
	}
	if got := instance.ItemManager().Get(1); got != first {
		t.Fatal("duplicate item replaced the authoritative entity")
	}
}
