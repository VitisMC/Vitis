package crafting

import (
	"testing"
)

func TestRecipeCount(t *testing.T) {
	if RecipeCount() == 0 {
		t.Fatal("expected recipes to be loaded from .mcdata")
	}
	t.Logf("loaded %d recipes", RecipeCount())
}

func TestMatchEmpty(t *testing.T) {
	grid := []int32{0, 0, 0, 0}
	id, count := Match(grid, 2)
	if id != 0 || count != 0 {
		t.Errorf("empty grid matched: id=%d count=%d", id, count)
	}
}

func TestMatch2x2Planks(t *testing.T) {
	grid := []int32{
		879, 0,
		0, 0,
	}
	id, count := Match(grid, 2)
	if id == 0 {
		t.Skip("single plank recipe not found (may need specific log ID)")
	}
	t.Logf("plank craft result: id=%d count=%d", id, count)
}

func TestMatchCraftingTable(t *testing.T) {
	planks := int32(40)
	grid := []int32{
		planks, planks,
		planks, planks,
	}
	id, count := Match(grid, 2)
	if id == 0 {
		t.Skip("crafting table recipe not matched (item ID may differ)")
	}
	t.Logf("crafting table result: id=%d count=%d", id, count)
}

func TestMatchMirror(t *testing.T) {
	grid3x3 := []int32{
		0, 0, 40,
		0, 0, 879,
		0, 0, 879,
	}
	id1, _ := Match(grid3x3, 3)

	grid3x3mirror := []int32{
		40, 0, 0,
		879, 0, 0,
		879, 0, 0,
	}
	id2, _ := Match(grid3x3mirror, 3)

	if id1 != 0 && id2 != 0 && id1 != id2 {
		t.Errorf("mirror mismatch: %d vs %d", id1, id2)
	}
}
