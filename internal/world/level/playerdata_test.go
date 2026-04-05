package level

import (
	"path/filepath"
	"testing"
)

func TestPlayerDataSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	worldDir := filepath.Join(dir, "testworld")

	pd := DefaultPlayerData("550e8400-e29b-41d4-a716-446655440000", 100.5, 65.0, -200.5, 0)
	pd.Yaw = 90.0
	pd.Pitch = -15.0
	pd.Health = 18.5
	pd.FoodLevel = 15
	pd.GameMode = 0

	if err := pd.Save(worldDir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadPlayerData(worldDir, "550e8400-e29b-41d4-a716-446655440000")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.GameMode != 0 {
		t.Fatalf("GameMode = %d, want 0", loaded.GameMode)
	}
	if loaded.Health != 18.5 {
		t.Fatalf("Health = %f, want 18.5", loaded.Health)
	}
	if loaded.FoodLevel != 15 {
		t.Fatalf("FoodLevel = %d, want 15", loaded.FoodLevel)
	}
	if loaded.Dimension != "minecraft:overworld" {
		t.Fatalf("Dimension = %q, want minecraft:overworld", loaded.Dimension)
	}
}

func TestPlayerDataLoadNonExistent(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadPlayerData(dir, "nonexistent-uuid")
	if err == nil {
		t.Fatal("expected error for non-existent player data")
	}
}

func TestPlayerDataOverwrite(t *testing.T) {
	dir := t.TempDir()
	worldDir := filepath.Join(dir, "testworld")
	uuid := "test-uuid-1234"

	pd1 := DefaultPlayerData(uuid, 0, 65, 0, 0)
	pd1.Health = 10.0
	if err := pd1.Save(worldDir); err != nil {
		t.Fatalf("Save 1: %v", err)
	}

	pd2 := DefaultPlayerData(uuid, 50, 70, 50, 0)
	pd2.Health = 5.0
	if err := pd2.Save(worldDir); err != nil {
		t.Fatalf("Save 2: %v", err)
	}

	loaded, err := LoadPlayerData(worldDir, uuid)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Health != 5.0 {
		t.Fatalf("Health = %f, want 5.0", loaded.Health)
	}
}
