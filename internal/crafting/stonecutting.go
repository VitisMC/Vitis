package crafting

import "github.com/vitismc/vitis/internal/inventory"

// OutputStack returns an independent result while retaining legacy ID/count fields.
func (r StonecuttingRecipe) OutputStack() inventory.ItemStack {
	if !r.output.Empty() {
		return r.output.Clone()
	}
	return inventory.NewSlot(r.Result, r.Count)
}

// StonecuttingChoices returns complete menu outputs in synchronized display order.
func (r *Runtime) StonecuttingChoices(inputID int32) []inventory.StonecuttingChoice {
	recipes := r.Stonecutting(inputID)
	choices := make([]inventory.StonecuttingChoice, len(recipes))
	for i, recipe := range recipes {
		choices[i] = inventory.StonecuttingChoice{RecipeID: recipe.ID, Output: recipe.OutputStack()}
	}
	return choices
}
