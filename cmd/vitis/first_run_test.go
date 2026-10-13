package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/vitismc/vitis/internal/config"
)

// TestInitializeServerFilesDefaults checks generated configuration and access policy defaults.
func TestInitializeServerFilesDefaults(t *testing.T) {
	root := t.TempDir()
	if err := initializeServerFiles(root, "configs/vitis.yaml"); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(filepath.Join(root, "configs", "vitis.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, config.Default()) {
		t.Fatal("generated config differs from built-in defaults")
	}
	tab := config.LoadTabConfig(filepath.Join(root, "configs", "tab.yaml"))
	if !reflect.DeepEqual(tab, config.DefaultTabConfig()) {
		t.Fatal("generated tab differs from built-in defaults")
	}
	for _, name := range []string{"ops.json", "whitelist.json", "banned-players.json", "banned-ips.json"} {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		var entries []json.RawMessage
		if err := json.Unmarshal(content, &entries); err != nil || entries == nil || len(entries) != 0 {
			t.Fatalf("invalid empty policy %s: %s", name, content)
		}
	}
}

// TestConcurrentDefaultPublication checks complete files and exclusive first-run ownership.
func TestConcurrentDefaultPublication(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "config.yaml")
	const writers = 8
	start := make(chan struct{})
	results := make(chan error, writers)
	var group sync.WaitGroup
	for i := 0; i < writers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			results <- createMissingFile(filePath, []byte("complete defaults\n"))
		}()
	}
	close(start)
	group.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	content, err := os.ReadFile(filePath)
	if err != nil || string(content) != "complete defaults\n" {
		t.Fatalf("incomplete defaults: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files remain: %v", err)
	}
}

// TestInitializeServerFilesPreserves verifies existing files survive repeated initialization.
func TestInitializeServerFilesPreserves(t *testing.T) {
	root := t.TempDir()
	if err := initializeServerFiles(root, "custom/vitis.yaml"); err != nil {
		t.Fatal(err)
	}
	paths := []string{"custom/vitis.yaml", "configs/tab.yaml", "ops.json", "whitelist.json", "banned-players.json", "banned-ips.json"}
	for _, name := range paths {
		if err := os.WriteFile(filepath.Join(root, name), []byte("keep original "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := initializeServerFiles(root, "custom/vitis.yaml"); err != nil {
		t.Fatal(err)
	}
	for _, name := range paths {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(content) != "keep original "+name {
			t.Fatalf("overwrote %s: %v", name, err)
		}
	}
}

// TestInitializeServerFilesAbsoluteConfig checks custom absolute paths and working-directory files.
func TestInitializeServerFilesAbsoluteConfig(t *testing.T) {
	root := t.TempDir()
	custom := filepath.Join(t.TempDir(), "other", "vitis.yaml")
	if err := initializeServerFiles(root, custom); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(custom); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "configs", "tab.yaml")); err != nil {
		t.Fatal(err)
	}
}

// TestInitializeServerFilesFailure checks that an unusable parent produces an error.
func TestInitializeServerFilesFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "blocked"), []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := initializeServerFiles(root, "blocked/vitis.yaml"); err == nil {
		t.Fatal("accepted unusable config parent")
	}
}
