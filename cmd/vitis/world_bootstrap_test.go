package main

import (
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vitismc/vitis/internal/config"
	"github.com/vitismc/vitis/internal/data/vanilla"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/world/level"
)

// bootstrapPack creates a validated managed pack to model an interrupted first startup.
func bootstrapPack(t *testing.T, worldDir string) string {
	t.Helper()
	path := filepath.Join(worldDir, "vanilla", protocol.CurrentMinecraftVersion)
	for _, category := range []string{"advancement", "loot_table", "recipe", "tags", "worldgen", "structure"} {
		dir := filepath.Join(path, "data", "minecraft", category)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "test.json"), []byte(`{}`), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(path, "pack.mcmeta"), []byte(`{"pack":{"description":"Vitis vanilla 1.21.4","pack_format":61}}`), 0644); err != nil {
		t.Fatal(err)
	}
	expected := vanilla.Version{ID: protocol.CurrentMinecraftVersion, Protocol: protocol.CurrentVersion, WorldVersion: protocol.CurrentDataVersion}
	expected.PackVersion.Data = datapack.CurrentPackFormat
	if err := vanilla.Validate(context.Background(), expected, path); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestFreshBootstrapPackDoesNotTriggerBackup checks installed resources alone are not a saved world.
func TestFreshBootstrapPackDoesNotTriggerBackup(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	worldDir := filepath.Join(root, "custom-world")
	bootstrapPack(t, worldDir)
	if _, err := prepareWorldLevel(worldDir, "custom-world", config.Default()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(worldDir + ".vitis-backup"); !os.IsNotExist(err) {
		t.Fatalf("fresh pack triggered backup: %v", err)
	}
	if err := preflightWorld(worldDir, config.Default()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(worldDir+".vitis-backup", "vanilla", protocol.CurrentMinecraftVersion, "pack.mcmeta")); err != nil {
		t.Fatalf("existing world backup omitted vanilla: %v", err)
	}
}

// TestInterruptedPreparationParentsDoNotTriggerBackup checks retry after cancelled extraction.
func TestInterruptedPreparationParentsDoNotTriggerBackup(t *testing.T) {
	worldDir := filepath.Join(t.TempDir(), "world")
	if err := os.MkdirAll(filepath.Join(worldDir, "vanilla"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := preflightWorld(worldDir, config.Default()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(worldDir + ".vitis-backup"); !os.IsNotExist(err) {
		t.Fatalf("cancelled first startup triggered backup: %v", err)
	}
}

// TestPreflightBacksUpUninitializedExistingData preserves files before installing any resources.
func TestPreflightBacksUpUninitializedExistingData(t *testing.T) {
	worldDir := filepath.Join(t.TempDir(), "world")
	if err := os.MkdirAll(filepath.Join(worldDir, "datapacks"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worldDir, "keep"), []byte("before resources"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := preflightWorld(worldDir, config.Default()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(worldDir, "level.dat")); !os.IsNotExist(err) {
		t.Fatalf("preflight wrote metadata: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(worldDir+".vitis-backup", "keep"))
	if err != nil || string(data) != "before resources" {
		t.Fatalf("backup = %q, error = %v", data, err)
	}
}

func TestBootstrapCreatesIndependentDimensions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "world")
	cfg := config.Default()
	cfg.World.DefaultWorldName = root
	cfg.World.BackupWorld = false
	manager, loop, err := bootstrapWorldRuntime(cfg, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if loop == nil || manager.Count() != 3 {
		t.Fatalf("runtime dimensions = %d, loop nil = %t", manager.Count(), loop == nil)
	}
	if _, ok := manager.Get(root); !ok {
		t.Fatal("overworld is missing")
	}
	if _, ok := manager.Get("minecraft:the_nether"); !ok {
		t.Fatal("Nether is missing")
	}
	if _, ok := manager.Get("minecraft:the_end"); !ok {
		t.Fatal("End is missing")
	}
	if nether, ok := manager.GetByDimension("minecraft:the_nether"); !ok || nether.Dimension() != "minecraft:the_nether" {
		t.Fatal("Nether dimension lookup is missing")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := manager.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

// TestPrepareWorldLevelCreatesMetadata checks that new worlds persist their seed.
func TestPrepareWorldLevelCreatesMetadata(t *testing.T) {
	worldDir := filepath.Join(t.TempDir(), "world")
	cfg := config.Default()
	cfg.World.Seed = 9876
	got, err := prepareWorldLevel(worldDir, "world", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got.Seed != 9876 || got.GameType != 0 || got.Difficulty != 2 {
		t.Fatalf("unexpected metadata: seed=%d game=%d difficulty=%d", got.Seed, got.GameType, got.Difficulty)
	}
	loaded, err := level.Open(worldDir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Seed != got.Seed {
		t.Fatalf("persisted seed = %d, want %d", loaded.Seed, got.Seed)
	}
}

// TestPrepareWorldLevelRejectsUnsupportedVersion leaves an older world intact.
func TestPrepareWorldLevelRejectsUnsupportedVersion(t *testing.T) {
	worldDir := filepath.Join(t.TempDir(), "world")
	if err := os.Mkdir(worldDir, 0755); err != nil {
		t.Fatal(err)
	}
	root := nbt.NewCompound().PutCompound("Data", nbt.NewCompound().PutInt("DataVersion", protocol.CurrentDataVersion-1))
	encoder := nbt.NewEncoder(128)
	if err := encoder.WriteNamedRootCompound("", root); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(filepath.Join(worldDir, "level.dat"))
	if err != nil {
		t.Fatal(err)
	}
	compressed := gzip.NewWriter(file)
	if _, err := compressed.Write(encoder.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareWorldLevel(worldDir, "world", config.Default()); err == nil || !strings.Contains(err.Error(), "unsupported DataVersion") {
		t.Fatalf("expected unsupported DataVersion error, got %v", err)
	}
	if _, err := os.Stat(worldDir + ".vitis-backup"); !os.IsNotExist(err) {
		t.Fatalf("rejected world was modified: %v", err)
	}
}

// TestPrepareWorldLevelPreservesExistingWorld checks the backup and stored seed.
func TestPrepareWorldLevelPreservesExistingWorld(t *testing.T) {
	worldDir := filepath.Join(t.TempDir(), "world")
	if err := os.MkdirAll(filepath.Join(worldDir, "region"), 0755); err != nil {
		t.Fatal(err)
	}
	chunkPath := filepath.Join(worldDir, "region", "r.0.0.mca")
	if err := os.WriteFile(chunkPath, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareWorldLevel(worldDir, "world", config.Default()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(worldDir+".vitis-backup", "level.dat")); !os.IsNotExist(err) {
		t.Fatalf("backup should precede level.dat creation, got %v", err)
	}
	cfg := config.Default()
	cfg.World.Seed = 9999
	got, err := prepareWorldLevel(worldDir, "world", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got.Seed != 42 {
		t.Fatalf("existing seed = %d, want 42", got.Seed)
	}
	backupChunk := filepath.Join(worldDir+".vitis-backup", "region", "r.0.0.mca")
	content, err := os.ReadFile(backupChunk)
	if err != nil || string(content) != "original" {
		t.Fatalf("backup content = %q, error = %v", content, err)
	}
	if err := os.WriteFile(chunkPath, []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareWorldLevel(worldDir, "world", cfg); err != nil {
		t.Fatal(err)
	}
	content, err = os.ReadFile(backupChunk)
	if err != nil || string(content) != "original" {
		t.Fatalf("backup overwritten: content = %q, error = %v", content, err)
	}
}

// TestPrepareWorldLevelRejectsDamagedMetadata checks fail-closed startup.
func TestPrepareWorldLevelRejectsDamagedMetadata(t *testing.T) {
	worldDir := filepath.Join(t.TempDir(), "world")
	metadata := level.New("world", 0, 64, 0)
	if err := metadata.Save(worldDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worldDir, "level.dat"), []byte("invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareWorldLevel(worldDir, "world", config.Default()); err == nil || !strings.Contains(err.Error(), "level") {
		t.Fatalf("expected invalid level.dat error, got %v", err)
	}
}
