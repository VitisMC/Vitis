package world

import (
	"context"
	"testing"

	gamerules "github.com/vitismc/vitis/internal/data/generated/game_rules"
	"github.com/vitismc/vitis/internal/world/level"
	"github.com/vitismc/vitis/internal/world/persistence"
)

// TestWorldMetadataSpawnRulesAndAbsoluteTime verifies persistent authoritative settings.
func TestWorldMetadataSpawnRulesAndAbsoluteTime(t *testing.T) {
	dir := t.TempDir()
	metadata := level.New("world", 1, 64, 2)
	metadata.GameRules["keepInventory"] = "true"
	metadata.GameRules["playersSleepingPercentage"] = "50"
	if err := metadata.Save(dir); err != nil {
		t.Fatal(err)
	}
	instance, err := New(Config{Name: "world", WorldDir: dir, Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	if instance.GameRule("keepInventory") != "true" {
		t.Fatal("saved rule not applied")
	}
	instance.SetWorldSpawn(-8, 75, 12, 45)
	instance.SetTimeOfDay(47999)
	instance.Tick()
	if instance.TimeOfDay() != 48000 {
		t.Fatal("day count was discarded")
	}
	if err := instance.SetGameRule("naturalRegeneration", "false"); err != nil {
		t.Fatal(err)
	}
	if err := instance.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	saved, err := level.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if saved.SpawnX != -8 || saved.SpawnAngle != 45 || saved.DayTime != 48000 || saved.GameRules["naturalRegeneration"] != "false" {
		t.Fatalf("world metadata lost: %+v", saved)
	}
	if gamerules.Default("spawnRadius") != "10" || gamerules.Default("randomTickSpeed") != "3" || gamerules.Default("doImmediateRespawn") != "false" || gamerules.Default("lavaSourceConversion") != "false" {
		t.Fatal("incorrect vanilla defaults")
	}
}

// TestMetadataAutosavePublishesImmutableSnapshot verifies asynchronous level writes.
func TestMetadataAutosavePublishesImmutableSnapshot(t *testing.T) {
	dir := t.TempDir()
	store := persistence.NewChunkStore(dir)
	instance, err := New(Config{Name: "world", WorldDir: dir, ChunkStore: store, AutosaveIntervalTicks: 1, AutosaveQueueCapacity: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := instance.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	instance.SetWorldSpawn(7, 75, 9, 60)
	if err := instance.SetGameRule("keepInventory", "true"); err != nil {
		t.Fatal(err)
	}
	instance.SetTimeOfDay(48000)
	instance.Tick()
	instance.SetWorldSpawn(99, 64, 99, 0)
	instance.autosave.close()
	saved, err := level.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if saved.SpawnX != 7 || saved.SpawnAngle != 60 || saved.DayTime != 48001 || saved.GameRules["keepInventory"] != "true" {
		t.Fatalf("autosave snapshot=%+v", saved)
	}
}
