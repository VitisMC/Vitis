package crafting

import (
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
)

// recipeStack creates a vanilla test ingredient with its item defaults.
func recipeStack(name string) inventory.ItemStack {
	return inventory.NewSlot(item.IDByName("minecraft:"+name), 1)
}

// TestSpecialRecipeMatrix verifies every built-in grid recipe and invalid extras.
func TestSpecialRecipeMatrix(t *testing.T) {
	sword := recipeStack("diamond_sword")
	sword.SetDamage(1000)
	book, err := inventory.ItemStackFromNBT(nbt.NewCompound().PutString("id", "minecraft:written_book").PutInt("count", 1).PutCompound("components", nbt.NewCompound().PutCompound("minecraft:written_book_content", nbt.NewCompound().PutString("title", "Title").PutString("author", "Tester").PutInt("generation", 0).PutList("pages", nbt.NewList(nbt.TagString)))))
	if err != nil {
		t.Fatal(err)
	}
	filled := recipeStack("filled_map")
	filled.SetComponent(dc.ComponentMapId, inventory.VarIntComponent(7))
	banner := recipeStack("white_banner")
	banner.SetComponent(dc.ComponentBannerPatterns, inventory.StructuredComponent{Data: map[string]any{"layers": []any{map[string]any{"pattern": inventory.RegistryHolder{ID: 0}, "colorId": int32(14)}}}})
	star := recipeStack("firework_star")
	star.SetComponent(dc.ComponentFireworkExplosion, inventory.FireworkExplosionComponent{Shape: 2, Colors: []int32{11743532}, Trail: true})
	cases := []struct {
		name   string
		grid   []inventory.ItemStack
		output string
		count  int32
	}{
		{"repairitem", []inventory.ItemStack{sword, sword}, "diamond_sword", 1},
		{"armordye", []inventory.ItemStack{recipeStack("leather_chestplate"), recipeStack("red_dye")}, "leather_chestplate", 1},
		{"mapcloning", []inventory.ItemStack{filled, recipeStack("map"), recipeStack("map")}, "filled_map", 3},
		{"mapextending", []inventory.ItemStack{recipeStack("paper"), recipeStack("paper"), recipeStack("paper"), recipeStack("paper"), filled, recipeStack("paper"), recipeStack("paper"), recipeStack("paper"), recipeStack("paper")}, "filled_map", 1},
		{"bookcloning", []inventory.ItemStack{book, recipeStack("writable_book")}, "written_book", 1},
		{"bannerduplicate", []inventory.ItemStack{banner, recipeStack("white_banner")}, "white_banner", 1},
		{"shielddecoration", []inventory.ItemStack{recipeStack("shield"), banner}, "shield", 1},
		{"tippedarrow", []inventory.ItemStack{recipeStack("arrow"), recipeStack("arrow"), recipeStack("arrow"), recipeStack("arrow"), recipeStack("lingering_potion"), recipeStack("arrow"), recipeStack("arrow"), recipeStack("arrow"), recipeStack("arrow")}, "tipped_arrow", 8},
		{"firework_rocket", []inventory.ItemStack{recipeStack("paper"), recipeStack("gunpowder"), star}, "firework_rocket", 3},
		{"firework_star", []inventory.ItemStack{recipeStack("gunpowder"), recipeStack("red_dye"), recipeStack("diamond"), recipeStack("glowstone_dust")}, "firework_star", 1},
		{"firework_star_fade", []inventory.ItemStack{star, recipeStack("blue_dye")}, "firework_star", 1},
		{"decorated_pot", []inventory.ItemStack{{}, recipeStack("brick"), {}, recipeStack("brick"), {}, recipeStack("archer_pottery_sherd"), {}, recipeStack("brick"), {}}, "decorated_pot", 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			grid := make([]inventory.ItemStack, 9)
			copy(grid, test.grid)
			kind := "minecraft:crafting_special_" + test.name
			if test.name == "decorated_pot" {
				kind = "minecraft:crafting_decorated_pot"
			}
			result, remainders, ok := specialOutput(kind, grid, 3)
			if !ok || item.NameByID(result.ItemID) != "minecraft:"+test.output || result.ItemCount != test.count {
				t.Fatalf("output=%+v valid=%v", result, ok)
			}
			if _, err := inventory.ItemStackToNBT(result); err != nil {
				t.Fatalf("persistent output: %v", err)
			}
			if test.name == "bookcloning" || test.name == "bannerduplicate" {
				found := false
				for _, stack := range remainders {
					found = found || !stack.Empty()
				}
				if !found {
					t.Fatal("original ingredient not returned")
				}
			}
			bad := make([]inventory.ItemStack, 9)
			copy(bad, grid)
			replaced := false
			for i, stack := range bad {
				if stack.Empty() {
					bad[i] = recipeStack("dirt")
					replaced = true
					break
				}
			}
			if !replaced {
				bad[0] = recipeStack("dirt")
			}
			if _, _, valid := specialOutput(kind, bad, 3); valid {
				t.Fatal("accepted unrelated ingredient")
			}
		})
	}
}

