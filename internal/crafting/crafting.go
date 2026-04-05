package crafting

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

// Recipe represents a shaped crafting recipe.
type Recipe struct {
	Pattern [][]int32
	Result  int32
	Count   int32
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

var allRecipes []Recipe

func init() {
	_, thisFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	recipePath := filepath.Join(projectRoot, ".mcdata", "1.21.4", "recipes.json")
	data, err := os.ReadFile(recipePath)
	if err != nil {
		return
	}

	var raw map[string][]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}

	for _, entries := range raw {
		for _, entry := range entries {
			var r recipeJSON
			if err := json.Unmarshal(entry, &r); err != nil {
				continue
			}
			if r.InShape == nil || r.Result.ID == 0 {
				continue
			}
			pattern := make([][]int32, len(r.InShape))
			for i, row := range r.InShape {
				pattern[i] = make([]int32, len(row))
				for j, val := range row {
					if val == nil {
						pattern[i][j] = 0
					} else {
						var id float64
						if err := json.Unmarshal(*val, &id); err == nil {
							pattern[i][j] = int32(id)
						}
					}
				}
			}
			count := r.Result.Count
			if count <= 0 {
				count = 1
			}
			allRecipes = append(allRecipes, Recipe{
				Pattern: pattern,
				Result:  int32(r.Result.ID),
				Count:   int32(count),
			})
		}
	}
}

type recipeJSON struct {
	InShape [][](*json.RawMessage) `json:"inShape"`
	Result  resultJSON             `json:"result"`
}

type resultJSON struct {
	ID    int     `json:"id"`
	Count int32   `json:"count"`
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
