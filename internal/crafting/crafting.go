package crafting

import (
	genrecipes "github.com/vitismc/vitis/internal/data/generated/recipes"
)

// Recipe represents a shaped crafting recipe.
type Recipe struct {
	Pattern     [][]int32
	Ingredients []int32
	Result      int32
	Count       int32
}

// Rows returns the number of rows in the pattern.
func (r *Recipe) Rows() int { return len(r.Pattern) }

// Cols returns the number of columns in the pattern.
func (r *Recipe) Cols() int {
	max := 0
	for _, row := range r.Pattern {
		if len(row) > max {
			max = len(row)
		}
	}
	return max
}

var allRecipes = buildRecipes()

// buildRecipes converts immutable generated recipe data into the package model.
func buildRecipes() []Recipe {
	recipes := make([]Recipe, 0, len(genrecipes.CraftingRecipes))
	for _, generated := range genrecipes.CraftingRecipes {
		recipes = append(recipes, Recipe{
			Pattern:     generated.Pattern,
			Ingredients: generated.Ingredients,
			Result:      generated.Result,
			Count:       generated.Count,
		})
	}
	return recipes
}

// Match checks a crafting grid (2x2 or 3x3) against all recipes.
// grid is row-major, with 0 meaning empty.
// Returns (resultItemID, resultCount) or (0, 0) if no match.
func Match(grid []int32, gridWidth int) (int32, int32) {
	if gridWidth <= 0 {
		return 0, 0
	}
	gridHeight := len(grid) / gridWidth
	if gridHeight*gridWidth != len(grid) {
		return 0, 0
	}

	minR, maxR, minC, maxC := trimBounds(grid, gridWidth, gridHeight)
	if minR > maxR {
		return 0, 0
	}

	usedW := maxC - minC + 1
	usedH := maxR - minR + 1

	for i := range allRecipes {
		r := &allRecipes[i]
		if len(r.Ingredients) > 0 && matchShapeless(grid, r.Ingredients) {
			return r.Result, r.Count
		}
		if r.Rows() == usedH && r.Cols() == usedW {
			if matchAt(grid, gridWidth, minR, minC, r) {
				return r.Result, r.Count
			}
		}
		if mirrorMatch(grid, gridWidth, minR, minC, usedH, usedW, r) {
			return r.Result, r.Count
		}
	}
	return 0, 0
}

// matchShapeless reports whether the non-empty grid items equal the recipe multiset.
func matchShapeless(grid []int32, ingredients []int32) bool {
	if len(ingredients) == 0 {
		return false
	}
	remaining := make(map[int32]int, len(ingredients))
	for _, ingredient := range ingredients {
		if ingredient <= 0 {
			return false
		}
		remaining[ingredient]++
	}
	nonEmpty := 0
	for _, itemID := range grid {
		if itemID == 0 {
			continue
		}
		nonEmpty++
		if remaining[itemID] == 0 {
			return false
		}
		remaining[itemID]--
	}
	return nonEmpty == len(ingredients)
}

func trimBounds(grid []int32, w, h int) (minR, maxR, minC, maxC int) {
	minR, maxR = h, -1
	minC, maxC = w, -1
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			if grid[r*w+c] != 0 {
				if r < minR {
					minR = r
				}
				if r > maxR {
					maxR = r
				}
				if c < minC {
					minC = c
				}
				if c > maxC {
					maxC = c
				}
			}
		}
	}
	return
}

func matchAt(grid []int32, gridW, startR, startC int, r *Recipe) bool {
	for pr, row := range r.Pattern {
		for pc, expected := range row {
			actual := grid[(startR+pr)*gridW+(startC+pc)]
			if expected == 0 && actual == 0 {
				continue
			}
			if expected != actual {
				return false
			}
		}
	}
	return true
}

func mirrorMatch(grid []int32, gridW, startR, startC, usedH, usedW int, r *Recipe) bool {
	if r.Rows() != usedH || r.Cols() != usedW {
		return false
	}
	for pr, row := range r.Pattern {
		for pc, expected := range row {
			mirrorC := usedW - 1 - pc
			actual := grid[(startR+pr)*gridW+(startC+mirrorC)]
			if expected == 0 && actual == 0 {
				continue
			}
			if expected != actual {
				return false
			}
		}
	}
	return true
}

// RecipeCount returns the total number of loaded recipes.
func RecipeCount() int {
	return len(allRecipes)
}
