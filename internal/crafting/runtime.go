package crafting

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
)

const compiledRecipesKey = "recipes"

// Runtime resolves crafting against the current atomic datapack snapshot.
type Runtime struct {
	resources     *datapack.Manager
	nextDisplayID atomic.Int32
	mapCanExtend  func(int32) bool
}

// RecipeSet contains every decoded recipe and executable grid recipes.
type RecipeSet struct {
	catalog      *Catalog
	definitions  map[string]Definition
	crafting     []runtimeRecipe
	cooking      []CookingRecipe
	stonecutting []StonecuttingRecipe
}

// CookingRecipe is one furnace, blast furnace, smoker, or campfire recipe.
type CookingRecipe struct {
	ID          string
	Type        string
	Ingredient  Ingredient
	Result      int32
	Count       int32
	CookingTime int
	Experience  float64
}

// StonecuttingRecipe is one selectable stonecutter result.
type StonecuttingRecipe struct {
	ID         string
	Ingredient Ingredient
	Result     int32
	Count      int32
	output     inventory.ItemStack
}

// Definition is one validated recipe resource.
type Definition struct {
	ID   string
	Type string
	Raw  json.RawMessage
}

// Ingredient is an immutable set of accepted item IDs.
type Ingredient struct {
	ids []int32
}

// runtimeRecipe is one shaped or shapeless recipe executable by a grid.
type runtimeRecipe struct {
	id          string
	kind        string
	output      inventory.ItemStack
	pattern     [][]Ingredient
	ingredients []Ingredient
	result      int32
	count       int32
}

// NewRuntime registers recipe compilation with the resource manager.
func NewRuntime(resources *datapack.Manager) (*Runtime, error) {
	if resources == nil {
		return nil, fmt.Errorf("create crafting runtime: nil resource manager")
	}
	runtime := &Runtime{resources: resources}
	if err := resources.RegisterCompiler(compiledRecipesKey, func(snapshot *datapack.Snapshot) (any, error) {
		compiled, err := compileRecipes(snapshot)
		if err != nil {
			return nil, err
		}
		set := compiled.(*RecipeSet)
		set.catalog = buildCatalog(set, func() int32 { return runtime.nextDisplayID.Add(1) - 1 })
		return set, nil
	}); err != nil {
		return nil, err
	}
	return runtime, nil
}

// Match resolves a crafting grid using the currently published recipe set.
func (r *Runtime) Match(grid []int32, gridWidth int) (int32, int32) {
	if r == nil || r.resources == nil {
		return 0, 0
	}
	snapshot := r.resources.Current()
	compiled, ok := snapshot.Compiled(compiledRecipesKey)
	if !ok {
		return 0, 0
	}
	recipes, ok := compiled.(*RecipeSet)
	if !ok || recipes == nil {
		return 0, 0
	}
	return recipes.match(grid, gridWidth)
}

// RecipeCount returns the number of definitions in the active datapacks.
func (r *Runtime) RecipeCount() int {
	if r == nil || r.resources == nil {
		return 0
	}
	compiled, ok := r.resources.Current().Compiled(compiledRecipesKey)
	if !ok {
		return 0
	}
	recipes, _ := compiled.(*RecipeSet)
	if recipes == nil {
		return 0
	}
	return len(recipes.definitions)
}

// Cooking resolves a cooking recipe type and input item.
func (r *Runtime) Cooking(recipeType string, inputID int32) (CookingRecipe, bool) {
	set := r.current()
	if set == nil || inputID <= 0 {
		return CookingRecipe{}, false
	}
	for _, recipe := range set.cooking {
		if recipe.Type == recipeType && recipe.Ingredient.matches(inputID) {
			return recipe, true
		}
	}
	return CookingRecipe{}, false
}

// Stonecutting returns all stonecutter choices for an input item.
func (r *Runtime) Stonecutting(inputID int32) []StonecuttingRecipe {
	set := r.current()
	if set == nil || inputID <= 0 {
		return nil
	}
	result := make([]StonecuttingRecipe, 0)
	for _, recipe := range set.stonecutting {
		if recipe.Ingredient.matches(inputID) {
			recipe.output = recipe.output.Clone()
			result = append(result, recipe)
		}
	}
	return result
}

// current returns the active compiled recipe set.
func (r *Runtime) current() *RecipeSet {
	if r == nil || r.resources == nil {
		return nil
	}
	compiled, ok := r.resources.Current().Compiled(compiledRecipesKey)
	if !ok {
		return nil
	}
	set, _ := compiled.(*RecipeSet)
	return set
}

