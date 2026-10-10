package crafting

import (
	"os"
	"testing"

	"github.com/vitismc/vitis/internal/advancement"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// TestPinnedVanillaCraftingAndBook validates executable recipes against 1.21.4 resources.
func TestPinnedVanillaCraftingAndBook(t *testing.T) {
	path := "../../.mcdata/1.21.4/vanilla"
	if _, err := os.Stat(path + "/pack.mcmeta"); os.IsNotExist(err) {
		t.Skip("pinned vanilla resources unavailable")
	}
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	advancements, err := advancement.NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{path}); err != nil {
		t.Fatal(err)
	}
	catalog := runtime.Catalog()
	if len(catalog.Displays) < 1000 {
		t.Fatalf("incomplete vanilla displays: %d", len(catalog.Displays))
	}
	packet := &play.RecipeBookAdd{Replace: true}
	for _, definition := range catalog.Displays {
		packet.Recipes = append(packet.Recipes, definition.Entry)
		if definition.Entry.Display.Type > 1 {
			continue
		}
		requirements, ok := definition.Placement(3)
		if !ok {
			t.Fatalf("unplaceable recipe %s", definition.ResourceID)
		}
		grid := make([]inventory.ItemStack, 9)
		for i, ids := range requirements {
			if len(ids) > 0 {
				grid[i] = inventory.NewSlot(ids[0], 1)
			}
		}
		result, ok := runtime.Evaluate(grid, 3)
		if !ok {
			t.Fatalf("non-executable vanilla recipe %s", definition.ResourceID)
		}
		if _, err := inventory.ItemStackToNBT(result.Output); err != nil {
			t.Fatalf("result %s: %v", definition.ResourceID, err)
		}
	}
	buffer := protocol.NewBuffer(4096)
	if err := packet.Encode(buffer); err != nil {
		t.Fatal(err)
	}
	decoded := &play.RecipeBookAdd{}
	if err := decoded.Decode(buffer); err != nil || buffer.Remaining() != 0 || len(decoded.Recipes) != len(packet.Recipes) || !decoded.Replace {
		t.Fatalf("vanilla recipe book wire round trip: %v", err)
	}
	progress := advancement.RecipeProgress{}
	recipes := advancements.ApplyRecipeEvent(&progress, advancement.RecipeEvent{Trigger: "minecraft:inventory_changed", Inventory: []inventory.ItemStack{inventory.NewSlot(item.IDByName("minecraft:oak_planks"), 1)}})
	found := false
	for _, id := range recipes {
		found = found || id == "minecraft:stick"
	}
	if !found {
		t.Fatal("vanilla plank trigger did not unlock sticks")
	}
}
