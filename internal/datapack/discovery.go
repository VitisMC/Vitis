package datapack

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Discover returns the vanilla pack followed by world packs in stable priority order.
func Discover(vanillaPath, worldDirectory string) ([]string, error) {
	if vanillaPath == "" {
		return nil, fmt.Errorf("discover datapacks: empty vanilla path")
	}
	info, err := os.Lstat(vanillaPath)
	if err != nil {
		return nil, fmt.Errorf("discover vanilla datapack: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("discover vanilla datapack: path must be a directory")
	}
	paths := []string{vanillaPath}
	datapackDirectory := filepath.Join(worldDirectory, "datapacks")
	entries, err := os.ReadDir(datapackDirectory)
	if err != nil {
		if os.IsNotExist(err) {
			return paths, nil
		}
		return nil, fmt.Errorf("discover world datapacks: %w", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		entryPath := filepath.Join(datapackDirectory, entry.Name())
		info, err := os.Lstat(entryPath)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("world datapack %q is a symlink", entry.Name())
		}
		if info.IsDir() || info.Mode().IsRegular() && strings.EqualFold(filepath.Ext(entry.Name()), ".zip") {
			paths = append(paths, entryPath)
		}
	}
	sort.Strings(paths[1:])
	return paths, nil
}

// ReloadWorld discovers and atomically reloads vanilla and world packs.
func (m *Manager) ReloadWorld(vanillaPath, worldDirectory string) error {
	paths, err := Discover(vanillaPath, worldDirectory)
	if err != nil {
		return err
	}
	return m.Reload(paths)
}
