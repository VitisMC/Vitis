package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vitismc/vitis/internal/config"
	"github.com/vitismc/vitis/internal/data/vanilla"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/session"
	"github.com/vitismc/vitis/internal/world/level"
)

// prepareWorldLevel validates existing metadata before the runtime can write chunks.
func prepareWorldLevel(worldDir, worldName string, cfg *config.Config) (*level.Level, error) {
	if err := preflightWorld(worldDir, cfg); err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(worldDir, "level.dat")); err == nil {
		return level.Open(worldDir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	worldLevel := level.New(worldName, 0, 64, 0)
	worldLevel.Seed = cfg.World.Seed
	worldLevel.GameType = session.GameModeFromString(cfg.Server.DefaultGameMode)
	worldLevel.Difficulty = int8(session.DifficultyFromString(cfg.World.Difficulty))
	worldLevel.GenerateFeatures = true
	if err := worldLevel.Save(worldDir); err != nil {
		return nil, fmt.Errorf("prepare world: save level.dat: %w", err)
	}
	return worldLevel, nil
}

// preflightWorld validates metadata and backs up existing data before resource installation.
func preflightWorld(worldDir string, cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("prepare world: nil config")
	}
	metadataPath := filepath.Join(worldDir, "level.dat")
	_, err := os.Stat(metadataPath)
	if err == nil {
		worldLevel, err := level.Open(worldDir)
		if err != nil {
			return fmt.Errorf("prepare world: %w", err)
		}
		if worldLevel.DataVersion != protocol.CurrentDataVersion {
			return fmt.Errorf("prepare world: unsupported DataVersion %d (expected %d)", worldLevel.DataVersion, protocol.CurrentDataVersion)
		}
		if cfg.World.BackupWorld {
			if _, err := level.BackupWorld(worldDir); err != nil {
				return fmt.Errorf("prepare world: %w", err)
			}
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("prepare world: stat level.dat: %w", err)
	}
	entries, err := os.ReadDir(worldDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("prepare world: read world directory: %w", err)
	}
	if len(entries) > 0 && cfg.World.BackupWorld && !onlyBootstrapResources(worldDir, entries) {
		if _, err := level.BackupWorld(worldDir); err != nil {
			return fmt.Errorf("prepare world: %w", err)
		}
	}
	return nil
}

// onlyBootstrapResources recognizes a new world containing only the managed vanilla pack.
func onlyBootstrapResources(worldDir string, entries []os.DirEntry) bool {
	if len(entries) != 1 || entries[0].Name() != "vanilla" || !entries[0].IsDir() {
		return false
	}
	versions, err := os.ReadDir(filepath.Join(worldDir, "vanilla"))
	if err != nil {
		return false
	}
	// Cancelled downloads can leave this empty parent without any world data.
	if len(versions) == 0 {
		return true
	}
	if len(versions) != 1 || versions[0].Name() != protocol.CurrentMinecraftVersion || !versions[0].IsDir() {
		return false
	}
	expected := vanilla.Version{ID: protocol.CurrentMinecraftVersion, Protocol: protocol.CurrentVersion, WorldVersion: protocol.CurrentDataVersion}
	expected.PackVersion.Data = datapack.CurrentPackFormat
	return vanilla.Validate(context.Background(), expected, filepath.Join(worldDir, "vanilla", expected.ID)) == nil
}
