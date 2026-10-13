package loot

import (
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/vitismc/vitis/internal/datapack"
)

// TestLootNumberProvidersSampleConstantsUniformAndBinomial verifies count distributions.
func TestLootNumberProvidersSampleConstantsUniformAndBinomial(t *testing.T) {
	for _, test := range []struct {
		raw              string
		minimum, maximum int
		mean             float64
	}{
		{`-1.2`, -2, -2, -2},
		{`{"type":"minecraft:constant","value":3.9}`, 3, 3, 3},
		{`{"type":"minecraft:uniform","min":-1.2,"max":1.8}`, -2, 1, -.5},
		{`{"type":"minecraft:uniform","min":{"type":"minecraft:constant","value":2},"max":4}`, 2, 4, 3},
		{`{"type":"minecraft:binomial","n":3,"p":0}`, 0, 0, 0},
		{`{"type":"minecraft:binomial","n":3,"p":1}`, 3, 3, 3},
		{`{"type":"minecraft:binomial","n":3,"p":0.5}`, 0, 3, 1.5},
	} {
		t.Run(test.raw, func(t *testing.T) {
			var provider NumberProvider
			if err := json.Unmarshal([]byte(test.raw), &provider); err != nil {
				t.Fatal(err)
			}
			rng := rand.New(rand.NewSource(1))
			total := 0
			for range 10000 {
				value := provider.Roll(rng)
				if value < test.minimum || value > test.maximum {
					t.Fatalf("out-of-range sample %d", value)
				}
				total += value
			}
			mean := float64(total) / 10000
			if mean < test.mean-.05 || mean > test.mean+.05 {
				t.Fatalf("mean %f want %f", mean, test.mean)
			}
		})
	}
}

// TestPoolBonusRollsSupportsProvidersAndNumericAPI verifies server-calculated luck bonuses.
func TestPoolBonusRollsSupportsProvidersAndNumericAPI(t *testing.T) {
	for _, bonus := range []string{`2`, `{"type":"minecraft:uniform","min":2,"max":2}`} {
		var table Table
		if err := json.Unmarshal([]byte(`{"pools":[{"rolls":1,"bonus_rolls":`+bonus+`,"entries":[{"type":"minecraft:item","name":"minecraft:stone"}]}]}`), &table); err != nil {
			t.Fatal(err)
		}
		drops := table.GetDrops(NewContext(rand.New(rand.NewSource(1))).WithLuck(1.5))
		if len(drops) != 4 {
			t.Fatalf("bonus %s produced %d rolls", bonus, len(drops))
		}
	}
	table := Table{Pools: []Pool{{Rolls: NumberProvider{value: 1}, BonusRolls: 2, Entries: []Entry{{Type: string(EntryTypeItem), Name: "minecraft:stone"}}}}}
	if drops := table.GetDrops(NewContext(rand.New(rand.NewSource(1))).WithLuck(1.5)); len(drops) != 4 {
		t.Fatal("numeric Go API bonus changed")
	}
}

// TestMalformedNumberProviderReloadPreservesTables verifies fail-closed resource validation.
func TestMalformedNumberProviderReloadPreservesTables(t *testing.T) {
	root := lootTestPack(t)
	path := "data/test/loot_table/reward.json"
	writeLootResource(t, root, path, `{"type":"minecraft:entity","pools":[{"rolls":1,"entries":[{"type":"minecraft:item","name":"minecraft:stone"}]}]}`)
	resources := &datapack.Manager{}
	if _, err := NewRuntime(resources); err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{root}); err != nil {
		t.Fatal(err)
	}
	previous := resources.Current()
	for _, raw := range []string{`null`, `1e309`, `{"type":"test:unknown"}`, `{"type":"minecraft:constant"}`, `{"type":"minecraft:uniform","min":1}`, `{"type":"minecraft:binomial","n":65537,"p":0.5}`} {
		writeLootResource(t, root, path, `{"type":"minecraft:entity","pools":[{"rolls":`+raw+`,"entries":[{"type":"minecraft:item","name":"minecraft:stone"}]}]}`)
		if err := resources.Reload([]string{root}); err == nil || resources.Current() != previous {
			t.Fatalf("invalid provider changed snapshot: %s", raw)
		}
	}
}

// TestVanillaSeedLootUsesBaseBinomialBonusWithoutFortune checks connected resource evaluation.
func TestVanillaSeedLootUsesBaseBinomialBonusWithoutFortune(t *testing.T) {
	runtime := entityLootRuntime(t)
	ctx := NewContext(rand.New(rand.NewSource(13))).WithBlockState(map[string]string{"age": "7"})
	stems, wheat := 0, 0
	for range 1000 {
		for _, drop := range runtime.Roll("minecraft:blocks/attached_melon_stem", ctx) {
			stems += drop.Count
		}
		for _, drop := range runtime.Roll("minecraft:blocks/wheat", ctx) {
			if drop.ItemID == "minecraft:wheat_seeds" {
				wheat += drop.Count
			}
		}
	}
	if stems < 1450 || stems > 1750 {
		t.Fatalf("stem seeds=%d", stems)
	}
	if wheat < 2500 || wheat > 2900 {
		t.Fatalf("mature wheat seed bonus=%d", wheat)
	}
}

// TestEntryConditionsRunOnceBeforeSelection protects random predicate probabilities.
func TestEntryConditionsRunOnceBeforeSelection(t *testing.T) {
	table := selectionTable(t, `[{"type":"minecraft:alternatives","children":[{"type":"minecraft:item","name":"minecraft:diamond","conditions":[{"condition":"minecraft:random_chance","chance":0.5}]},{"type":"minecraft:item","name":"minecraft:stone"}]}]`)
	ctx := NewContext(rand.New(rand.NewSource(14)))
	diamonds := 0
	for range 10000 {
		drops := table.GetDrops(ctx)
		if len(drops) != 1 {
			t.Fatal("selected entry condition was reevaluated")
		}
		if drops[0].ItemID == "minecraft:diamond" {
			diamonds++
		}
	}
	if diamonds < 4800 || diamonds > 5200 {
		t.Fatalf("diamond probability changed: %d", diamonds)
	}
}
