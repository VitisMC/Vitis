package scripts

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestGenerationManifest keeps both updaters' shared command and rollback inventory complete.
func TestGenerationManifest(t *testing.T) {
	data, err := os.ReadFile("generation_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Generators  []string `json:"generators"`
		GeneratedGo []string `json:"generated_go"`
		Fixtures    []string `json:"fixtures"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	var actual []string
	root := "../internal/data/generator"
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() == "main.go" {
			relative, err := filepath.Rel(root, filepath.Dir(path))
			if err != nil {
				return err
			}
			actual = append(actual, filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	declared := append([]string(nil), manifest.Generators...)
	sort.Strings(actual)
	sort.Strings(declared)
	if !reflect.DeepEqual(actual, declared) {
		t.Fatalf("generator inventory mismatch: actual=%v declared=%v", actual, declared)
	}
	positions := make(map[string]int)
	for i, name := range manifest.Generators {
		positions[name] = i
	}
	if positions["block"] >= positions["block/worldgen"] || positions["biome"] >= positions["worldgen"] {
		t.Fatal("generated dependencies are scheduled after their consumers")
	}
	seen := make(map[string]bool)
	for _, path := range append(manifest.GeneratedGo, manifest.Fixtures...) {
		if seen[path] || filepath.IsAbs(path) || strings.Contains(path, "..") {
			t.Fatalf("duplicate or unsafe output %q", path)
		}
		seen[path] = true
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(path))); err != nil {
			t.Fatalf("missing generation output %s: %v", path, err)
		}
	}
}