// compileRecipes validates all recipe resources and compiles grid recipes.
func compileRecipes(snapshot *datapack.Snapshot) (any, error) {
	set := &RecipeSet{definitions: make(map[string]Definition)}
	for _, name := range snapshot.Names() {
		id, ok := recipeIDFromPath(name)
		if !ok {
			continue
		}
		data, _ := snapshot.Get(name)
		var header struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(data, &header); err != nil {
			return nil, fmt.Errorf("recipe %s: %w", id, err)
		}
		if !knownRecipeTypes[header.Type] {
			return nil, fmt.Errorf("recipe %s: unsupported type %q", id, header.Type)
		}
		set.definitions[id] = Definition{ID: id, Type: header.Type, Raw: append(json.RawMessage(nil), data...)}
		var recipe *runtimeRecipe
		var err error
		switch header.Type {
		case "minecraft:crafting_shaped":
			recipe, err = compileShaped(snapshot, id, data)
		case "minecraft:crafting_shapeless":
			recipe, err = compileShapeless(snapshot, id, data)
		case "minecraft:crafting_transmute":
			recipe, err = compileTransmute(snapshot, id, data)
		case "minecraft:smelting", "minecraft:blasting", "minecraft:smoking", "minecraft:campfire_cooking":
			var cooking CookingRecipe
			cooking, err = compileCooking(snapshot, id, header.Type, data)
			if err == nil {
				set.cooking = append(set.cooking, cooking)
			}
		case "minecraft:stonecutting":
			var stonecutting StonecuttingRecipe
			stonecutting, err = compileStonecutting(snapshot, id, data)
			if err == nil {
				set.stonecutting = append(set.stonecutting, stonecutting)
			}
		}
		if err != nil {
			return nil, err
		}
		if recipe == nil && (strings.HasPrefix(header.Type, "minecraft:crafting_special_") || header.Type == "minecraft:crafting_decorated_pot") {
			recipe = &runtimeRecipe{id: id}
		}
		if recipe != nil {
			recipe.kind = header.Type
			if header.Type == "minecraft:crafting_shaped" || header.Type == "minecraft:crafting_shapeless" {
				recipe.output, err = recipeOutput(data)
				if err != nil {
					return nil, fmt.Errorf("recipe %s result: %w", id, err)
				}
			}
			set.crafting = append(set.crafting, *recipe)
		}
	}
	sort.Slice(set.crafting, func(i, j int) bool { return set.crafting[i].id < set.crafting[j].id })
	sort.Slice(set.cooking, func(i, j int) bool { return set.cooking[i].ID < set.cooking[j].ID })
	sort.Slice(set.stonecutting, func(i, j int) bool { return set.stonecutting[i].ID < set.stonecutting[j].ID })
	return set, nil
}

// compileCooking compiles one cooking recipe.
func compileCooking(snapshot *datapack.Snapshot, id, recipeType string, data []byte) (CookingRecipe, error) {
	var raw struct {
		Ingredient  ingredientJSON `json:"ingredient"`
		Result      resultJSON     `json:"result"`
		CookingTime int            `json:"cookingtime"`
		Experience  float64        `json:"experience"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return CookingRecipe{}, fmt.Errorf("recipe %s: %w", id, err)
	}
	ingredient, err := resolveIngredient(snapshot, raw.Ingredient)
	if err != nil {
		return CookingRecipe{}, fmt.Errorf("recipe %s ingredient: %w", id, err)
	}
	result, count, err := resolveResult(raw.Result)
	if err != nil {
		return CookingRecipe{}, fmt.Errorf("recipe %s: %w", id, err)
	}
	if raw.CookingTime <= 0 || raw.Experience < 0 {
		return CookingRecipe{}, fmt.Errorf("recipe %s: invalid cooking values", id)
	}
	return CookingRecipe{ID: id, Type: recipeType, Ingredient: ingredient, Result: result, Count: count, CookingTime: raw.CookingTime, Experience: raw.Experience}, nil
}

// compileStonecutting compiles one stonecutter recipe.
func compileStonecutting(snapshot *datapack.Snapshot, id string, data []byte) (StonecuttingRecipe, error) {
	var raw struct {
		Ingredient ingredientJSON `json:"ingredient"`
		Result     resultJSON     `json:"result"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return StonecuttingRecipe{}, fmt.Errorf("recipe %s: %w", id, err)
	}
	ingredient, err := resolveIngredient(snapshot, raw.Ingredient)
	if err != nil {
		return StonecuttingRecipe{}, fmt.Errorf("recipe %s ingredient: %w", id, err)
	}
	output, err := recipeOutput(data)
	if err != nil {
		return StonecuttingRecipe{}, fmt.Errorf("recipe %s: %w", id, err)
	}
	if output.Empty() || output.ItemCount > output.MaxStackSize() {
		return StonecuttingRecipe{}, fmt.Errorf("recipe %s: invalid result stack", id)
	}
	return StonecuttingRecipe{ID: id, Ingredient: ingredient, Result: output.ItemID, Count: output.ItemCount, output: output}, nil
}