// TestSpecialRecipeComponentLimits verifies repairs, book generations, and shield reuse.
func TestSpecialRecipeComponentLimits(t *testing.T) {
	sword := recipeStack("diamond_sword")
	sword.RemoveComponent(dc.ComponentDamage)
	if _, _, ok := specialOutput("minecraft:crafting_special_repairitem", []inventory.ItemStack{sword, recipeStack("diamond_sword"), {}, {}}, 2); ok {
		t.Fatal("repaired an item with removed damage")
	}
	shield := recipeStack("shield")
	shield.SetComponent(dc.ComponentBaseColor, inventory.VarIntComponent(0))
	if _, _, ok := specialOutput("minecraft:crafting_special_shielddecoration", []inventory.ItemStack{shield, recipeStack("white_banner"), {}, {}}, 2); ok {
		t.Fatal("decorated a shield twice")
	}
}

// TestSpecialRecipeBoundaryRestrictions rejects exhausted copies and invalid fireworks.
func TestSpecialRecipeBoundaryRestrictions(t *testing.T) {
	book, err := inventory.ItemStackFromNBT(nbt.NewCompound().PutString("id", "minecraft:written_book").PutCompound("components", nbt.NewCompound().PutCompound("minecraft:written_book_content", nbt.NewCompound().PutString("title", "Title").PutString("author", "Tester").PutInt("generation", 2).PutList("pages", nbt.NewList(nbt.TagString)))))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := specialOutput("minecraft:crafting_special_bookcloning", []inventory.ItemStack{book, recipeStack("writable_book"), {}, {}}, 2); ok {
		t.Fatal("copied a second-generation book")
	}
	grid := []inventory.ItemStack{recipeStack("paper"), recipeStack("gunpowder"), recipeStack("gunpowder"), recipeStack("gunpowder"), recipeStack("gunpowder"), {}, {}, {}, {}}
	if _, _, ok := specialOutput("minecraft:crafting_special_firework_rocket", grid, 3); ok {
		t.Fatal("accepted four gunpowder slots")
	}
	grid = []inventory.ItemStack{recipeStack("gunpowder"), recipeStack("red_dye"), recipeStack("feather"), recipeStack("fire_charge")}
	if _, _, ok := specialOutput("minecraft:crafting_special_firework_star", grid, 2); ok {
		t.Fatal("accepted multiple firework shapes")
	}
	banner := recipeStack("white_banner")
	layers := make([]any, 7)
	for i := range layers {
		layers[i] = map[string]any{"pattern": inventory.RegistryHolder{ID: 0}, "colorId": int32(0)}
	}
	banner.SetComponent(dc.ComponentBannerPatterns, inventory.StructuredComponent{Data: map[string]any{"layers": layers}})
	if _, _, ok := specialOutput("minecraft:crafting_special_bannerduplicate", []inventory.ItemStack{banner, recipeStack("white_banner"), {}, {}}, 2); ok {
		t.Fatal("copied a banner exceeding six patterns")
	}
}
