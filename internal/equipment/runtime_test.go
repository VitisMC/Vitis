package equipment

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	ench "github.com/vitismc/vitis/internal/data/generated/enchantment"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
)

// writeDamageTag writes one test pack resource without publishing a snapshot.
func writeDamageTag(t *testing.T, directory, name, contents string) {
	t.Helper()
	path := filepath.Join(directory, "data", "example", "tags", "damage_type", name+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "pack.mcmeta"), []byte(`{"pack":{"pack_format":61,"description":"equipment test"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestEquipmentDamageTagsReloadAtomically checks nested references, rollback and concurrent reads.
func TestEquipmentDamageTagsReloadAtomically(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	writeDamageTag(t, directory, "attacks", `{"values":["minecraft:mob_attack"]}`)
	writeDamageTag(t, directory, "resistant", `{"values":["#example:attacks","minecraft:arrow"]}`)
	if err := manager.Reload([]string{directory}); err != nil {
		t.Fatal(err)
	}
	stack := inventory.NewSlot(item.IDByName("minecraft:diamond_chestplate"), 1)
	stack.SetComponent(dc.ComponentDamageResistant, inventory.StructuredComponent{Data: "example:resistant"})
	if !runtime.ResistsDamage(stack, "minecraft:mob_attack") || !runtime.ResistsDamage(stack, "minecraft:arrow") || runtime.ResistsDamage(stack, "minecraft:cactus") {
		t.Fatal("compiled damage resistance ignored tag membership")
	}
	previous := manager.Current()
	for _, invalid := range []string{`{"values":["minecraft:missing_type"]}`, `{"values":["#example:resistant"]}`, `{"values":["#example:missing"]}`} {
		writeDamageTag(t, directory, "attacks", invalid)
		if err := manager.Reload([]string{directory}); err == nil || manager.Current() != previous || !runtime.ResistsDamage(stack, "minecraft:arrow") {
			t.Fatal("failed reload replaced equipment definitions")
		}
	}
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		for range 1000 {
			runtime.ResistsDamage(stack, "minecraft:mob_attack")
		}
	}()
	writeDamageTag(t, directory, "attacks", `{"values":["minecraft:cactus"]}`)
	if err := manager.Reload([]string{directory}); err != nil {
		t.Fatal(err)
	}
	wait.Wait()
	if runtime.ResistsDamage(stack, "minecraft:mob_attack") || !runtime.ResistsDamage(stack, "minecraft:cactus") {
		t.Fatal("successful reload retained old equipment rules")
	}
	stack.RemoveComponent(dc.ComponentDamageResistant)
	if runtime.ResistsDamage(stack, "minecraft:arrow") {
		t.Fatal("removed resistance component remained active")
	}
}

// TestVanillaEquipmentDamageTagsCompile checks all packaged damage definitions and default fire resistance.
func TestVanillaEquipmentDamageTagsCompile(t *testing.T) {
	if _, err := os.Stat("../../.mcdata/1.21.4/vanilla/pack.mcmeta"); os.IsNotExist(err) {
		t.Skip("pinned vanilla resources are not installed")
	} else if err != nil {
		t.Fatal(err)
	}
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	pickup := runtime.PickupRules()
	helmet := inventory.NewSlot(item.IDByName("minecraft:iron_helmet"), 1)
	helmet.SetEnchant(ench.EnchantmentIDByName("minecraft:binding_curse"), 1)
	if !pickup.PreventsArmorChange(helmet) || !pickup.IsPreferredWeapon("stray", inventory.NewSlot(item.IDByName("minecraft:bow"), 1)) || !pickup.IsPreferredWeapon("drowned", inventory.NewSlot(item.IDByName("minecraft:trident"), 1)) || pickup.CanHoldItem("wither_skeleton", inventory.NewSlot(item.IDByName("minecraft:crossbow"), 1)) {
		t.Fatal("native pickup tags or armor-change effects lost their rules")
	}
	stack := inventory.NewSlot(item.IDByName("minecraft:netherite_chestplate"), 1)
	var fallback *Runtime
	for _, rules := range []*Runtime{runtime, fallback} {
		if !rules.ResistsDamage(stack, "minecraft:lava") || rules.ResistsDamage(stack, "minecraft:arrow") {
			t.Fatal("native damage-resistant component lost its fire rule")
		}
	}
	if _, err := NewRuntime(nil); err == nil {
		t.Fatal("nil resource manager accepted")
	}
}
