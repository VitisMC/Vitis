package loot

import (
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/item"
)

// TestDirectDamagePredicatesDistinguishMeleeAndIndirectHits covers the remaining damage-source field.
func TestDirectDamagePredicatesDistinguishMeleeAndIndirectHits(t *testing.T) {
	ctx := NewContext(rand.New(rand.NewSource(1)))
	for _, value := range []bool{true, false} {
		ctx.DamageIsDirect = &value
		for _, expected := range []string{"true", "false", "null", "1"} {
			condition := Condition{Condition: "minecraft:damage_source_properties", Predicate: json.RawMessage(`{"is_direct":` + expected + `}`)}
			want := expected == "true" && value || expected == "false" && !value
			if got := evaluateCondition(&condition, ctx); got != want {
				t.Fatalf("direct=%v expected=%s: result=%v", value, expected, got)
			}
		}
	}
	ctx.DamageIsDirect = nil
	if evaluateCondition(&Condition{Condition: "minecraft:damage_source_properties", Predicate: json.RawMessage(`{"is_direct":false}`)}, ctx) {
		t.Fatal("missing source matched an indirect-damage requirement")
	}
	if !matchesDamageTags(json.RawMessage(`[]`), ctx) {
		t.Fatal("empty damage tag requirements imposed a restriction")
	}
	if evaluateCondition(&Condition{Condition: "minecraft:damage_source_properties", Predicate: json.RawMessage(`null`)}, ctx) {
		t.Fatal("null damage predicate evaluated as true")
	}
}

// TestDamageTagsControlEntityRewards checks actual entity tables against the fatal damage type.
func TestDamageTagsControlEntityRewards(t *testing.T) {
	runtime := entityLootRuntime(t)
	for _, damage := range []string{"minecraft:lightning_bolt", "minecraft:arrow", "minecraft:on_fire", ""} {
		ctx := NewContext(rand.New(rand.NewSource(1)))
		ctx.DamageType = damage
		bowl := false
		for _, stack := range runtime.RollStacks("minecraft:entities/turtle", ctx) {
			bowl = bowl || item.NameByID(stack.ItemID) == "minecraft:bowl"
		}
		if bowl != (damage == "minecraft:lightning_bolt") {
			t.Fatalf("damage %q: bowl=%v", damage, bowl)
		}
	}
}

// TestDamageTagPredicatesRejectMissingContextAndMalformedRequirements prevents conditional rewards from becoming unconditional.
func TestDamageTagPredicatesRejectMissingContextAndMalformedRequirements(t *testing.T) {
	ctx := NewContext(rand.New(rand.NewSource(1)))
	ctx.DamageType = "minecraft:arrow"
	ctx.registryTagResolver = func(registry, tag string) []string {
		if registry == "damage_type" && tag == "minecraft:is_projectile" {
			return []string{"minecraft:arrow"}
		}
		return nil
	}
	for _, test := range []struct {
		tags string
		want bool
	}{
		{`[{"id":"minecraft:is_projectile","expected":true}]`, true},
		{`[{"id":"minecraft:is_projectile","expected":false}]`, false},
		{`[{"id":"minecraft:is_projectile","expected":true},{"id":"minecraft:is_lightning","expected":false}]`, true},
		{`[{"id":"minecraft:is_projectile"}]`, false},
		{`[{"id":"#minecraft:is_projectile","expected":true}]`, false},
		{`[{"expected":false}]`, false},
		{`null`, false},
		{`{}`, false},
	} {
		condition := Condition{Condition: "minecraft:damage_source_properties", Predicate: json.RawMessage(`{"tags":` + test.tags + `}`)}
		if got := evaluateCondition(&condition, ctx); got != test.want {
			t.Fatalf("tags %s: got=%v want=%v", test.tags, got, test.want)
		}
	}
	ctx.DamageType = ""
	if matchesDamageTags(json.RawMessage(`[{"id":"minecraft:is_lightning","expected":false}]`), ctx) {
		t.Fatal("missing death cause satisfied an exclusion")
	}
}

// TestDamageTagReloadRetainsPreviousSnapshot rejects missing and malformed references atomically.
func TestDamageTagReloadRetainsPreviousSnapshot(t *testing.T) {
	pack := lootTestPack(t)
	writeLootResource(t, pack, "data/test/tags/damage_type/required.json", `{"values":["minecraft:arrow"]}`)
	writeLootResource(t, pack, "data/test/loot_table/reward.json", `{"type":"minecraft:entity","pools":[{"rolls":1,"conditions":[{"condition":"minecraft:inverted","term":{"condition":"minecraft:damage_source_properties","predicate":{"tags":[{"id":"test:required","expected":false}]}}}],"entries":[{"type":"minecraft:item","name":"minecraft:diamond"}]}]}`)
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{pack}); err != nil {
		t.Fatal(err)
	}
	previous := resources.Current()
	for _, broken := range []string{`{"values":["#test:missing"]}`, `{"values":[42]}`} {
		writeLootResource(t, pack, "data/test/tags/damage_type/required.json", broken)
		if err := resources.Reload([]string{pack}); err == nil || resources.Current() != previous {
			t.Fatal("invalid damage type tag published a new snapshot")
		}
		ctx := NewContext(rand.New(rand.NewSource(1)))
		ctx.DamageType = "minecraft:arrow"
		if got := runtime.RollStacks("test:reward", ctx); len(got) != 1 {
			t.Fatalf("prior rewards changed after failed reload: %v", got)
		}
	}
	writeLootResource(t, pack, "data/test/tags/damage_type/required.json", `{"values":["minecraft:arrow"]}`)
	for _, predicate := range []string{`{"is_direct":null}`, `{"is_direct":1}`, `{"tags":[{"id":"test:required"}]}`, `null`} {
		writeLootResource(t, pack, "data/test/loot_table/reward.json", `{"type":"minecraft:entity","pools":[{"rolls":1,"entries":[{"type":"minecraft:item","name":"minecraft:diamond","conditions":[{"condition":"minecraft:damage_source_properties","predicate":`+predicate+`}]}]}]}`)
		if err := resources.Reload([]string{pack}); err == nil || resources.Current() != previous {
			t.Fatalf("invalid damage predicate published a snapshot: %s", predicate)
		}
	}
}