// compileShaped compiles a shaped recipe and its tag ingredients.
func compileShaped(snapshot *datapack.Snapshot, id string, data []byte) (*runtimeRecipe, error) {
	var raw struct {
		Pattern []string                  `json:"pattern"`
		Key     map[string]ingredientJSON `json:"key"`
		Result  resultJSON                `json:"result"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("recipe %s: %w", id, err)
	}
	if len(raw.Pattern) == 0 || len(raw.Pattern) > 3 {
		return nil, fmt.Errorf("recipe %s: shaped pattern height must be 1-3", id)
	}
	width := len([]rune(raw.Pattern[0]))
	if width == 0 || width > 3 {
		return nil, fmt.Errorf("recipe %s: shaped pattern width must be 1-3", id)
	}
	keys := make(map[rune]Ingredient, len(raw.Key))
	for symbol, value := range raw.Key {
		runes := []rune(symbol)
		if len(runes) != 1 || runes[0] == ' ' {
			return nil, fmt.Errorf("recipe %s: invalid key %q", id, symbol)
		}
		ingredient, err := resolveIngredient(snapshot, value)
		if err != nil {
			return nil, fmt.Errorf("recipe %s key %q: %w", id, symbol, err)
		}
		keys[runes[0]] = ingredient
	}
	pattern := make([][]Ingredient, len(raw.Pattern))
	used := make(map[rune]bool)
	for rowIndex, row := range raw.Pattern {
		rowRunes := []rune(row)
		if len(rowRunes) != width {
			return nil, fmt.Errorf("recipe %s: inconsistent pattern width", id)
		}
		pattern[rowIndex] = make([]Ingredient, width)
		for column, symbol := range rowRunes {
			if symbol == ' ' {
				continue
			}
			ingredient, ok := keys[symbol]
			if !ok {
				return nil, fmt.Errorf("recipe %s: undefined key %q", id, symbol)
			}
			used[symbol] = true
			pattern[rowIndex][column] = ingredient
		}
	}
	for symbol := range keys {
		if !used[symbol] {
			return nil, fmt.Errorf("recipe %s: unused key %q", id, symbol)
		}
	}
	result, count, err := resolveResult(raw.Result)
	if err != nil {
		return nil, fmt.Errorf("recipe %s: %w", id, err)
	}
	minRow, maxRow, minCol, maxCol := len(pattern), -1, width, -1
	for row, entries := range pattern {
		for col, ingredient := range entries {
			if len(ingredient.ids) > 0 {
				minRow = min(minRow, row)
				maxRow = max(maxRow, row)
				minCol = min(minCol, col)
				maxCol = max(maxCol, col)
			}
		}
	}
	if maxRow < 0 {
		return nil, fmt.Errorf("recipe %s: empty shaped pattern", id)
	}
	trimmed := make([][]Ingredient, maxRow-minRow+1)
	for row := range trimmed {
		trimmed[row] = append([]Ingredient(nil), pattern[minRow+row][minCol:maxCol+1]...)
	}
	return &runtimeRecipe{id: id, pattern: trimmed, result: result, count: count}, nil
}

// compileShapeless compiles a shapeless recipe.
func compileShapeless(snapshot *datapack.Snapshot, id string, data []byte) (*runtimeRecipe, error) {
	var raw struct {
		Ingredients []ingredientJSON `json:"ingredients"`
		Result      resultJSON       `json:"result"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("recipe %s: %w", id, err)
	}
	if len(raw.Ingredients) == 0 || len(raw.Ingredients) > 9 {
		return nil, fmt.Errorf("recipe %s: shapeless ingredient count must be 1-9", id)
	}
	ingredients := make([]Ingredient, len(raw.Ingredients))
	for index, value := range raw.Ingredients {
		ingredient, err := resolveIngredient(snapshot, value)
		if err != nil {
			return nil, fmt.Errorf("recipe %s ingredient %d: %w", id, index, err)
		}
		ingredients[index] = ingredient
	}
	result, count, err := resolveResult(raw.Result)
	if err != nil {
		return nil, fmt.Errorf("recipe %s: %w", id, err)
	}
	return &runtimeRecipe{id: id, ingredients: ingredients, result: result, count: count}, nil
}

