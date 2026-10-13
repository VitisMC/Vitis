package level

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

func TestReadGzipNBTRejectsOversizedPayload(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(make([]byte, maxPersistentNBTSize+1)); err != nil {
		t.Fatalf("write gzip payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	if _, err := readGzipNBT(&compressed); err == nil {
		t.Fatal("oversized gzip payload was accepted")
	}
}

func TestNewAndSave(t *testing.T) {
	dir := t.TempDir()
	worldDir := filepath.Join(dir, "testworld")

	l := New("TestWorld", 0, 64, 0)
	if err := l.Save(worldDir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path := filepath.Join(worldDir, "level.dat")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("level.dat is empty")
	}
}

func TestSaveAndOpen(t *testing.T) {
	dir := t.TempDir()
	worldDir := filepath.Join(dir, "testworld")

	original := New("RoundTrip", 100, 65, -200)
	original.GameType = 0
	original.Difficulty = 2
	original.Hardcore = true
	original.DayTime = 12000
	original.Time = 54321

	if err := original.Save(worldDir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Open(worldDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if loaded.LevelName != "RoundTrip" {
		t.Fatalf("LevelName = %q, want %q", loaded.LevelName, "RoundTrip")
	}
	if loaded.SpawnX != 100 {
		t.Fatalf("SpawnX = %d, want 100", loaded.SpawnX)
	}
	if loaded.SpawnY != 65 {
		t.Fatalf("SpawnY = %d, want 65", loaded.SpawnY)
	}
	if loaded.SpawnZ != -200 {
		t.Fatalf("SpawnZ = %d, want -200", loaded.SpawnZ)
	}
	if loaded.GameType != 0 {
		t.Fatalf("GameType = %d, want 0", loaded.GameType)
	}
	if loaded.Difficulty != 2 {
		t.Fatalf("Difficulty = %d, want 2", loaded.Difficulty)
	}
	if !loaded.Hardcore {
		t.Fatal("Hardcore = false, want true")
	}
	if loaded.DayTime != 12000 {
		t.Fatalf("DayTime = %d, want 12000", loaded.DayTime)
	}
	if loaded.Time != 54321 {
		t.Fatalf("Time = %d, want 54321", loaded.Time)
	}
	if !loaded.AllowCommands {
		t.Fatal("AllowCommands = false, want true")
	}
	if !loaded.Initialized {
		t.Fatal("Initialized = false, want true")
	}
	if loaded.Version != 19133 {
		t.Fatalf("Version = %d, want 19133", loaded.Version)
	}
	if loaded.DataVersion != protocol.CurrentDataVersion {
		t.Fatalf("DataVersion = %d, want %d", loaded.DataVersion, protocol.CurrentDataVersion)
	}
}

func TestSaveCreatesBackup(t *testing.T) {
	dir := t.TempDir()
	worldDir := filepath.Join(dir, "testworld")

	l1 := New("First", 0, 64, 0)
	if err := l1.Save(worldDir); err != nil {
		t.Fatalf("Save 1: %v", err)
	}

	l2 := New("Second", 10, 70, 20)
	if err := l2.Save(worldDir); err != nil {
		t.Fatalf("Save 2: %v", err)
	}

	oldPath := filepath.Join(worldDir, "level.dat_old")
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatalf("level.dat_old missing: %v", err)
	}

	loaded, err := Open(worldDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if loaded.LevelName != "Second" {
		t.Fatalf("LevelName = %q, want %q", loaded.LevelName, "Second")
	}
}

func TestLevelSavePreservesUnknownNBT(t *testing.T) {
	worldDir := filepath.Join(t.TempDir(), "world")
	level := New("world", 0, 64, 0)
	level.Seed = 42
	level.ServerBrands = nil
	level.raw = nbt.NewCompound().PutCompound("Data", nbt.NewCompound().
		PutInt("DataVersion", protocol.CurrentDataVersion).
		PutString("UnknownRootData", "preserve").
		PutList("ServerBrands", nbt.NewList(nbt.TagString).Add("Imported")).
		PutCompound("WorldGenSettings", nbt.NewCompound().
			PutLong("seed", 42).
			PutBool("bonus_chest", true).
			PutString("UnknownWorldGen", "preserve")))

	if err := level.Save(worldDir); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Open(worldDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	data, _ := loaded.raw.GetCompound("Data")
	if value, _ := data.GetString("UnknownRootData"); value != "preserve" {
		t.Fatalf("UnknownRootData = %q; want preserve", value)
	}
	worldGen, _ := data.GetCompound("WorldGenSettings")
	if value, _ := worldGen.GetString("UnknownWorldGen"); value != "preserve" {
		t.Fatalf("UnknownWorldGen = %q; want preserve", value)
	}
	if bonus, _ := worldGen.GetBool("bonus_chest"); !bonus {
		t.Fatal("bonus_chest was reset")
	}
	if !loaded.BonusChestEnabled() {
		t.Fatal("bonus chest setting was not exposed to world generation")
	}
	if brands, ok := data.GetList("ServerBrands"); !ok || brands.Len() != 1 {
		t.Fatal("imported ServerBrands were not preserved")
	}
}

func TestOpenNonExistent(t *testing.T) {
	_, err := Open("/nonexistent/path/to/world")
	if err == nil {
		t.Fatal("expected error for non-existent world")
	}
}

func TestSaveRejectsUnsupportedDataVersion(t *testing.T) {
	level := New("unsupported", 0, 64, 0)
	level.DataVersion = protocol.CurrentDataVersion - 1
	if err := level.Save(t.TempDir()); err == nil {
		t.Fatal("Save succeeded for unsupported DataVersion")
	}
}
