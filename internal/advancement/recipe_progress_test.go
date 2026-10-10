package advancement

import (
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"testing"
)

// TestRecipeRewardsRequireCriteriaAndSurviveSnapshots checks AND/OR and one-time grants.
func TestRecipeRewardsRequireCriteriaAndSurviveSnapshots(t *testing.T) {
	pack := advancementTestPack(t)
	writeAdvancement(t, pack, "data/test/advancement/recipe.json", `{"criteria":{"item":{"trigger":"minecraft:inventory_changed","conditions":{"items":[{"items":"minecraft:diamond","count":{"min":2}}]}},"tick":{"trigger":"minecraft:tick"},"bed":{"trigger":"minecraft:enter_block","conditions":{"block":"minecraft:red_bed"}}},"requirements":[["item"],["tick","bed"]],"rewards":{"recipes":["test:diamond"]}}`)
	writeAdvancement(t, pack, "data/test/advancement/chained.json", `{"criteria":{"recipe":{"trigger":"minecraft:recipe_unlocked","conditions":{"recipe":"test:diamond"}}},"rewards":{"recipes":["test:chain"]}}`)
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{pack}); err != nil {
		t.Fatal(err)
	}
	progress := RecipeProgress{}
	stack := inventory.NewSlot(item.IDByName("minecraft:diamond"), 1)
	if rewards := runtime.ApplyRecipeEvent(&progress, RecipeEvent{Trigger: "minecraft:inventory_changed", Inventory: []inventory.ItemStack{stack}}); len(rewards) != 0 {
		t.Fatal("matched insufficient count")
	}
	stack.ItemCount = 2
	if rewards := runtime.ApplyRecipeEvent(&progress, RecipeEvent{Trigger: "minecraft:inventory_changed", Inventory: []inventory.ItemStack{stack}}); len(rewards) != 0 {
		t.Fatal("ignored AND requirement")
	}
	restored := progress.Clone()
	if rewards := runtime.ApplyRecipeEvent(&restored, RecipeEvent{Trigger: "minecraft:enter_block", Block: "minecraft:red_bed"}); len(rewards) != 1 || rewards[0] != "test:diamond" {
		t.Fatalf("rewards=%v", rewards)
	}
	if progress.Rewarded["test:recipe"] {
		t.Fatal("snapshot shares rewarded state")
	}
	if rewards := runtime.ApplyRecipeEvent(&restored, RecipeEvent{Trigger: "minecraft:tick"}); len(rewards) != 0 {
		t.Fatal("repeated reward")
	}
	if rewards := runtime.ApplyRecipeEvent(&restored, RecipeEvent{Trigger: "minecraft:recipe_unlocked", KnowsRecipe: func(id string) bool { return id == "test:diamond" }}); len(rewards) != 1 || rewards[0] != "test:chain" {
		t.Fatalf("chained rewards=%v", rewards)
	}
}