// compileTransmute compiles the grid shape of a transmute recipe.
func compileTransmute(snapshot *datapack.Snapshot, id string, data []byte) (*runtimeRecipe, error) {
	var raw struct {
		Input    ingredientJSON `json:"input"`
		Material ingredientJSON `json:"material"`
		Result   string         `json:"result"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("recipe %s: %w", id, err)
	}
	input, err := resolveIngredient(snapshot, raw.Input)
	if err != nil {
		return nil, fmt.Errorf("recipe %s input: %w", id, err)
	}
	material, err := resolveIngredient(snapshot, raw.Material)
	if err != nil {
		return nil, fmt.Errorf("recipe %s material: %w", id, err)
	}
	result := item.IDByName(raw.Result)
	if result <= 0 {
		return nil, fmt.Errorf("recipe %s: unknown result %q", id, raw.Result)
	}
	return &runtimeRecipe{id: id, ingredients: []Ingredient{input, material}, result: result, count: 1}, nil
}

// resultJSON is a static recipe result in data pack format 61.
type resultJSON struct {
	ID    string `json:"id"`
	Count int32  `json:"count"`
}

// ingredientJSON accepts either one ingredient identifier or a list of alternatives.
type ingredientJSON []string

// UnmarshalJSON decodes the two ingredient forms supported by data pack format 61.
func (i *ingredientJSON) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		if single == "" {
			return fmt.Errorf("empty ingredient identifier")
		}
		*i = ingredientJSON{single}
		return nil
	}
	var alternatives []string
	if err := json.Unmarshal(data, &alternatives); err != nil {
		return fmt.Errorf("ingredient must be a string or string array: %w", err)
	}
	if len(alternatives) == 0 {
		return fmt.Errorf("ingredient alternatives must not be empty")
	}
	for _, alternative := range alternatives {
		if alternative == "" {
			return fmt.Errorf("empty ingredient alternative")
		}
	}
	*i = ingredientJSON(alternatives)
	return nil
}

// resolveResult validates a static item result.
func resolveResult(result resultJSON) (int32, int32, error) {
	id := item.IDByName(result.ID)
	if id <= 0 {
		return 0, 0, fmt.Errorf("unknown result %q", result.ID)
	}
	if result.Count == 0 {
		result.Count = 1
	}
	if result.Count < 1 || result.Count > item.StackSize(id) {
		return 0, 0, fmt.Errorf("invalid result count %d for %q", result.Count, result.ID)
	}
	return id, result.Count, nil
}

// resolveIngredient resolves an item or item tag to sorted numeric IDs.
func resolveIngredient(snapshot *datapack.Snapshot, values ingredientJSON) (Ingredient, error) {
	ids := make([]int32, 0, len(values))
	seen := make(map[int32]bool, len(values))
	for _, value := range values {
		var names []string
		if strings.HasPrefix(value, "#") {
			resolved, err := snapshot.ResolveTag("item", strings.TrimPrefix(value, "#"))
			if err != nil {
				return Ingredient{}, err
			}
			names = resolved
		} else {
			names = []string{value}
		}
		for _, name := range names {
			id := item.IDByName(name)
			if id <= 0 {
				return Ingredient{}, fmt.Errorf("unknown item %q", name)
			}
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	if len(ids) == 0 {
		return Ingredient{}, fmt.Errorf("empty ingredient %q", []string(values))
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return Ingredient{ids: ids}, nil
}

// matches reports whether an ingredient accepts an item ID.
func (i Ingredient) matches(id int32) bool {
	index := sort.Search(len(i.ids), func(index int) bool { return i.ids[index] >= id })
	return index < len(i.ids) && i.ids[index] == id
}

// match resolves all executable crafting recipes.
func (s *RecipeSet) match(grid []int32, width int) (int32, int32) {
	if width <= 0 || len(grid)%width != 0 {
		return 0, 0
	}
	minRow, maxRow, minColumn, maxColumn := trimBounds(grid, width, len(grid)/width)
	if minRow > maxRow {
		return 0, 0
	}
	usedHeight, usedWidth := maxRow-minRow+1, maxColumn-minColumn+1
	for index := range s.crafting {
		recipe := &s.crafting[index]
		if len(recipe.ingredients) > 0 && matchRuntimeShapeless(grid, recipe.ingredients) {
			return recipe.result, recipe.count
		}
		if len(recipe.pattern) == usedHeight && len(recipe.pattern[0]) == usedWidth {
			if matchRuntimePattern(grid, width, minRow, minColumn, recipe.pattern, false) ||
				matchRuntimePattern(grid, width, minRow, minColumn, recipe.pattern, true) {
				return recipe.result, recipe.count
			}
		}
	}
	return 0, 0
}

// matchRuntimePattern matches a shaped pattern in normal or mirrored order.
func matchRuntimePattern(grid []int32, width, startRow, startColumn int, pattern [][]Ingredient, mirrored bool) bool {
	patternWidth := len(pattern[0])
	for row, entries := range pattern {
		for column, ingredient := range entries {
			gridColumn := column
			if mirrored {
				gridColumn = patternWidth - 1 - column
			}
			actual := grid[(startRow+row)*width+startColumn+gridColumn]
			if len(ingredient.ids) == 0 {
				if actual != 0 {
					return false
				}
			} else if !ingredient.matches(actual) {
				return false
			}
		}
	}
	return true
}

// matchRuntimeShapeless performs a complete matching for overlapping tags.
func matchRuntimeShapeless(grid []int32, ingredients []Ingredient) bool {
	actual := make([]int32, 0, len(grid))
	for _, id := range grid {
		if id != 0 {
			actual = append(actual, id)
		}
	}
	if len(actual) != len(ingredients) {
		return false
	}
	used := make([]bool, len(actual))
	var match func(int) bool
	match = func(index int) bool {
		if index == len(ingredients) {
			return true
		}
		for actualIndex, id := range actual {
			if used[actualIndex] || !ingredients[index].matches(id) {
				continue
			}
			used[actualIndex] = true
			if match(index + 1) {
				return true
			}
			used[actualIndex] = false
		}
		return false
	}
	return match(0)
}

// recipeIDFromPath converts a canonical recipe path to a resource ID.
func recipeIDFromPath(name string) (string, bool) {
	parts := strings.Split(name, "/")
	if len(parts) < 4 || parts[0] != "data" || parts[2] != "recipe" || !strings.HasSuffix(parts[len(parts)-1], ".json") {
		return "", false
	}
	local := strings.TrimSuffix(strings.Join(parts[3:], "/"), ".json")
	return parts[1] + ":" + local, true
}

var knownRecipeTypes = map[string]bool{
	"minecraft:blasting":                            true,
	"minecraft:campfire_cooking":                    true,
	"minecraft:crafting_decorated_pot":              true,
	"minecraft:crafting_shaped":                     true,
	"minecraft:crafting_shapeless":                  true,
	"minecraft:crafting_special_armordye":           true,
	"minecraft:crafting_special_bannerduplicate":    true,
	"minecraft:crafting_special_bookcloning":        true,
	"minecraft:crafting_special_firework_rocket":    true,
	"minecraft:crafting_special_firework_star":      true,
	"minecraft:crafting_special_firework_star_fade": true,
	"minecraft:crafting_special_mapcloning":         true,
	"minecraft:crafting_special_mapextending":       true,
	"minecraft:crafting_special_repairitem":         true,
	"minecraft:crafting_special_shielddecoration":   true,
	"minecraft:crafting_special_tippedarrow":        true,
	"minecraft:crafting_transmute":                  true,
	"minecraft:smelting":                            true,
	"minecraft:smithing_transform":                  true,
	"minecraft:smithing_trim":                       true,
	"minecraft:smoking":                             true,
	"minecraft:stonecutting":                        true,
}

// SetMapExtensionValidator installs the pure SavedData dependency before serving.
func (r *Runtime) SetMapExtensionValidator(validator func(int32) bool) { r.mapCanExtend = validator }

// IsSpecial reports recipes that remain available when limited crafting is enabled.
func (r *Runtime) IsSpecial(id string) bool {
	set := r.current()
	if set == nil {
		return false
	}
	definition, exists := set.definitions[id]
	return exists && (strings.HasPrefix(definition.Type, "minecraft:crafting_special_") || definition.Type == "minecraft:crafting_decorated_pot")
}
