package loot

import (
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestToolPredicatesRejectMismatchedItemsAndUnsupportedFields protects conditional rewards.
func TestToolPredicatesRejectMismatchedItemsAndUnsupportedFields(t *testing.T) {
	ctx := NewContext(rand.New(rand.NewSource(1))).WithTool("minecraft:diamond_sword", map[string]int{"minecraft:looting": 3})
	for _, test := range []struct {
		predicate string
		want      bool
	}{
		{`{"items":"minecraft:diamond_sword"}`, true},
		{`{"items":["minecraft:iron_sword","minecraft:diamond_sword"]}`, true},
		{`{"items":"minecraft:iron_sword"}`, false},
		{`{"items":"minecraft:diamond_sword","components":{"minecraft:custom_name":"named"}}`, false},
		{`{"predicates":{"minecraft:enchantments":[{"enchantments":"minecraft:looting","levels":{"min":1,"max":2}}]}}`, false},
	} {
		condition := Condition{Condition: string(ConditionMatchTool), Predicate: json.RawMessage(test.predicate)}
		if actual := evaluateCondition(&condition, ctx); actual != test.want {
			t.Fatalf("predicate %s: actual=%v want=%v", test.predicate, actual, test.want)
		}
	}
	if matchesEntity(json.RawMessage(`{}`), nil, ctx) {
		t.Fatal("missing attacker satisfied an entity predicate")
	}
}

// entityLootRuntime loads the real entity tables and their referenced tags and recipes.
func entityLootRuntime(t *testing.T) *Runtime {
	t.Helper()
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{"../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	return runtime
}

// TestEntityLootUsesDeathFlagsAndLooting verifies cooked drops and enchantment bonuses.
func TestEntityLootUsesDeathFlagsAndLooting(t *testing.T) {
	runtime := entityLootRuntime(t)
	roll := func(level int, onFire bool) (int32, map[string]int) {
		ctx := NewContext(rand.New(rand.NewSource(1))).WithTool("minecraft:diamond_sword", map[string]int{"minecraft:looting": level})
		ctx.Entity = &EntityInfo{Type: "minecraft:cow", OnFire: onFire}
		ctx.KilledByPlayer = true
		counts := map[string]int{}
		var total int32
		for range 400 {
			for _, stack := range runtime.RollStacks("minecraft:entities/cow", ctx) {
				counts[item.NameByID(stack.ItemID)]++
				total += stack.ItemCount
			}
		}
		return total, counts
	}
	plain, raw := roll(0, false)
	enchanted, cooked := roll(3, true)
	if raw["minecraft:beef"] == 0 || cooked["minecraft:cooked_beef"] == 0 || cooked["minecraft:beef"] != 0 || enchanted <= plain {
		t.Fatalf("raw=%v, cooked=%v, totals=%d/%d", raw, cooked, plain, enchanted)
	}
}

// TestEntityPredicatesFailClosedAndRespectSavedTraits prevents unconditional special drops.
func TestEntityPredicatesFailClosedAndRespectSavedTraits(t *testing.T) {
	runtime := entityLootRuntime(t)
	ctx := NewContext(rand.New(rand.NewSource(1)))
	ctx.Entity = &EntityInfo{Type: "minecraft:sheep", Color: "black"}
	counts := map[string]int{}
	for range 50 {
		for _, stack := range runtime.RollStacks("minecraft:entities/sheep", ctx) {
			counts[item.NameByID(stack.ItemID)]++
		}
	}
	if counts["minecraft:black_wool"] != 50 || counts["minecraft:white_wool"] != 0 {
		t.Fatalf("wrong sheep drops: %v", counts)
	}
	ctx.Entity.Sheared = true
	for _, stack := range runtime.RollStacks("minecraft:entities/sheep", ctx) {
		if item.NameByID(stack.ItemID) == "minecraft:black_wool" {
			t.Fatal("sheared sheep dropped wool")
		}
	}
	condition := Condition{Condition: "minecraft:entity_properties", Entity: "this", Predicate: []byte(`{"location":{"dimension":"minecraft:overworld"}}`)}
	if evaluateCondition(&condition, ctx) {
		t.Fatal("unsupported constraint evaluated as true")
	}
	ctx.Entity = &EntityInfo{Type: "minecraft:creeper"}
	ctx.Attacker = &EntityInfo{Type: "minecraft:skeleton"}
	foundDisc := false
	for _, stack := range runtime.RollStacks("minecraft:entities/creeper", ctx) {
		if item.NameByID(stack.ItemID) != "minecraft:gunpowder" {
			foundDisc = true
		}
	}
	if !foundDisc {
		t.Fatal("skeleton kill did not resolve the music-disc tag")
	}
}

// TestFullStackLootPreservesPotionAndBottleComponents verifies wire and persistent round trips.
func TestFullStackLootPreservesPotionAndBottleComponents(t *testing.T) {
	pack := lootTestPack(t)
	writeLootResource(t, pack, "data/test/loot_table/reward.json", `{"type":"minecraft:entity","pools":[{"rolls":1,"entries":[{"type":"minecraft:item","name":"minecraft:potion","functions":[{"function":"minecraft:set_potion","id":"minecraft:poison"}]}]},{"rolls":1,"entries":[{"type":"minecraft:item","name":"minecraft:ominous_bottle","functions":[{"function":"minecraft:set_ominous_bottle_amplifier","amplifier":3}]}]}]}`)
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{pack}); err != nil {
		t.Fatal(err)
	}
	stacks := runtime.RollStacks("test:reward", NewContext(rand.New(rand.NewSource(1))))
	if len(stacks) != 2 {
		t.Fatalf("stacks = %v", stacks)
	}
	for _, stack := range stacks {
		if len(stack.ComponentPatch.Added) == 0 {
			t.Fatal("loot function discarded components")
		}
		buffer := protocol.NewBuffer(128)
		if err := inventory.EncodeItemStack(buffer, stack); err != nil {
			t.Fatal(err)
		}
		decoded, err := inventory.DecodeItemStack(protocol.WrapBuffer(buffer.Bytes()))
		if err != nil || !decoded.StackableWith(stack) {
			t.Fatalf("wire stack: %v", err)
		}
		raw, err := inventory.ItemStackToNBT(stack)
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := inventory.ItemStackFromNBT(raw)
		if err != nil || !loaded.StackableWith(stack) {
			t.Fatalf("persistent stack: %v", err)
		}
	}
}
