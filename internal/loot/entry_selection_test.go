package loot

import (
	"encoding/json"
	"math/rand"
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
)

// selectionTable decodes a pool using the same entry defaults as resource compilation.
func selectionTable(t *testing.T, entries string) *Table {
	t.Helper()
	var table Table
	if err := json.Unmarshal([]byte(`{"pools":[{"rolls":1,"entries":`+entries+`}]}`), &table); err != nil {
		t.Fatal(err)
	}
	return &table
}

// TestCompositeLootSelectsOneWeightedLeaf prevents group and sequence overproduction.
func TestCompositeLootSelectsOneWeightedLeaf(t *testing.T) {
	for _, kind := range []string{"group", "sequence"} {
		t.Run(kind, func(t *testing.T) {
			table := selectionTable(t, `[{"type":"minecraft:`+kind+`","children":[{"type":"minecraft:item","name":"minecraft:stone","weight":3},{"type":"minecraft:item","name":"minecraft:diamond","weight":1}]}]`)
			ctx := NewContext(rand.New(rand.NewSource(9)))
			counts := map[string]int{}
			for range 2000 {
				drops := table.GetDrops(ctx)
				if len(drops) != 1 || drops[0].Count != 1 {
					t.Fatalf("one roll yielded %v", drops)
				}
				counts[drops[0].ItemID]++
			}
			if counts["minecraft:stone"] < 1400 || counts["minecraft:stone"] > 1600 {
				t.Fatalf("incorrect leaf weights: %v", counts)
			}
		})
	}
}

// TestCompositeExpansionControlsAlternativesAndSequence verifies nested failure and empty success.
func TestCompositeExpansionControlsAlternativesAndSequence(t *testing.T) {
	for _, test := range []struct{ name, entries, want string }{
		{"nested alternative failure", `[{"type":"minecraft:alternatives","children":[{"type":"minecraft:alternatives","children":[{"type":"minecraft:item","name":"minecraft:stone","conditions":[{"condition":"minecraft:random_chance","chance":0}]}]},{"type":"minecraft:item","name":"minecraft:diamond"}]}]`, "minecraft:diamond"},
		{"zero weight successful alternative", `[{"type":"minecraft:alternatives","children":[{"type":"minecraft:item","name":"minecraft:stone","weight":0},{"type":"minecraft:item","name":"minecraft:diamond"}]}]`, ""},
		{"sequence retains preceding leaves", `[{"type":"minecraft:sequence","children":[{"type":"minecraft:item","name":"minecraft:stone"},{"type":"minecraft:item","name":"minecraft:diamond","conditions":[{"condition":"minecraft:random_chance","chance":0}]},{"type":"minecraft:item","name":"minecraft:gold_ingot"}]}]`, "minecraft:stone"},
		{"empty singleton stops alternative", `[{"type":"minecraft:alternatives","children":[{"type":"minecraft:empty"},{"type":"minecraft:item","name":"minecraft:diamond"}]}]`, ""},
		{"empty any_of fails", `[{"type":"minecraft:item","name":"minecraft:diamond","conditions":[{"condition":"minecraft:any_of","terms":[]}]}]`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			drops := selectionTable(t, test.entries).GetDrops(NewContext(rand.New(rand.NewSource(1))))
			if test.want == "" {
				if len(drops) != 0 {
					t.Fatalf("unexpected drops: %v", drops)
				}
				return
			}
			if len(drops) != 1 || drops[0].ItemID != test.want {
				t.Fatalf("drops=%v want=%s", drops, test.want)
			}
		})
	}
}

// TestExpandedTagsParticipatePerItemInPoolWeights verifies tag expansion rather than nested choice.
func TestExpandedTagsParticipatePerItemInPoolWeights(t *testing.T) {
	table := selectionTable(t, `[{"type":"minecraft:tag","name":"test:choices","expand":true,"functions":[{"function":"minecraft:set_count","count":2}]},{"type":"minecraft:item","name":"minecraft:diamond"}]`)
	ctx := NewContext(rand.New(rand.NewSource(91)))
	ctx.tagResolver = func(string) []string { return []string{"minecraft:stone", "minecraft:dirt"} }
	counts := map[string]int{}
	for range 6000 {
		drops := table.GetDrops(ctx)
		if len(drops) != 1 {
			t.Fatalf("drops=%v", drops)
		}
		want := 1
		if drops[0].ItemID != "minecraft:diamond" {
			want = 2
		}
		if drops[0].Count != want {
			t.Fatal("expanded tag lost its functions")
		}
		counts[drops[0].ItemID]++
	}
	for name, count := range counts {
		if count < 1800 || count > 2200 {
			t.Fatalf("tag member weight %s=%d", name, count)
		}
	}
	if len(counts) != 3 {
		t.Fatalf("missing choices: %v", counts)
	}
}

