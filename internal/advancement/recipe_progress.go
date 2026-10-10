package advancement

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
)

// RecipeProgress stores criteria and one-time rewards for recipe advancements.
type RecipeProgress struct {
	Criteria map[string]map[string]bool
	Rewarded map[string]bool
}

// Clone returns independent persistent advancement progress.
func (p RecipeProgress) Clone() RecipeProgress {
	result := RecipeProgress{Criteria: make(map[string]map[string]bool), Rewarded: maps.Clone(p.Rewarded)}
	for id, criteria := range p.Criteria {
		result.Criteria[id] = maps.Clone(criteria)
	}
	return result
}

// RecipeEvent contains authoritative data for the supported recipe triggers.
type RecipeEvent struct {
	Trigger     string
	Inventory   []inventory.ItemStack
	Block       string
	KnowsRecipe func(string) bool
}

// ApplyRecipeEvent grants matching criteria and recipe rewards at most once.
func (r *Runtime) ApplyRecipeEvent(progress *RecipeProgress, event RecipeEvent) []string {
	if r == nil || progress == nil {
		return nil
	}
	if progress.Criteria == nil {
		progress.Criteria = make(map[string]map[string]bool)
	}
	if progress.Rewarded == nil {
		progress.Rewarded = make(map[string]bool)
	}
	set := r.current()
	if set == nil {
		return nil
	}
	var recipes []string
	for _, id := range set.ordered {
		definition := set.definitions[id]
		if len(definition.Rewards.Recipes) == 0 || progress.Rewarded[id] {
			continue
		}
		criteria := progress.Criteria[id]
		if criteria == nil {
			criteria = make(map[string]bool)

		}
		for name, criterion := range definition.Criteria {
			if criteria[name] || criterion.Trigger != event.Trigger {
				continue
			}
			if r.matchesRecipeCriterion(criterion, event) {
				criteria[name] = true
			}
		}
		if len(criteria) > 0 {
			progress.Criteria[id] = criteria
		}
		complete := true
		for _, group := range definition.Requirements {
			satisfied := false
			for _, name := range group {
				satisfied = satisfied || criteria[name]
			}
			complete = complete && satisfied
		}
		if complete {
			progress.Rewarded[id] = true
			recipes = append(recipes, definition.Rewards.Recipes...)
		}
	}
	return recipes
}

// matchesRecipeCriterion evaluates supported predicates without guessing unknown ones.
func (r *Runtime) matchesRecipeCriterion(criterion Criterion, event RecipeEvent) bool {
	var conditions map[string]json.RawMessage
	if len(criterion.Conditions) > 0 && json.Unmarshal(criterion.Conditions, &conditions) != nil {
		return false
	}
	for key := range conditions {
		if key != "items" && key != "slots" && key != "recipe" && key != "block" {
			return false
		}
	}
	switch event.Trigger {
	case "minecraft:tick":
		return len(conditions) == 0
	case "minecraft:recipe_unlocked":
		var id string
		if json.Unmarshal(conditions["recipe"], &id) != nil {
			return false
		}
		return event.KnowsRecipe != nil && event.KnowsRecipe(id)
	case "minecraft:enter_block":
		var name string
		if json.Unmarshal(conditions["block"], &name) != nil {
			return false
		}
		return event.Block == name
	case "minecraft:inventory_changed":
		var predicates []map[string]json.RawMessage
		if raw := conditions["items"]; len(raw) > 0 && json.Unmarshal(raw, &predicates) != nil {
			return false
		}
		for _, predicate := range predicates {
			for key := range predicate {
				if key != "items" && key != "count" {
					return false
				}
			}
			matched := false
			for _, stack := range event.Inventory {
				if stack.Empty() || !matchesIntRange(predicate["count"], stack.ItemCount) {
					continue
				}
				var choices []string
				raw := predicate["items"]
				if len(raw) == 0 {
					matched = true
					break
				}
				var name string
				if json.Unmarshal(raw, &name) == nil {
					choices = []string{name}
				} else if json.Unmarshal(raw, &choices) != nil {
					return false
				}
				for _, choice := range choices {
					if strings.HasPrefix(choice, "#") {
						names, err := r.resources.Current().ResolveTag("item", strings.TrimPrefix(choice, "#"))
						if err == nil && slices.Contains(names, item.NameByID(stack.ItemID)) {
							matched = true
						}
					} else if item.NameByID(stack.ItemID) == choice {
						matched = true
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				return false
			}
		}
		if raw := conditions["slots"]; len(raw) > 0 {
			var ranges map[string]json.RawMessage
			if json.Unmarshal(raw, &ranges) != nil {
				return false
			}
			occupied, full := int32(0), int32(0)
			for _, stack := range event.Inventory {
				if !stack.Empty() {
					occupied++
					if stack.ItemCount >= stack.MaxStackSize() {
						full++
					}
				}
			}
			values := map[string]int32{"occupied": occupied, "empty": int32(len(event.Inventory)) - occupied, "full": full}
			for name, bounds := range ranges {
				value, ok := values[name]
				if !ok || !matchesIntRange(bounds, value) {
					return false
				}
			}
		}
		return true
	}
	return false
}

// matchesIntRange validates exact or inclusive integer predicate bounds.
func matchesIntRange(raw json.RawMessage, value int32) bool {
	if len(raw) == 0 {
		return true
	}
	var exact int32
	if json.Unmarshal(raw, &exact) == nil {
		return value == exact
	}
	var bounds struct{ Min, Max *int32 }
	if json.Unmarshal(raw, &bounds) != nil {
		return false
	}
	return (bounds.Min == nil || value >= *bounds.Min) && (bounds.Max == nil || value <= *bounds.Max)
}
