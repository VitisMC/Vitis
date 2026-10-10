package crafting

import (
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
)

// TestStonecuttingComponentsAndReload verifies immutable results and resource rollback.
func TestStonecuttingComponentsAndReload(t *testing.T) {
	pack := runtimeTestPack(t)
	path := "data/test/recipe/slabs.json"
	writeRuntimeResource(t, pack, path, `{"type":"minecraft:stonecutting","ingredient":"minecraft:stone","result":{"id":"minecraft:stone_slab","count":2,"components":{"minecraft:max_stack_size":16,"minecraft:custom_data":{"marker":"keep"}}}}`)
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{pack}); err != nil {
		t.Fatal(err)
	}
	input := item.IDByName("minecraft:stone")
	choices := runtime.StonecuttingChoices(input)
	if len(choices) != 1 || choices[0].Output.MaxStackSize() != 16 {
		t.Fatal("component-bearing result missing")
	}
	persisted, err := inventory.ItemStackToNBT(choices[0].Output)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := inventory.ItemStackFromNBT(persisted)
	if err != nil || !restored.StackableWith(choices[0].Output) {
		t.Fatalf("result persistence: %v", err)
	}
	choices[0].Output.SetComponent(dc.ComponentMaxStackSize, inventory.VarIntComponent(32))
	if runtime.StonecuttingChoices(input)[0].Output.MaxStackSize() != 16 {
		t.Fatal("result mutation changed resource snapshot")
	}
	for _, display := range runtime.Catalog().Displays {
		if display.ResourceID == "test:slabs" && display.Entry.Display.Result.Stack.MaxStackSize() != 16 {
			t.Fatal("client display lost components")
		}
	}
	previous := resources.Current()
	writeRuntimeResource(t, pack, path, `{"type":"minecraft:stonecutting","ingredient":"minecraft:stone","result":{"id":"minecraft:stone_slab","count":2,"components":{"minecraft:max_stack_size":0}}}`)
	if err := resources.Reload([]string{pack}); err == nil {
		t.Fatal("invalid component reload accepted")
	}
	if resources.Current() != previous || runtime.StonecuttingChoices(input)[0].Output.MaxStackSize() != 16 {
		t.Fatal("failed reload replaced valid menu recipes")
	}
}
