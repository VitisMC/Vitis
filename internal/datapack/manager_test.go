package datapack

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestReloadCompilersAreAtomic verifies derived resources publish with their snapshot.
func TestReloadCompilersAreAtomic(t *testing.T) {
	root := writeTestPack(t, `{"value":1}`)
	manager := &Manager{}
	wantErr := errors.New("invalid recipes")
	if err := manager.RegisterCompiler("recipes", func(snapshot *Snapshot) (any, error) {
		data, _ := snapshot.Get("data/test/recipe/example.json")
		if string(data) == `{"invalid":true}` {
			return nil, wantErr
		}
		return string(data), nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{root}); err != nil {
		t.Fatal(err)
	}
	first := manager.Current()
	compiled, ok := first.Compiled("recipes")
	if !ok || compiled != `{"value":1}` {
		t.Fatalf("compiled = %v, %v", compiled, ok)
	}
	filePath := filepath.Join(root, "data", "test", "recipe", "example.json")
	if err := os.WriteFile(filePath, []byte(`{"invalid":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{root}); !errors.Is(err, wantErr) {
		t.Fatalf("reload error = %v", err)
	}
	if manager.Current() != first {
		t.Fatal("failed compiler published a snapshot")
	}
	if err := manager.RegisterCompiler("late", func(*Snapshot) (any, error) { return true, nil }); err == nil {
		t.Fatal("accepted compiler registration after initial load")
	}
}

// TestConcurrentReloadsSerializeCompilation ensures compilers never run against overlapping reloads.
func TestConcurrentReloadsSerializeCompilation(t *testing.T) {
	root := writeTestPack(t, `{"value":1}`)
	manager := &Manager{}
	var active atomic.Int32
	var overlapping atomic.Bool
	if err := manager.RegisterCompiler("test", func(*Snapshot) (any, error) {
		if active.Add(1) != 1 {
			overlapping.Store(true)
		}
		time.Sleep(10 * time.Millisecond)
		active.Add(-1)
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
	const reloads = 8
	start := make(chan struct{})
	var workers sync.WaitGroup
	errors := make(chan error, reloads)
	for range reloads {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			errors <- manager.Reload([]string{root})
		}()
	}
	close(start)
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if overlapping.Load() {
		t.Fatal("resource compilers ran during overlapping reloads")
	}
}

// writeTestPack creates a minimal directory pack with one resource.
func writeTestPack(t *testing.T, value string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pack.mcmeta"), []byte(`{"pack":{"description":"test","pack_format":61}}`), 0600); err != nil {
		t.Fatal(err)
	}
	resourceDir := filepath.Join(root, "data", "test", "recipe")
	if err := os.MkdirAll(resourceDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resourceDir, "example.json"), []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestReloadPublishesCompleteSnapshot verifies order, immutability, and rollback.
func TestReloadPublishesCompleteSnapshot(t *testing.T) {
	first := writeTestPack(t, `{"value":1}`)
	second := writeTestPack(t, `{"value":2}`)
	manager := &Manager{}
	if err := manager.Reload([]string{first, second}); err != nil {
		t.Fatal(err)
	}
	const name = "data/test/recipe/example.json"
	snapshot := manager.Current()
	data, ok := snapshot.Get(name)
	if !ok || string(data) != `{"value":2}` {
		t.Fatalf("winning resource = %q, %v", data, ok)
	}
	if layers := snapshot.Layers(name); len(layers) != 2 || string(layers[0].Data) != `{"value":1}` {
		t.Fatalf("resource layers = %+v", layers)
	}
	data[0] = 'x'
	if current, _ := snapshot.Get(name); current[0] == 'x' {
		t.Fatal("resource data was mutable")
	}
	if err := os.WriteFile(filepath.Join(second, "pack.mcmeta"), []byte(`{"pack":{"pack_format":60}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{first, second}); err == nil {
		t.Fatal("accepted incompatible pack")
	}
	if manager.Current() != snapshot {
		t.Fatal("failed reload changed active snapshot")
	}
}

// TestReloadZIP verifies loading resources from a ZIP pack.
func TestReloadZIP(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "pack.zip")
	writeTestZIP(t, zipPath, map[string]string{
		"pack.mcmeta":                `{"pack":{"description":"test","pack_format":61}}`,
		"data/test/recipe/test.json": `{"result":"minecraft:stone"}`,
	})
	manager := &Manager{}
	if err := manager.Reload([]string{zipPath}); err != nil {
		t.Fatal(err)
	}
	if data, ok := manager.Current().Get("data/test/recipe/test.json"); !ok || len(data) == 0 {
		t.Fatalf("missing ZIP resource: %q, %v", data, ok)
	}
}

// TestReloadSupportedFormatRange accepts a pack whose declared range includes 61.
func TestReloadSupportedFormatRange(t *testing.T) {
	root := writeTestPack(t, `{}`)
	metadata := `{"pack":{"description":"test","pack_format":60,"supported_formats":[60,61]}}`
	if err := os.WriteFile(filepath.Join(root, "pack.mcmeta"), []byte(metadata), 0600); err != nil {
		t.Fatal(err)
	}
	if err := (&Manager{}).Reload([]string{root}); err != nil {
		t.Fatal(err)
	}
}

// TestReloadRejectsMalformedResource rejects invalid JSON before publication.
func TestReloadRejectsMalformedResource(t *testing.T) {
	root := writeTestPack(t, `{}`)
	filePath := filepath.Join(root, "data", "test", "recipe", "example.json")
	if err := os.WriteFile(filePath, []byte(`{`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := (&Manager{}).Reload([]string{root}); err == nil {
		t.Fatal("accepted malformed JSON resource")
	}
}

// TestReloadRejectsUnsafeZIP verifies that ZIP entries cannot escape pack roots.
func TestReloadRejectsUnsafeZIP(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "data/../escape", "data/test/recipe/../../escape"} {
		t.Run(name, func(t *testing.T) {
			zipPath := filepath.Join(t.TempDir(), "pack.zip")
			writeTestZIP(t, zipPath, map[string]string{
				"pack.mcmeta": `{"pack":{"description":"test","pack_format":61}}`,
				name:          "malicious",
			})
			if err := (&Manager{}).Reload([]string{zipPath}); err == nil {
				t.Fatalf("accepted unsafe ZIP entry %q", name)
			}
		})
	}
}

// TestReloadRejectsSymlink verifies that directory packs cannot follow links.
func TestReloadRejectsSymlink(t *testing.T) {
	root := writeTestPack(t, `{}`)
	if err := os.Symlink(filepath.Join(root, "pack.mcmeta"), filepath.Join(root, "data", "test", "recipe", "link.json")); err != nil {
		t.Fatal(err)
	}
	if err := (&Manager{}).Reload([]string{root}); err == nil {
		t.Fatal("accepted symlink in pack")
	}
}

// writeTestZIP writes named entries to a temporary ZIP archive.
func writeTestZIP(t *testing.T, filePath string, entries map[string]string) {
	t.Helper()
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, data := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
