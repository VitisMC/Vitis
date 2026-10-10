package loot

import (
	"encoding/json"
	"strings"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	potion "github.com/vitismc/vitis/internal/data/generated/potion"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
)

// matchesEntity evaluates supported predicates and rejects unsupported constraints.
func matchesEntity(raw json.RawMessage, subject *EntityInfo, ctx *Context) bool {
	var properties map[string]json.RawMessage
	if json.Unmarshal(raw, &properties) != nil {
		return false
	}
	if subject == nil && len(properties) == 0 {
		return false
	}
	for key, value := range properties {
		if key == "type_specific" {
			var predicate struct {
				Type string `json:"type"`
				Open *bool  `json:"in_open_water"`
			}
			if json.Unmarshal(value, &predicate) != nil {
				return false
			}
			if predicate.Type == "minecraft:fishing_hook" {
				if predicate.Open == nil || ctx.OpenWater != *predicate.Open {
					return false
				}
				continue
			}
		}
		if subject == nil {
			return false
		}
		switch key {
		case "type":
			var name string
			if json.Unmarshal(value, &name) != nil || !matchesRegistryName("entity_type", name, subject.Type, ctx) {
				return false
			}
		case "flags":
			var flags map[string]bool
			if json.Unmarshal(value, &flags) != nil {
				return false
			}
			for flag, expected := range flags {
				switch flag {
				case "is_on_fire":
					if subject.OnFire != expected {
						return false
					}
				case "is_baby":
					if subject.Baby != expected {
						return false
					}
				default:
					return false
				}
			}
		case "type_specific":
			var fields map[string]json.RawMessage
			if json.Unmarshal(value, &fields) != nil {
				return false
			}
			for field, expected := range fields {
				switch field {
				case "type":
					var name string
					if json.Unmarshal(expected, &name) != nil || subject.Type != name {
						return false
					}
				case "color":
					var color string
					if json.Unmarshal(expected, &color) != nil || subject.Color != color {
						return false
					}
				case "sheared":
					var sheared bool
					if json.Unmarshal(expected, &sheared) != nil || subject.Sheared != sheared {
						return false
					}
				case "size":
					var size int
					if json.Unmarshal(expected, &size) != nil || subject.Size != size {
						return false
					}
				case "is_captain":
					var captain bool
					if json.Unmarshal(expected, &captain) != nil || subject.Captain != captain {
						return false
					}
				default:
					return false
				}
			}
		case "equipment":
			var equipment map[string]json.RawMessage
			if json.Unmarshal(value, &equipment) != nil {
				return false
			}
			for slot, predicate := range equipment {
				if slot != "mainhand" || !matchesMainhand(predicate, subject.Mainhand, ctx) {
					return false
				}
			}
		default:
			return false
		}
	}
	return true
}

// matchesRegistryName matches a resource location or a tag from the same snapshot.
func matchesRegistryName(registry, expected, actual string, ctx *Context) bool {
	if !strings.HasPrefix(expected, "#") {
		return expected == actual
	}
	if ctx.registryTagResolver == nil {
		return false
	}
	for _, name := range ctx.registryTagResolver(registry, strings.TrimPrefix(expected, "#")) {
		if name == actual {
			return true
		}
	}
	return false
}

// matchesMainhand evaluates item and enchantment requirements on a captured weapon.
func matchesMainhand(raw json.RawMessage, tool *ToolInfo, ctx *Context) bool {
	if tool == nil {
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return false
	}
	for name, value := range fields {
		switch name {
		case "items":
			var names []string
			var name string
			if json.Unmarshal(value, &name) == nil {
				names = []string{name}
			} else if json.Unmarshal(value, &names) != nil {
				return false
			}
			matched := false
			for _, name := range names {
				matched = matched || matchesRegistryName("item", name, tool.ItemID, ctx)
			}
			if !matched {
				return false
			}
		case "predicates":
		default:
			return false
		}
	}
	var predicate struct {
		Predicates map[string]json.RawMessage `json:"predicates"`
	}
	if json.Unmarshal(raw, &predicate) != nil {
		return false
	}
	for name, rawRequirements := range predicate.Predicates {
		if name != "minecraft:enchantments" {
			return false
		}
		var requirements []struct {
			Enchantments string               `json:"enchantments"`
			Levels       *NumberProviderRange `json:"levels"`
		}
		if json.Unmarshal(rawRequirements, &requirements) != nil {
			return false
		}
		for _, requirement := range requirements {
			matched := false
			for enchantment, level := range tool.Enchantments {
				if level <= 0 || !matchesRegistryName("enchantment", requirement.Enchantments, enchantment, ctx) {
					continue
				}
				if requirement.Levels != nil && (requirement.Levels.Min != nil && float64(level) < *requirement.Levels.Min || requirement.Levels.Max != nil && float64(level) > *requirement.Levels.Max) {
					continue
				}
				matched = true
			}
			if !matched {
				return false
			}
		}
	}
	return true
}

// applyStackFunction changes item identity and data components after count evaluation.
func applyStackFunction(fn *Function, stack *inventory.ItemStack, ctx *Context) {
	switch fn.Function {
	case "minecraft:set_components":
		for _, component := range fn.componentPatch.ComponentPatch.Added {
			stack.SetComponent(component.TypeID, component.Value)
		}
		for _, id := range fn.componentPatch.ComponentPatch.Removed {
			stack.RemoveComponent(id)
		}
	case "minecraft:furnace_smelt":
		if ctx.smeltResolver != nil {
			if name := ctx.smeltResolver(item.NameByID(stack.ItemID)); name != "" {
				stack.ItemID = item.IDByName(name)
			}
		}
	case "minecraft:set_ominous_bottle_amplifier":
		if fn.Amplifier != nil {
			stack.SetComponent(dc.ComponentOminousBottleAmplifier, inventory.VarIntComponent(max(0, min(4, int32(fn.Amplifier.Roll(ctx.Rng))))))
		}
	case "minecraft:set_potion":
		if id := potion.PotionIDByName(fn.ID); id >= 0 {
			stack.SetComponent(dc.ComponentPotionContents, inventory.StructuredComponent{Data: map[string]any{"potionId": id, "customColor": nil, "customEffects": []any{}, "customName": nil}})
		}
	}
}
