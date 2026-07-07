package world

import (
	"context"
	"testing"
	"time"

	genbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	genblock "github.com/vitismc/vitis/internal/data/generated/block"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	"github.com/vitismc/vitis/internal/world/spawning"
)

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

func TestWorldTickRunsNaturalMobSpawning(t *testing.T) {
	instance, err := New(Config{
		Name:      "spawning_world",
		Seed:      11,
		BiomeID:   genbiome.BiomePlains,
		Generator: flatSpawnGenerator{},
	})
	if err != nil {
		t.Fatalf("create world failed: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = instance.ChunkManager().Close(ctx)
	}()

	for x := int32(-3); x <= 3; x++ {
		for z := int32(-3); z <= 3; z++ {
			instance.LoadChunk(x, z)
		}
	}
	waitForChunks(t, instance, -3, 3)

	player := entity.NewPlayer(
		1000,
		protocol.UUID{1, 2},
		"tester",
		entity.Vec3{X: 0, Y: 64, Z: 0},
		entity.Vec2{},
		nil,
		4,
	)
	player.Living().SetGameMode(0)
	instance.entities.Add(player.Entity)
	instance.tracker.AddPlayer(player)
	instance.spawner = spawning.NewSpawnerWithConfig(spawning.Config{
		Seed:                3,
		IntervalTicks:       1,
		AttemptsPerCategory: 16,
		MinSpawnDistance:    24,
		MaxSpawnDistance:    32,
		MobCaps: map[entity.MobCategory]int{
			entity.MobCategoryPassive: 4,
		},
		SpawnTable: []spawning.SpawnEntry{{
			TypeName:        "pig",
			Weight:          1,
			MinGroup:        1,
			MaxGroup:        1,
			Category:        entity.MobCategoryPassive,
			BiomeCategories: []string{"plains"},
		}},
	})

	instance.Tick()

	if got := instance.MobManager().Count(); got == 0 {
		t.Fatalf("expected natural spawning to create a mob")
	}
}

type flatSpawnGenerator struct{}

func (flatSpawnGenerator) Generate(x, z int32) (*chunk.Chunk, error) {
	gameData := section.NewChunk(x, z, section.OverworldSections, genbiome.BiomePlains)
	for lx := 0; lx < 16; lx++ {
		for lz := 0; lz < 16; lz++ {
			gameData.SetBlock(lx, 62, lz, genblock.DirtDefaultState)
			gameData.SetBlock(lx, 63, lz, genblock.GrassBlockDefaultState)
		}
	}
	gameData.ComputeLight()

	c := chunk.New(x, z)
	c.SetGameData(gameData)
	c.SetState(chunk.StateLoaded)
	return c, nil
}

func waitForChunks(t *testing.T, instance *World, min, max int32) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		instance.Tick()
		allLoaded := true
		for x := min; x <= max && allLoaded; x++ {
			for z := min; z <= max; z++ {
				if _, ok := instance.GetChunk(x, z); !ok {
					allLoaded = false
					break
				}
			}
		}
		if allLoaded {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for chunks")
}
