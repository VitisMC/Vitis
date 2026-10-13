package crafting

import (
	"github.com/vitismc/vitis/internal/datapack"
	"testing"
)

// TestRecipeBookReloadPreservesResourceIDs rejects stale displays after atomic reload.
func TestRecipeBookReloadPreservesResourceIDs(t *testing.T) {
	pack := runtimeTestPack(t)
	writeRuntimeResource(t, pack, "data/test/recipe/example.json", `{"type":"minecraft:crafting_shapeless","ingredients":["minecraft:stone"],"result":{"id":"minecraft:diamond"}}`)
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{pack}); err != nil {
		t.Fatal(err)
	}
	book := NewRecipeBook()
	book.Bind(runtime.Catalog())
	ids := book.UnlockRecipes("test:example")
	if len(ids) != 1 {
		t.Fatal("recipe was not unlocked")
	}
	oldID := ids[0]
	state := book.Snapshot()
	if err := resources.Reload([]string{pack}); err != nil {
		t.Fatal(err)
	}
	if !book.Bind(runtime.Catalog()) || book.Knows(oldID) {
		t.Fatal("stale display survived reload")
	}
	restored := RestoreBook(state)
	restored.Bind(runtime.Catalog())
	if len(restored.Known()) != 1 || !restored.KnowsRecipe("test:example") {
		t.Fatal("resource ownership was lost")
	}
	if _, err := runtime.DeclarePacket(); err != nil {
		t.Fatal(err)
	}
}
