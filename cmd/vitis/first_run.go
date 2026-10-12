package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vitismc/vitis/internal/config"
	"github.com/vitismc/vitis/internal/data"
	"github.com/vitismc/vitis/internal/operator"
	"gopkg.in/yaml.v3"
)

// initializeServerFiles writes defaults only where server files are absent.
func initializeServerFiles(root, configPath string) error {
	if configPath == "" {
		configPath = config.DefaultConfigPath
	}
	cfg, err := yaml.Marshal(config.Default())
	if err != nil {
		return err
	}
	tab, err := yaml.Marshal(config.DefaultTabConfig())
	if err != nil {
		return err
	}
	type initialFile struct {
		path    string
		content []byte
	}
	files := []initialFile{
		{configPath, cfg},
		{"configs/tab.yaml", tab},
	}
	policies := []struct {
		path    string
		entries any
	}{
		{"ops.json", []operator.Operator{}},
		{"whitelist.json", []data.WhitelistEntry{}},
		{"banned-players.json", []data.BanEntry{}},
		{"banned-ips.json", []data.IPBanEntry{}},
	}
	for _, policy := range policies {
		content, err := json.MarshalIndent(policy.entries, "", "  ")
		if err != nil {
			return err
		}
		files = append(files, initialFile{policy.path, append(content, '\n')})
	}
	for _, file := range files {
		filePath := file.path
		if !filepath.IsAbs(filePath) {
			filePath = filepath.Join(root, filePath)
		}
		if err := createMissingFile(filePath, file.content); err != nil {
			return fmt.Errorf("initialize %q: %w", filePath, err)
		}
	}
	return nil
}

// createMissingFile publishes complete defaults atomically without replacing existing paths.
func createMissingFile(filePath string, content []byte) (err error) {
	if _, err := os.Lstat(filePath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(filePath), ".vitis-init-*")
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			err = errors.Join(err, file.Close())
		}
		err = errors.Join(err, os.Remove(file.Name()))
	}()
	if err = file.Chmod(0644); err != nil {
		return err
	}
	if _, err = file.Write(content); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	err = file.Close()
	closed = true
	if err != nil {
		return err
	}
	if err = os.Link(file.Name(), filePath); errors.Is(err, os.ErrExist) {
		return nil
	}
	return err
}
