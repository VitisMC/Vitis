package advancement

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vitismc/vitis/internal/datapack"
)

// TestRuntimeLoadsDefinitionsAndTracksProgress verifies validation, ordering, and OR requirements.
func TestRuntimeLoadsDefinitionsAndTracksProgress(t *testing.T) {
	pack := advancementTestPack(t)
	writeAdvancement(t, pack, "data/test/advancement/root.json", `{"criteria":{"root":{"trigger":"minecraft:tick"}}}`)
	writeAdvancement(t, pack, "data/test/advancement/child.json", `{"parent":"test:root","criteria":{"a":{"trigger":"minecraft:tick"},"b":{"trigger":"minecraft:tick"}},"requirements":[["a","b"]]}`)
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{pack}); err != nil {
		t.Fatal(err)
	}
	ids := runtime.IDs()
	if len(ids) != 2 || ids[0] != "test:root" || ids[1] != "test:child" {
		t.Fatalf("ordered IDs = %v", ids)
	}
	definition, _ := runtime.Definition("test:child")
	progress := NewProgress()
	changed, complete, err := progress.Grant(definition, "b", time.Unix(1, 0))
	if err != nil || !changed || !complete || !progress.IsComplete(definition) {
		t.Fatalf("grant = changed %v complete %v err %v", changed, complete, err)
	}
}

// TestRuntimeRejectsBrokenParentAndKeepsSnapshot verifies atomic rollback.
func TestRuntimeRejectsBrokenParentAndKeepsSnapshot(t *testing.T) {
	pack := advancementTestPack(t)
	path := "data/test/advancement/root.json"
	writeAdvancement(t, pack, path, `{"criteria":{"root":{"trigger":"minecraft:tick"}}}`)
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{pack}); err != nil {
		t.Fatal(err)
	}
	previous := resources.Current()
	writeAdvancement(t, pack, path, `{"parent":"test:missing","criteria":{"root":{"trigger":"minecraft:tick"}}}`)
	if err := resources.Reload([]string{pack}); err == nil {
		t.Fatal("accepted missing parent")
	}
	if resources.Current() != previous || runtime.Count() != 1 {
		t.Fatal("failed reload changed active definitions")
	}
}

// advancementTestPack creates an empty data pack.
func advancementTestPack(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pack.mcmeta"), []byte(`{"pack":{"description":"test","pack_format":61}}`), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

// writeAdvancement writes one advancement resource.
func writeAdvancement(t *testing.T, root, name, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}
