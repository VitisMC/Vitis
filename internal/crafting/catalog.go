package crafting

import (
	"encoding/json"
	"sort"

	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// DisplayDefinition associates a recipe resource with its current client display.
type DisplayDefinition struct {
	ResourceID    string
	Entry         play.RecipeBookEntry
	placement     []Ingredient
	Width, Height int
}

// Catalog is immutable display data for one atomic resource snapshot.
type Catalog struct {
	Displays   []DisplayDefinition
	byID       map[int32]int
	byResource map[string]int
}

// Catalog returns the currently published recipe display definitions.
func (r *Runtime) Catalog() *Catalog {
	set := r.current()
	if set == nil {
		return nil
	}
	return set.catalog
}

// Display resolves an ephemeral display ID in this catalog only.
func (c *Catalog) Display(id int32) (DisplayDefinition, bool) {
	if c == nil {
		return DisplayDefinition{}, false
	}
	i, ok := c.byID[id]
	if !ok {
		return DisplayDefinition{}, false
	}
	return c.Displays[i], true
}

// Resource resolves a persistent recipe resource ID.
func (c *Catalog) Resource(id string) (DisplayDefinition, bool) {
	if c == nil {
		return DisplayDefinition{}, false
	}
	i, ok := c.byResource[id]
	if !ok {
		return DisplayDefinition{}, false
	}
	return c.Displays[i], true
}

// Placement returns ingredient alternatives for the requested grid dimensions.
func (d DisplayDefinition) Placement(width int) ([][]int32, bool) {
	if width < 2 || width > 3 || d.Width > width || d.Height > width || len(d.placement) == 0 || len(d.placement) > width*width {
		return nil, false
	}
	result := make([][]int32, width*width)
	if d.Width > 0 {
		row, col := (width-d.Height)/2, (width-d.Width)/2
		for i, ingredient := range d.placement {
			result[(row+i/d.Width)*width+col+i%d.Width] = append([]int32(nil), ingredient.ids...)
		}
	} else {
		for i, ingredient := range d.placement {
			result[i] = append([]int32(nil), ingredient.ids...)
		}
	}
	return result, true
}

// ingredientDisplay expands a registry ingredient without discarding alternatives.
func ingredientDisplay(ingredient Ingredient) play.SlotDisplay {
	if len(ingredient.ids) == 0 {
		return play.SlotDisplay{}
	}
	if len(ingredient.ids) == 1 {
		return play.SlotDisplay{Type: 2, ItemID: ingredient.ids[0]}
	}
	children := make([]play.SlotDisplay, len(ingredient.ids))
	for i, id := range ingredient.ids {
		children[i] = play.SlotDisplay{Type: 2, ItemID: id}
	}
	return play.SlotDisplay{Type: 7, Children: children}
}

// buildCatalog creates displays in deterministic resource order.
func buildCatalog(set *RecipeSet, nextID func() int32) *Catalog {
	catalog := &Catalog{byID: make(map[int32]int), byResource: make(map[string]int)}
	groups := make(map[string]int32)
	add := func(id string, display play.RecipeDisplay, placement []Ingredient, width, height int, category int32) {
		var header struct{ Category, Group string }
		_ = json.Unmarshal(set.definitions[id].Raw, &header)
		if display.Type <= 1 {
			switch header.Category {
			case "building":
				category = 0
			case "redstone":
				category = 1
			case "equipment":
				category = 2
			default:
				category = 3
			}
		}
		if display.Type == 2 {
			switch set.definitions[id].Type {
			case "minecraft:smelting":
				if header.Category == "food" {
					category = 4
				} else if header.Category == "blocks" {
					category = 5
				}
			case "minecraft:blasting":
				if header.Category == "blocks" {
					category = 7
				}
			}
		}
		group := int32(-1)
		if header.Group != "" {
			key := set.definitions[id].Type + ":" + header.Group
			var ok bool
			group, ok = groups[key]
			if !ok {
				group = int32(len(groups))
				groups[key] = group
			}
		}
		requirements := [][]int32(nil)
		if placement != nil {
			requirements = make([][]int32, 0, len(placement))
			for _, ingredient := range placement {
				if len(ingredient.ids) > 0 {
					requirements = append(requirements, append([]int32(nil), ingredient.ids...))
				}
			}
		}
		definition := DisplayDefinition{ResourceID: id, Entry: play.RecipeBookEntry{DisplayID: nextID(), Display: display, Group: group, Category: category, Requirements: requirements}, placement: placement, Width: width, Height: height}
		catalog.Displays = append(catalog.Displays, definition)
	}
	for _, recipe := range set.crafting {
		if len(recipe.pattern) == 0 && len(recipe.ingredients) == 0 {
			continue
		}
		output := recipe.output.Clone()
		if output.Empty() {
			output = inventory.NewSlot(recipe.result, recipe.count)
		}
		display := play.RecipeDisplay{Type: 0, Result: play.SlotDisplay{Type: 3, Stack: output}, Station: play.SlotDisplay{Type: 2, ItemID: item.IDByName("minecraft:crafting_table")}}
		placement := append([]Ingredient(nil), recipe.ingredients...)
		width, height := 0, 0
		if len(recipe.pattern) > 0 {
			display.Type = 1
			height = len(recipe.pattern)
			width = len(recipe.pattern[0])
			display.Width, display.Height = int32(width), int32(height)
			placement = nil
			for _, row := range recipe.pattern {
				placement = append(placement, row...)
			}
		}
		for _, ingredient := range placement {
			display.Ingredients = append(display.Ingredients, ingredientDisplay(ingredient))
		}
		add(recipe.id, display, placement, width, height, 3)
	}
	for _, recipe := range set.cooking {
		station, category := "minecraft:furnace", int32(6)
		switch recipe.Type {
		case "minecraft:blasting":
			station, category = "minecraft:blast_furnace", 8
		case "minecraft:smoking":
			station, category = "minecraft:smoker", 9
		case "minecraft:campfire_cooking":
			station, category = "minecraft:campfire", 12
		}
		add(recipe.ID, play.RecipeDisplay{Type: 2, Ingredients: []play.SlotDisplay{ingredientDisplay(recipe.Ingredient)}, Fuel: play.SlotDisplay{Type: 1}, Result: play.SlotDisplay{Type: 3, Stack: inventory.NewSlot(recipe.Result, recipe.Count)}, Station: play.SlotDisplay{Type: 2, ItemID: item.IDByName(station)}, Duration: int32(recipe.CookingTime), Experience: float32(recipe.Experience)}, nil, 0, 0, category)
	}
	for _, recipe := range set.stonecutting {
		add(recipe.ID, play.RecipeDisplay{Type: 3, Ingredients: []play.SlotDisplay{ingredientDisplay(recipe.Ingredient)}, Result: play.SlotDisplay{Type: 3, Stack: recipe.OutputStack()}, Station: play.SlotDisplay{Type: 2, ItemID: item.IDByName("minecraft:stonecutter")}}, nil, 0, 0, 10)
	}
	sort.Slice(catalog.Displays, func(i, j int) bool { return catalog.Displays[i].ResourceID < catalog.Displays[j].ResourceID })
	for i, definition := range catalog.Displays {
		catalog.byID[definition.Entry.DisplayID] = i
		catalog.byResource[definition.ResourceID] = i
	}
	return catalog
}