// TestLootWeightRoundsAfterApplyingLuck covers negative fractional quality and explicit zero.
func TestLootWeightRoundsAfterApplyingLuck(t *testing.T) {
	entry := Entry{Weight: 1, Quality: -1}
	if entryWeight(&entry, .5) != 0 {
		t.Fatal("negative fractional quality rounded toward zero")
	}
	if entryWeight(&Entry{Weight: 2, Quality: 1}, .9) != 2 {
		t.Fatal("fractional luck rounded up")
	}
}

// TestLootFunctionsComposeAcrossTablesAndPreserveComponents verifies nested rewards and stack splitting.
func TestLootFunctionsComposeAcrossTablesAndPreserveComponents(t *testing.T) {
	pack := lootTestPack(t)
	writeLootResource(t, pack, "data/test/loot_table/child.json", `{"type":"minecraft:entity","pools":[{"rolls":1,"entries":[{"type":"minecraft:item","name":"minecraft:diamond_sword","functions":[{"function":"minecraft:set_components","components":{"minecraft:damage":5,"minecraft:max_stack_size":16,"!minecraft:enchantable":{}}}]}]}],"functions":[{"function":"minecraft:set_count","count":2}]}`)
	writeLootResource(t, pack, "data/test/loot_table/root.json", `{"type":"minecraft:entity","pools":[{"rolls":1,"entries":[{"type":"minecraft:loot_table","value":"test:child","functions":[{"function":"minecraft:set_count","count":2,"add":true}]}],"functions":[{"function":"minecraft:set_count","count":3,"add":true}]}],"functions":[{"function":"minecraft:set_count","count":30,"add":true},{"function":"minecraft:set_components","components":{"minecraft:damage":7}}]}`)
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{pack}); err != nil {
		t.Fatal(err)
	}
	stacks := runtime.RollStacks("test:root", NewContext(rand.New(rand.NewSource(1))))
	if len(stacks) != 3 || stacks[0].ItemCount != 16 || stacks[1].ItemCount != 16 || stacks[2].ItemCount != 5 {
		t.Fatalf("function order or splitting incorrect: %+v", stacks)
	}
	for _, stack := range stacks {
		if stack.ItemID != item.IDByName("minecraft:diamond_sword") || stack.Damage != 7 || stack.MaxStackSize() != 16 {
			t.Fatalf("components lost: %+v", stack)
		}
		if _, present := stack.Component(dc.ComponentEnchantable); present {
			t.Fatal("removed component restored")
		}
		raw, err := inventory.ItemStackToNBT(stack)
		if err != nil {
			t.Fatal(err)
		}
		restored, err := inventory.ItemStackFromNBT(raw)
		if err != nil || !restored.StackableWith(stack) {
			t.Fatalf("component persistence: %v", err)
		}
	}
	stacks[0].SetDamage(99)
	if stacks[1].Damage != 7 || runtime.RollStacks("test:root", NewContext(rand.New(rand.NewSource(1))))[0].Damage != 7 {
		t.Fatal("reward shares mutable component state")
	}
	drops := runtime.Roll("test:root", NewContext(rand.New(rand.NewSource(1))))
	if len(drops) != 1 || drops[0].Count != 37 || drops[0].Stack.ItemID != 0 {
		t.Fatalf("legacy adapter=%+v", drops)
	}
	previous := resources.Current()
	writeLootResource(t, pack, "data/test/loot_table/root.json", `{"type":"minecraft:entity","functions":[{"function":"minecraft:set_components","components":{"minecraft:damage":"invalid"}}]}`)
	if err := resources.Reload([]string{pack}); err == nil || resources.Current() != previous {
		t.Fatal("invalid component patch replaced active snapshot")
	}
}

// TestEnchantedCountIncreaseAppliesLimitOnlyWithEnchantment verifies Looting caps.
func TestEnchantedCountIncreaseAppliesLimitOnlyWithEnchantment(t *testing.T) {
	var fn Function
	if err := json.Unmarshal([]byte(`{"function":"minecraft:enchanted_count_increase","enchantment":"minecraft:looting","count":2,"limit":5}`), &fn); err != nil {
		t.Fatal(err)
	}
	ctx := NewContext(rand.New(rand.NewSource(1))).WithTool("minecraft:diamond_sword", map[string]int{"minecraft:looting": 3})
	if count := applyFunction(&fn, 3, ctx); count != 5 {
		t.Fatalf("enchanted count=%d", count)
	}
	ctx.Tool.Enchantments = nil
	if count := applyFunction(&fn, 6, ctx); count != 6 {
		t.Fatalf("unenchanted count=%d", count)
	}
}
