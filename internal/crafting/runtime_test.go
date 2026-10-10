package crafting

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/item"
)

// TestRuntimeLoadsTagsAndMatches verifies shaped and overlapping shapeless inputs.
func TestRuntimeLoadsTagsAndMatches(t *testing.T) {
	pack := runtimeTestPack(t)
	writeRuntimeResource(t, pack, "data/test/tags/item/planks.json", `{"values":["minecraft:oak_planks","minecraft:spruce_planks"]}`)
	writeRuntimeResource(t, pack, "data/test/recipe/sticks.json", `{
		"type":"minecraft:crafting_shaped",
		"pattern":["#","#"],
		"key":{"#":["#test:planks","minecraft:bamboo_planks"]},
		"result":{"id":"minecraft:stick","count":4}
	}`)
	writeRuntimeResource(t, pack, "data/test/recipe/overlap.json", `{
		"type":"minecraft:crafting_shapeless",
		"ingredients":["#test:planks","minecraft:oak_planks"],
		"result":{"id":"minecraft:diamond","count":1}
	}`)
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{pack}); err != nil {
		t.Fatal(err)
	}
	if runtime.RecipeCount() != 2 {
		t.Fatalf("recipe count = %d", runtime.RecipeCount())
	}
	oak := item.IDByName("minecraft:oak_planks")
	spruce := item.IDByName("minecraft:spruce_planks")
	if result, count := runtime.Match([]int32{spruce, 0, spruce, 0}, 2); result != item.IDByName("minecraft:stick") || count != 4 {
		t.Fatalf("shaped result = %d x%d", result, count)
	}
	bamboo := item.IDByName("minecraft:bamboo_planks")
	if result, count := runtime.Match([]int32{bamboo, 0, bamboo, 0}, 2); result != item.IDByName("minecraft:stick") || count != 4 {
		t.Fatalf("alternative shaped result = %d x%d", result, count)
	}
	if result, count := runtime.Match([]int32{oak, spruce, 0, 0}, 2); result != item.IDByName("minecraft:diamond") || count != 1 {
		t.Fatalf("shapeless result = %d x%d", result, count)
	}
}

// TestRuntimeFailedReloadKeepsRecipes verifies compiled recipes and resources roll back together.
func TestRuntimeFailedReloadKeepsRecipes(t *testing.T) {
	pack := runtimeTestPack(t)
	recipePath := "data/test/recipe/example.json"
	writeRuntimeResource(t, pack, recipePath, `{
		"type":"minecraft:crafting_shapeless",
		"ingredients":["minecraft:stone"],
		"result":{"id":"minecraft:diamond","count":1}
	}`)
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{pack}); err != nil {
		t.Fatal(err)
	}
	previous := resources.Current()
	writeRuntimeResource(t, pack, recipePath, `{
		"type":"minecraft:crafting_shapeless",
		"ingredients":["minecraft:stone"],
		"result":{"id":"test:missing","count":1}
	}`)
	if err := resources.Reload([]string{pack}); err == nil {
		t.Fatal("accepted invalid result item")
	}
	if resources.Current() != previous {
		t.Fatal("failed reload published resources")
	}
	result, count := runtime.Match([]int32{item.IDByName("minecraft:stone")}, 1)
	if result != item.IDByName("minecraft:diamond") || count != 1 {
		t.Fatalf("active recipe changed: %d x%d", result, count)
	}
}

// TestRuntimeCookingAndStonecutting verifies non-grid recipe indexes and tag inputs.
func TestRuntimeCookingAndStonecutting(t *testing.T) {
	pack := runtimeTestPack(t)
	writeRuntimeResource(t, pack, "data/test/tags/item/logs.json", `{"values":["minecraft:oak_log","minecraft:spruce_log"]}`)
	writeRuntimeResource(t, pack, "data/test/recipe/charcoal.json", `{
		"type":"minecraft:smelting","ingredient":"#test:logs",
		"result":{"id":"minecraft:charcoal"},"cookingtime":200,"experience":0.15
	}`)
	writeRuntimeResource(t, pack, "data/test/recipe/slab.json", `{
		"type":"minecraft:stonecutting","ingredient":"minecraft:stone",
		"result":{"id":"minecraft:stone_slab","count":2}
	}`)
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{pack}); err != nil {
		t.Fatal(err)
	}
	cooking, ok := runtime.Cooking("minecraft:smelting", item.IDByName("minecraft:spruce_log"))
	if !ok || cooking.Result != item.IDByName("minecraft:charcoal") || cooking.CookingTime != 200 || cooking.Experience != 0.15 {
		t.Fatalf("cooking recipe = %+v, found %v", cooking, ok)
	}
	choices := runtime.Stonecutting(item.IDByName("minecraft:stone"))
	if len(choices) != 1 || choices[0].Result != item.IDByName("minecraft:stone_slab") || choices[0].Count != 2 {
		t.Fatalf("stonecutting choices = %+v", choices)
	}
}

// runtimeTestPack creates an empty pack format 61 directory.
func runtimeTestPack(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pack.mcmeta"), []byte(`{"pack":{"description":"test","pack_format":61}}`), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

// writeRuntimeResource writes one resource to a test pack.
func writeRuntimeResource(t *testing.T, root, name, data string) {
	t.Helper()
	filePath := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(filePath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}
