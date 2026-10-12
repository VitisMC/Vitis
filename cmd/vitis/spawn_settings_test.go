package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/worldgen"
)

// TestDimensionSeaLevelReaderFollowsAtomicResourceReloads verifies dimension metadata and rollback.
func TestDimensionSeaLevelReaderFollowsAtomicResourceReloads(t *testing.T) {
	resources := &datapack.Manager{}
	runtime, err := worldgen.NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	vanilla := "../../.mcdata/1.21.4/vanilla"
	if err := resources.Reload([]string{vanilla}); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		id    string
		level int
		known bool
	}{
		{"minecraft:overworld", 63, true}, {"minecraft:the_nether", 32, true}, {"minecraft:the_end", 0, true}, {"test:missing", 0, false},
	} {
		level, known := dimensionSeaLevelReader(runtime, test.id)()
		if level != test.level || known != test.known {
			t.Fatalf("%s: level=%d known=%v", test.id, level, known)
		}
	}
	if _, known := dimensionSeaLevelReader(nil, "minecraft:overworld")(); known {
		t.Fatal("nil runtime has sea level")
	}
	reader := dimensionSeaLevelReader(runtime, "minecraft:overworld")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pack.mcmeta"), []byte(`{"pack":{"pack_format":61,"description":"spawn settings"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "data", "minecraft", "worldgen", "noise_settings")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "overworld.json")
	data, err := os.ReadFile(filepath.Join(vanilla, "data", "minecraft", "worldgen", "noise_settings", "overworld.json"))
	if err != nil {
		t.Fatal(err)
	}
	var definition map[string]json.RawMessage
	if err := json.Unmarshal(data, &definition); err != nil {
		t.Fatal(err)
	}
	definition["sea_level"] = json.RawMessage(`80`)
	data, err = json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	packs := []string{vanilla, root}
	if err := resources.Reload(packs); err != nil {
		t.Fatal(err)
	}
	if level, known := reader(); level != 80 || !known {
		t.Fatalf("reader did not follow reload: %d %v", level, known)
	}
	previous := resources.Current()
	if err := os.WriteFile(path, []byte(`{"sea_level":"invalid"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload(packs); err == nil {
		t.Fatal("invalid metadata accepted")
	}
	if level, known := reader(); level != 80 || !known || resources.Current() != previous {
		t.Fatal("failed reload replaced sea level")
	}
}
