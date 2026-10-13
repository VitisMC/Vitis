package level

import (
	"github.com/vitismc/vitis/internal/advancement"
	"github.com/vitismc/vitis/internal/crafting"
	"github.com/vitismc/vitis/internal/nbt"
)

var recipeSettingNames = [4][2]string{{"isGuiOpen", "isFilteringCraftable"}, {"isFurnaceGuiOpen", "isFurnaceFilteringCraftable"}, {"isBlastingFurnaceGuiOpen", "isBlastingFurnaceFilteringCraftable"}, {"isSmokerGuiOpen", "isSmokerFilteringCraftable"}}

// parseRecipeBook restores resource identifiers, settings, and recipe criteria.
func (pd *PlayerData) parseRecipeBook(root *nbt.Compound) {
	if book, ok := root.GetCompound("recipeBook"); ok {
		pd.RecipeBook.Recipes = recipeStringList(book, "recipes")
		pd.RecipeBook.Highlights = recipeStringList(book, "toBeDisplayed")
		for i, names := range recipeSettingNames {
			pd.RecipeBook.Settings[i].Open, _ = book.GetBool(names[0])
			pd.RecipeBook.Settings[i].Filtering, _ = book.GetBool(names[1])
		}
	}
	pd.RecipeProgress = advancement.RecipeProgress{Criteria: make(map[string]map[string]bool), Rewarded: make(map[string]bool)}
	if progress, ok := root.GetCompound("VitisRecipeAdvancements"); ok {
		for _, id := range progress.Keys() {
			definition, ok := progress.GetCompound(id)
			if !ok {
				continue
			}
			pd.RecipeProgress.Rewarded[id], _ = definition.GetBool("rewarded")
			criteria, _ := definition.GetCompound("criteria")
			pd.RecipeProgress.Criteria[id] = make(map[string]bool)
			for _, name := range criteria.Keys() {
				pd.RecipeProgress.Criteria[id][name], _ = criteria.GetBool(name)
			}
		}
	}
}

// writeRecipeBook writes independently owned persistent recipe state.
func (pd *PlayerData) writeRecipeBook(root *nbt.Compound) {
	book := nbt.NewCompound()
	recipes, highlights := nbt.NewList(nbt.TagString), nbt.NewList(nbt.TagString)
	for _, id := range pd.RecipeBook.Recipes {
		recipes.Add(id)
	}
	for _, id := range pd.RecipeBook.Highlights {
		highlights.Add(id)
	}
	book.PutList("recipes", recipes).PutList("toBeDisplayed", highlights)
	for i, names := range recipeSettingNames {
		settings := pd.RecipeBook.Settings[i]
		book.PutBool(names[0], settings.Open).PutBool(names[1], settings.Filtering)
	}
	root.PutCompound("recipeBook", book)
	progress := nbt.NewCompound()
	for id, criteria := range pd.RecipeProgress.Criteria {
		c := nbt.NewCompound()
		for name, granted := range criteria {
			c.PutBool(name, granted)
		}
		progress.PutCompound(id, nbt.NewCompound().PutCompound("criteria", c).PutBool("rewarded", pd.RecipeProgress.Rewarded[id]))
	}
	root.PutCompound("VitisRecipeAdvancements", progress)
}

// cloneBookState returns independent resource identifier slices.
func cloneBookState(state crafting.BookState) crafting.BookState {
	state.Recipes = append([]string(nil), state.Recipes...)
	state.Highlights = append([]string(nil), state.Highlights...)
	return state
}

// recipeStringList reads persistent resource identifiers from a string list.
func recipeStringList(c *nbt.Compound, key string) []string {
	list, ok := c.GetList(key)
	if !ok || list.ElementType() != nbt.TagString {
		return nil
	}
	result := make([]string, 0, list.Len())
	for _, value := range list.Elements() {
		if id, ok := value.(string); ok {
			result = append(result, id)
		}
	}
	return result
}
