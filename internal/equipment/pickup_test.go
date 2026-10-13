package equipment

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	ench "github.com/vitismc/vitis/internal/data/generated/enchantment"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
)

// writePickupResource writes a minimal pack with one selection resource.
func writePickupResource(t *testing.T, directory, resource, contents string) {
	t.Helper()
	path := filepath.Join(directory, "data", "minecraft", resource+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "pack.mcmeta"), []byte(`{"pack":{"pack_format":61,"description":"pickup test"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestPickupRulesReloadAtomically verifies snapshot ownership, resource overrides and rollback.
func TestPickupRulesReloadAtomically(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	writePickupResource(t, directory, "tags/item/skeleton_preferred_weapons", `{"values":["minecraft:bow"]}`)
	writePickupResource(t, directory, "tags/item/wither_skeleton_disliked_weapons", `{"values":["minecraft:bow","minecraft:crossbow"]}`)
	writePickupResource(t, directory, "enchantment/binding_curse", `{"effects":{"minecraft:prevent_armor_change":{}}}`)
	if err := manager.Reload([]string{directory}); err != nil {
		t.Fatal(err)
	}
	bow := inventory.NewSlot(item.IDByName("minecraft:bow"), 1)
	sword := inventory.NewSlot(item.IDByName("minecraft:diamond_sword"), 1)
	helmet := inventory.NewSlot(item.IDByName("minecraft:iron_helmet"), 1)
	helmet.SetEnchant(ench.EnchantmentIDByName("minecraft:binding_curse"), 1)
	previous, acquired := manager.Current(), runtime.PickupRules()
	if !acquired.IsPreferredWeapon("stray", bow) || acquired.IsPreferredWeapon("stray", sword) || acquired.CanHoldItem("wither_skeleton", bow) || !acquired.PreventsArmorChange(helmet) {
		t.Fatal("selection ignored compiled tags or effects")
	}
	for _, invalid := range []string{`{"values":["minecraft:missing_item"]}`, `{"values":["#minecraft:skeleton_preferred_weapons"]}`, `{"values":["#minecraft:missing"]}`} {
		writePickupResource(t, directory, "tags/item/skeleton_preferred_weapons", invalid)
		if err := manager.Reload([]string{directory}); err == nil || manager.Current() != previous || !runtime.PickupRules().IsPreferredWeapon("skeleton", bow) {
			t.Fatal("invalid tag replaced the published pickup snapshot")
		}
	}
	writePickupResource(t, directory, "tags/item/skeleton_preferred_weapons", `{"values":["minecraft:diamond_sword"]}`)
	writePickupResource(t, directory, "enchantment/binding_curse", `{"effects":{"minecraft:prevent_armor_change":{"unexpected":true}}}`)
	if err := manager.Reload([]string{directory}); err == nil || manager.Current() != previous {
		t.Fatal("invalid armor-change effect was published")
	}
	writePickupResource(t, directory, "enchantment/binding_curse", `{"effects":{}}`)
	writePickupResource(t, directory, "enchantment/sharpness", `{"effects":{"minecraft:prevent_armor_change":{}}}`)
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		for range 1000 {
			rules := runtime.PickupRules()
			rules.IsPreferredWeapon("skeleton", bow)
			rules.PreventsArmorChange(helmet)
		}
	}()
	if err := manager.Reload([]string{directory}); err != nil {
		t.Fatal(err)
	}
	wait.Wait()
	rules := runtime.PickupRules()
	if rules.IsPreferredWeapon("skeleton", bow) || !rules.IsPreferredWeapon("skeleton", sword) || rules.PreventsArmorChange(helmet) || !acquired.IsPreferredWeapon("skeleton", bow) || !acquired.PreventsArmorChange(helmet) {
		t.Fatal("reload mutated an acquired snapshot or retained obsolete definitions")
	}
	helmet.SetEnchant(ench.EnchantmentIDByName("minecraft:sharpness"), 1)
	if !rules.PreventsArmorChange(helmet) {
		t.Fatal("armor-change effect was hardcoded to Binding")
	}
}
