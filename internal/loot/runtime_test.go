package loot

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/vitismc/vitis/internal/datapack"
)

// TestRuntimeLoadsAndOverridesTables verifies resource indexing and pack priority.
func TestRuntimeLoadsAndOverridesTables(t *testing.T) {
	first := lootTestPack(t)
	second := lootTestPack(t)
	writeLootResource(t, first, "data/test/loot_table/blocks/example.json", `{"type":"minecraft:block","pools":[{"rolls":1,"bonus_rolls":0,"entries":[{"type":"minecraft:item","name":"minecraft:stone"}]}]}`)
	writeLootResource(t, second, "data/test/loot_table/blocks/example.json", `{"type":"minecraft:block","pools":[{"rolls":1,"bonus_rolls":0,"entries":[{"type":"minecraft:item","name":"minecraft:diamond"}]}]}`)
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{first, second}); err != nil {
		t.Fatal(err)
	}
	table := runtime.Table("test:blocks/example")
	if table == nil || table.Pools[0].Entries[0].Name != "minecraft:diamond" {
		t.Fatalf("active table = %#v", table)
	}
}

// TestRuntimeFailedReloadKeepsTables verifies atomic rollback on invalid loot.
func TestRuntimeFailedReloadKeepsTables(t *testing.T) {
	pack := lootTestPack(t)
	path := "data/test/loot_table/blocks/example.json"
	writeLootResource(t, pack, path, `{"type":"minecraft:block","pools":[{"rolls":1,"entries":[{"type":"minecraft:item","name":"minecraft:stone"}]}]}`)
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{pack}); err != nil {
		t.Fatal(err)
	}
	previous := resources.Current()
	writeLootResource(t, pack, path, `{"type":"minecraft:block","pools":[{"rolls":1,"entries":[{"type":"minecraft:item","name":"test:missing"}]}]}`)
	if err := resources.Reload([]string{pack}); err == nil {
		t.Fatal("accepted invalid item")
	}
	if resources.Current() != previous || runtime.TableCount() != 1 {
		t.Fatal("failed reload changed active tables")
	}
}

// TestRuntimeRollsWeightedNestedTables verifies one weighted choice per roll.
func TestRuntimeRollsWeightedNestedTables(t *testing.T) {
	pack := lootTestPack(t)
	writeLootResource(t, pack, "data/test/loot_table/child.json", `{"type":"minecraft:fishing","pools":[{"rolls":1,"entries":[{"type":"minecraft:item","name":"minecraft:cod"}]}]}`)
	writeLootResource(t, pack, "data/test/loot_table/root.json", `{"type":"minecraft:fishing","pools":[{"rolls":1,"entries":[{"type":"minecraft:loot_table","value":"test:child","weight":3},{"type":"minecraft:item","name":"minecraft:salmon","weight":1}]}]}`)
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{pack}); err != nil {
		t.Fatal(err)
	}
	ctx := NewContext(rand.New(rand.NewSource(42)))
	counts := map[string]int{}
	for range 1000 {
		drops := runtime.Roll("test:root", ctx)
		if len(drops) != 1 {
			t.Fatalf("drops = %v", drops)
		}
		counts[drops[0].ItemID]++
	}
	if counts["minecraft:cod"] < 650 || counts["minecraft:cod"] > 850 || counts["minecraft:salmon"] == 0 {
		t.Fatalf("weighted counts = %v", counts)
	}
}

// TestRuntimeRollsVanillaFishing verifies nested weighted fishing tables from generated resources.
func TestRuntimeRollsVanillaFishing(t *testing.T) {
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{"../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	ctx := NewContext(rand.New(rand.NewSource(9))).WithOpenWater(true).WithLuck(0)
	counts := map[string]int{}
	for range 2000 {
		drops := runtime.Roll("minecraft:gameplay/fishing", ctx)
		if len(drops) != 1 {
			t.Fatalf("fishing drops = %v", drops)
		}
		counts[drops[0].ItemID]++
	}
	if counts["minecraft:cod"] == 0 || counts["minecraft:name_tag"] == 0 {
		t.Fatalf("fishing distribution missed fish or treasure: %v", counts)
	}
}

// lootTestPack creates an empty data pack.
func lootTestPack(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pack.mcmeta"), []byte(`{"pack":{"description":"test","pack_format":61}}`), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

// writeLootResource writes one loot resource to a test pack.
func writeLootResource(t *testing.T, root, name, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}
