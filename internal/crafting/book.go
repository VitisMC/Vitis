package crafting

import (
	"fmt"
	"sort"
	"sync"
)

const recipeBookTypeCount = 4

// BookType identifies one client recipe book category.
type BookType int32

const (
	BookCrafting BookType = iota
	BookFurnace
	BookBlastFurnace
	BookSmoker
)

// BookSettings stores UI settings for one recipe book category.
type BookSettings struct {
	Open      bool
	Filtering bool
}

// RecipeBook stores one player's unlocked and highlighted display IDs.
type RecipeBook struct {
	mu                 sync.RWMutex
	settings           [recipeBookTypeCount]BookSettings
	known              map[int32]struct{}
	highlight          map[int32]struct{}
	resources          map[string]struct{}
	resourceHighlights map[string]struct{}
	catalog            *Catalog
}

// NewRecipeBook creates empty player recipe book state.
func NewRecipeBook() *RecipeBook {
	return &RecipeBook{known: make(map[int32]struct{}), highlight: make(map[int32]struct{}), resources: make(map[string]struct{}), resourceHighlights: make(map[string]struct{})}
}

// SetSettings updates one category's UI settings.
func (b *RecipeBook) SetSettings(bookType BookType, open, filtering bool) error {
	if b == nil || bookType < 0 || bookType >= recipeBookTypeCount {
		return fmt.Errorf("recipe book type out of range: %d", bookType)
	}
	b.mu.Lock()
	b.settings[bookType] = BookSettings{Open: open, Filtering: filtering}
	b.mu.Unlock()
	return nil
}

// Settings returns one category's UI settings.
func (b *RecipeBook) Settings(bookType BookType) (BookSettings, bool) {
	if b == nil || bookType < 0 || bookType >= recipeBookTypeCount {
		return BookSettings{}, false
	}
	b.mu.RLock()
	settings := b.settings[bookType]
	b.mu.RUnlock()
	return settings, true
}

// Unlock adds display IDs and marks newly learned recipes for highlighting.
func (b *RecipeBook) Unlock(displayIDs ...int32) []int32 {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	added := make([]int32, 0, len(displayIDs))
	for _, id := range displayIDs {
		if id < 0 {
			continue
		}
		if _, exists := b.known[id]; exists {
			continue
		}
		b.known[id] = struct{}{}
		b.highlight[id] = struct{}{}
		if definition, ok := b.catalog.Display(id); ok {
			b.resources[definition.ResourceID] = struct{}{}
			b.resourceHighlights[definition.ResourceID] = struct{}{}
		}
		added = append(added, id)
	}
	sort.Slice(added, func(i, j int) bool { return added[i] < added[j] })
	return added
}

// MarkSeen removes the highlight from a known display ID.
func (b *RecipeBook) MarkSeen(displayID int32) bool {
	if b == nil || displayID < 0 {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, known := b.known[displayID]; !known {
		return false
	}
	if _, highlighted := b.highlight[displayID]; !highlighted {
		return false
	}
	delete(b.highlight, displayID)
	if definition, ok := b.catalog.Display(displayID); ok {
		delete(b.resourceHighlights, definition.ResourceID)
	}
	return true
}

// Remove deletes known display IDs and returns those that existed.
func (b *RecipeBook) Remove(displayIDs ...int32) []int32 {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	removed := make([]int32, 0, len(displayIDs))
	for _, id := range displayIDs {
		if _, exists := b.known[id]; !exists {
			continue
		}
		delete(b.known, id)
		delete(b.highlight, id)
		if definition, ok := b.catalog.Display(id); ok {
			delete(b.resources, definition.ResourceID)
			delete(b.resourceHighlights, definition.ResourceID)
		}
		removed = append(removed, id)
	}
	sort.Slice(removed, func(i, j int) bool { return removed[i] < removed[j] })
	return removed
}

// Known returns sorted unlocked display IDs.
func (b *RecipeBook) Known() []int32 {
	if b == nil {
		return nil
	}
	b.mu.RLock()
	ids := make([]int32, 0, len(b.known))
	for id := range b.known {
		ids = append(ids, id)
	}
	b.mu.RUnlock()
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// BookState stores durable resource identifiers and UI settings.
type BookState struct {
	Recipes, Highlights []string
	Settings            [recipeBookTypeCount]BookSettings
}

// RestoreBook restores durable state without accepting persistent display IDs.
func RestoreBook(state BookState) *RecipeBook {
	b := NewRecipeBook()
	b.settings = state.Settings
	for _, id := range state.Recipes {
		b.resources[id] = struct{}{}
	}
	for _, id := range state.Highlights {
		if _, ok := b.resources[id]; ok {
			b.resourceHighlights[id] = struct{}{}
		}
	}
	return b
}

// Snapshot returns independent persistent recipe-book state.
func (b *RecipeBook) Snapshot() BookState {
	if b == nil {
		return BookState{}
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	state := BookState{Settings: b.settings}
	for id := range b.resources {
		state.Recipes = append(state.Recipes, id)
	}
	sort.Strings(state.Recipes)
	for id := range b.resourceHighlights {
		state.Highlights = append(state.Highlights, id)
	}
	sort.Strings(state.Highlights)
	return state
}

// Bind rebuilds ephemeral identifiers when the resource catalog changes.
func (b *RecipeBook) Bind(catalog *Catalog) bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.catalog == catalog {
		return false
	}
	b.catalog = catalog
	b.known = make(map[int32]struct{})
	b.highlight = make(map[int32]struct{})
	for id := range b.resources {
		if definition, ok := catalog.Resource(id); ok {
			b.known[definition.Entry.DisplayID] = struct{}{}
			if _, yes := b.resourceHighlights[id]; yes {
				b.highlight[definition.Entry.DisplayID] = struct{}{}
			}
		}
	}
	return true
}

// UnlockRecipes adds persistent resource identifiers and highlights them once.
func (b *RecipeBook) UnlockRecipes(ids ...string) []int32 {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var displays []int32
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, known := b.resources[id]; known {
			continue
		}
		b.resources[id] = struct{}{}
		b.resourceHighlights[id] = struct{}{}
		if definition, ok := b.catalog.Resource(id); ok {
			display := definition.Entry.DisplayID
			b.known[display] = struct{}{}
			b.highlight[display] = struct{}{}
			displays = append(displays, display)
		}
	}
	return displays
}

// Knows reports whether a current display has been unlocked.
func (b *RecipeBook) Knows(id int32) bool {
	if b == nil {
		return false
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, ok := b.known[id]
	return ok
}

// KnowsRecipe reports durable recipe ownership for advancement predicates.
func (b *RecipeBook) KnowsRecipe(id string) bool {
	if b == nil {
		return false
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, ok := b.resources[id]
	return ok
}

// Highlighted reports whether a current display is new to this player.
func (b *RecipeBook) Highlighted(id int32) bool {
	if b == nil {
		return false
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, ok := b.highlight[id]
	return ok
}
