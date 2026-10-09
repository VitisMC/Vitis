package equipment

import (
	"encoding/json"
	"fmt"
	"strings"

	ench "github.com/vitismc/vitis/internal/data/generated/enchantment"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
)

// PickupRules holds immutable item preferences and armor-change enchantment effects.
type PickupRules struct {
	itemTags map[string]map[int32]bool
	binding  map[int32]bool
}

// PickupRules acquires item-selection rules from one published snapshot.
func (r *Runtime) PickupRules() *PickupRules {
	if r != nil {
		if snapshot := r.resources.Current(); snapshot != nil {
			compiled, _ := snapshot.Compiled(compiledDamageTags)
			definitions, _ := compiled.(*equipmentDefinitions)
			if definitions != nil {
				return definitions.pickup
			}
		}
	}
	return nil
}

// IsPreferredWeapon checks the species' current resource-defined weapon preference.
func (r *PickupRules) IsPreferredWeapon(species string, stack inventory.ItemStack) bool {
	tag := ""
	switch species {
	case "skeleton", "stray":
		tag = "skeleton_preferred_weapons"
	case "drowned":
		tag = "drowned_preferred_weapons"
	default:
		return false
	}
	if r != nil {
		return r.itemTags[tag][stack.ItemID]
	}
	return species == "drowned" && stack.ItemID == item.IDByName("minecraft:trident") || species != "drowned" && stack.ItemID == item.IDByName("minecraft:bow")
}

// CanHoldItem rejects species-specific disallowed pickup items.
func (r *PickupRules) CanHoldItem(species string, stack inventory.ItemStack) bool {
	if stack.Empty() {
		return false
	}
	if species != "wither_skeleton" {
		return true
	}
	if r != nil {
		return !r.itemTags["wither_skeleton_disliked_weapons"][stack.ItemID]
	}
	return stack.ItemID != item.IDByName("minecraft:bow") && stack.ItemID != item.IDByName("minecraft:crossbow")
}

// PreventsArmorChange resolves active prevent_armor_change enchantment effects.
func (r *PickupRules) PreventsArmorChange(stack inventory.ItemStack) bool {
	if r == nil {
		return stack.EnchantLevel(ench.EnchantmentIDByName("minecraft:binding_curse")) > 0
	}
	for id, level := range stack.Enchantments {
		if level > 0 && r.binding[id] {
			return true
		}
	}
	return false
}

// compilePickupRules validates used item tags and armor-change effects before publication.
func compilePickupRules(snapshot *datapack.Snapshot) (*PickupRules, error) {
	rules := &PickupRules{itemTags: map[string]map[int32]bool{}, binding: map[int32]bool{}}
	for _, tag := range [...]string{"skeleton_preferred_weapons", "drowned_preferred_weapons", "wither_skeleton_disliked_weapons"} {
		path := "data/minecraft/tags/item/" + tag + ".json"
		if _, present := snapshot.Get(path); !present {
			continue
		}
		members, err := snapshot.ResolveTag("item", "minecraft:"+tag)
		if err != nil {
			return nil, err
		}
		rules.itemTags[tag] = make(map[int32]bool, len(members))
		for _, member := range members {
			id := item.IDByName(member)
			if id < 0 {
				return nil, fmt.Errorf("equipment item tag %s: unknown item %s", tag, member)
			}
			rules.itemTags[tag][id] = true
		}
	}
	for _, name := range snapshot.Names() {
		parts := strings.SplitN(name, "/", 3)
		if len(parts) != 3 || parts[0] != "data" || !strings.HasPrefix(parts[2], "enchantment/") || !strings.HasSuffix(name, ".json") {
			continue
		}
		raw, _ := snapshot.Get(name)
		var definition struct {
			Effects map[string]json.RawMessage `json:"effects"`
		}
		if err := json.Unmarshal(raw, &definition); err != nil {
			return nil, fmt.Errorf("equipment enchantment %s: %w", name, err)
		}
		if effect, present := definition.Effects["minecraft:prevent_armor_change"]; present {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(effect, &fields); err != nil || fields == nil || len(fields) != 0 {
				return nil, fmt.Errorf("equipment enchantment %s: invalid prevent_armor_change effect", name)
			}
			id := ench.EnchantmentIDByName(parts[1] + ":" + strings.TrimSuffix(strings.TrimPrefix(parts[2], "enchantment/"), ".json"))
			if id < 0 {
				return nil, fmt.Errorf("equipment enchantment %s: unregistered enchantment", name)
			}
			rules.binding[id] = true
		}
	}
	return rules, nil
}
