package crafting

import (
	"encoding/json"
	"strings"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	remainder "github.com/vitismc/vitis/internal/data/generated/recipe_remainder"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
)

// Evaluate resolves a complete stack grid and returns a pure crafting transaction.
func (r *Runtime) Evaluate(grid []inventory.ItemStack, width int) (inventory.CraftResult, bool) {
	set := r.current()
	if set == nil || width < 2 || width > 3 || len(grid) != width*width {
		return inventory.CraftResult{}, false
	}
	ids := make([]int32, len(grid))
	for i, s := range grid {
		if !s.Empty() {
			ids[i] = s.ItemID
		}
	}
	for _, recipe := range set.crafting {
		var output inventory.ItemStack
		var extra []inventory.ItemStack
		var ok bool
		if recipe.kind == "minecraft:crafting_transmute" {
			if recipe.accepts(ids, width) {
				output, ok = transmuteOutput(recipe, grid)
			}
		} else if strings.HasPrefix(recipe.kind, "minecraft:crafting_special_") || recipe.kind == "minecraft:crafting_decorated_pot" {
			output, extra, ok = specialOutput(recipe.kind, grid, width)
		} else if recipe.accepts(ids, width) {
			output = recipe.output.Clone()
			if output.Empty() {
				output = inventory.NewSlot(recipe.result, recipe.count)
			}
			ok = true
		}
		if ok && recipe.kind == "minecraft:crafting_special_mapextending" {
			value, exists := grid[4].Component(dc.ComponentMapId)
			ok = exists && r.mapCanExtend != nil && r.mapCanExtend(int32(value.(inventory.VarIntComponent)))
		}
		if !ok || output.Empty() {
			continue
		}
		result := inventory.CraftResult{RecipeID: recipe.id, Output: output, Consume: make([]int32, len(grid)), Remainders: make([]inventory.ItemStack, len(grid))}
		for i, s := range grid {
			if !s.Empty() {
				result.Consume[i] = 1
				if name := remainder.GetRemainder(item.NameByID(s.ItemID)); name != "" {
					result.Remainders[i] = inventory.NewSlot(item.IDByName(name), 1)
				}
			}
		}
		if extra != nil {
			for i, s := range extra {
				if !s.Empty() {
					result.Remainders[i] = s.Clone()
				}
			}
		}
		return result, true
	}
	return inventory.CraftResult{}, false
}

// accepts matches one shaped or shapeless recipe against the entire grid.
func (r runtimeRecipe) accepts(grid []int32, width int) bool {
	if len(r.ingredients) > 0 {
		return matchRuntimeShapeless(grid, r.ingredients)
	}
	if len(r.pattern) == 0 {
		return false
	}
	minR, maxR, minC, maxC := trimBounds(grid, width, len(grid)/width)
	if maxR-minR+1 != len(r.pattern) || maxC-minC+1 != len(r.pattern[0]) {
		return false
	}
	return matchRuntimePattern(grid, width, minR, minC, r.pattern, false) || matchRuntimePattern(grid, width, minR, minC, r.pattern, true)
}

// transmuteOutput transfers the source stack's components to a new item type.
func transmuteOutput(r runtimeRecipe, grid []inventory.ItemStack) (inventory.ItemStack, bool) {
	for i, s := range grid {
		if s.Empty() || !r.ingredients[0].matches(s.ItemID) {
			continue
		}
		for j, material := range grid {
			if i != j && !material.Empty() && r.ingredients[1].matches(material.ItemID) {
				s = s.Clone()
				s.ItemID = r.result
				s.ItemCount = 1
				if s.PersistentData != nil {
					s.PersistentData.PutString("id", item.NameByID(s.ItemID))
				}
				return s, true
			}
		}
	}
	return inventory.ItemStack{}, false
}

// recipeOutput decodes component-bearing results once during recipe compilation.
func recipeOutput(raw json.RawMessage) (inventory.ItemStack, error) {
	var definition struct{ Result json.RawMessage }
	if err := json.Unmarshal(raw, &definition); err != nil {
		return inventory.ItemStack{}, err
	}
	t, value, err := nbt.FromJSON(definition.Result)
	if err != nil {
		return inventory.ItemStack{}, err
	}
	if t != nbt.TagCompound {
		return inventory.ItemStack{}, nil
	}
	return inventory.ItemStackFromNBT(value.(*nbt.Compound))
}

// copyComponent copies one explicitly set component without sharing mutable state.
func copyComponent(to *inventory.ItemStack, from inventory.ItemStack, id int32) {
	if value, ok := from.Component(id); ok {
		to.SetComponent(id, value)
	}
}

// componentFields returns compound codec fields or an empty immutable view.
func componentFields(s inventory.ItemStack, id int32) map[string]any {
	if value, ok := s.Component(id); ok {
		if structured, ok := value.(inventory.StructuredComponent); ok {
			if m, ok := structured.Data.(map[string]any); ok {
				return m
			}
		}
	}
	return map[string]any{}
}

// hasComponent reports whether a stack explicitly contains a component.
func hasComponent(s inventory.ItemStack, id int32) bool { _, ok := s.Component(id); return ok }

// damageLimit resolves the effective durability used by repair recipes.
func damageLimit(s inventory.ItemStack) int32 {
	if v, ok := s.Component(dc.ComponentMaxDamage); ok {
		if n, ok := v.(inventory.VarIntComponent); ok {
			return int32(n)
		}
	}
	return item.MaxDurability(s.ItemID)
}
