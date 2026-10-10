// Package loot implements Minecraft loot table evaluation.
package loot

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"

	"github.com/vitismc/vitis/internal/inventory"
)

type TableType string

const (
	TableTypeBlock  TableType = "minecraft:block"
	TableTypeEntity TableType = "minecraft:entity"
	TableTypeChest  TableType = "minecraft:chest"
)

type Table struct {
	Type           TableType  `json:"type"`
	RandomSequence string     `json:"random_sequence,omitempty"`
	Pools          []Pool     `json:"pools,omitempty"`
	Functions      []Function `json:"functions,omitempty"`
}

type Pool struct {
	Rolls         NumberProvider `json:"rolls"`
	BonusRolls    float64        `json:"bonus_rolls"`
	Entries       []Entry        `json:"entries"`
	Conditions    []Condition    `json:"conditions,omitempty"`
	Functions     []Function     `json:"functions,omitempty"`
	bonusProvider *NumberProvider
}

// UnmarshalJSON accepts number-provider bonuses without changing the legacy numeric field.
func (p *Pool) UnmarshalJSON(data []byte) error {
	type definition Pool
	var raw struct {
		*definition
		Bonus json.RawMessage `json:"bonus_rolls"`
	}
	value := definition{}
	raw.definition = &value
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*p = Pool(value)
	if len(raw.Bonus) == 0 {
		return nil
	}
	var numeric float64
	if err := json.Unmarshal(raw.Bonus, &numeric); err == nil && string(raw.Bonus) != "null" {
		p.BonusRolls = numeric
	}
	p.bonusProvider = &NumberProvider{}
	return json.Unmarshal(raw.Bonus, p.bonusProvider)
}

type Entry struct {
	Type            string         `json:"type"`
	Name            string         `json:"name,omitempty"`
	Value           LootTableValue `json:"value,omitempty"`
	Expand          bool           `json:"expand,omitempty"`
	Children        []Entry        `json:"children,omitempty"`
	Conditions      []Condition    `json:"conditions,omitempty"`
	Functions       []Function     `json:"functions,omitempty"`
	Weight          int            `json:"weight,omitempty"`
	Quality         int            `json:"quality,omitempty"`
	weightSpecified bool
}

// UnmarshalJSON distinguishes an explicit zero weight from the default weight.
func (e *Entry) UnmarshalJSON(data []byte) error {
	type definition Entry
	value := definition{Weight: 1}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*e = Entry(value)
	e.weightSpecified = true
	return nil
}

// LootTableValue references a named table or embeds an inline table.
type LootTableValue struct {
	ID     string
	Inline *Table
}

// UnmarshalJSON decodes a named or inline loot table value.
func (v *LootTableValue) UnmarshalJSON(data []byte) error {
	var id string
	if err := json.Unmarshal(data, &id); err == nil {
		v.ID = id
		v.Inline = nil
		return nil
	}
	var table Table
	if err := json.Unmarshal(data, &table); err != nil {
		return err
	}
	v.ID = ""
	v.Inline = &table
	return nil
}

type Condition struct {
	Condition         string  `json:"condition"`
	Chance            float64 `json:"chance,omitempty"`
	Entity            string  `json:"entity,omitempty"`
	UnenchantedChance float64 `json:"unenchanted_chance,omitempty"`
	EnchantedChance   struct {
		Type     string  `json:"type"`
		Base     float64 `json:"base"`
		PerLevel float64 `json:"per_level_above_first"`
	} `json:"enchanted_chance,omitempty"`
	Enchantment string          `json:"enchantment,omitempty"`
	Chances     []float64       `json:"chances,omitempty"`
	Predicate   json.RawMessage `json:"predicate,omitempty"`
	Terms       []Condition     `json:"terms,omitempty"`
	Term        *Condition      `json:"term,omitempty"`
	Block       string          `json:"block,omitempty"`
	Properties  json.RawMessage `json:"properties,omitempty"`
}

type Function struct {
	Function       string                     `json:"function"`
	Enchantment    string                     `json:"enchantment,omitempty"`
	Formula        string                     `json:"formula,omitempty"`
	Parameters     *BonusParameters           `json:"parameters,omitempty"`
	Count          *NumberProvider            `json:"count,omitempty"`
	Add            bool                       `json:"add,omitempty"`
	Limit          *NumberProviderRange       `json:"limit,omitempty"`
	Conditions     []Condition                `json:"conditions,omitempty"`
	Amplifier      *NumberProvider            `json:"amplifier,omitempty"`
	ID             string                     `json:"id,omitempty"`
	Components     map[string]json.RawMessage `json:"components,omitempty"`
	componentPatch inventory.ItemStack
}

type BonusParameters struct {
	BonusMultiplier float64 `json:"bonusMultiplier,omitempty"`
	Extra           int     `json:"extra,omitempty"`
	Probability     float64 `json:"probability,omitempty"`
}

type NumberProvider struct {
	value        float64
	provType     string
	lower, upper *NumberProvider
}

type NumberProviderRange struct {
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

// UnmarshalJSON decodes either an exact numeric limit or a min/max range.
func (r *NumberProviderRange) UnmarshalJSON(data []byte) error {
	var exact float64
	if err := json.Unmarshal(data, &exact); err == nil {
		r.Min = &exact
		r.Max = &exact
		return nil
	}
	type rangeValue NumberProviderRange
	var value rangeValue
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*r = NumberProviderRange(value)
	return nil
}

// UnmarshalJSON validates constant, uniform and binomial providers recursively.
func (n *NumberProvider) UnmarshalJSON(data []byte) error {
	return n.unmarshal(data, 0)
}

// unmarshal bounds provider nesting and rejects unknown or malformed distributions.
func (n *NumberProvider) unmarshal(data []byte, depth int) error {
	if depth >= 16 || string(data) == "null" {
		return fmt.Errorf("invalid loot number provider")
	}
	var f float64
	if err := json.Unmarshal(data, &f); err == nil {
		if math.IsNaN(f) || math.IsInf(f, 0) || f < math.MinInt32 || f > math.MaxInt32 {
			return fmt.Errorf("loot number is out of range")
		}
		*n = NumberProvider{}
		n.value = f
		return nil
	}

	var obj struct {
		Type  string          `json:"type"`
		Value json.RawMessage `json:"value"`
		Min   json.RawMessage `json:"min"`
		Max   json.RawMessage `json:"max"`
		N     json.RawMessage `json:"n"`
		P     json.RawMessage `json:"p"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	*n = NumberProvider{provType: obj.Type}
	if obj.Type == "minecraft:constant" {
		if err := n.unmarshal(obj.Value, depth+1); err != nil {
			return err
		}
		if n.provType != "" {
			return fmt.Errorf("constant loot value must be numeric")
		}
		return nil
	}
	var lower, upper json.RawMessage
	switch obj.Type {
	case "minecraft:uniform":
		lower, upper = obj.Min, obj.Max
	case "minecraft:binomial":
		lower, upper = obj.N, obj.P
	default:
		return fmt.Errorf("unsupported loot number provider %q", obj.Type)
	}
	n.lower, n.upper = &NumberProvider{}, &NumberProvider{}
	if err := n.lower.unmarshal(lower, depth+1); err != nil {
		return err
	}
	if err := n.upper.unmarshal(upper, depth+1); err != nil {
		return err
	}
	if obj.Type == "minecraft:binomial" && n.lower.maximum() > 65536 {
		return fmt.Errorf("binomial loot trials exceed limit")
	}
	return nil
}

// maximum bounds trial counts without sampling the provider.
func (n *NumberProvider) maximum() float64 {
	switch n.provType {
	case "minecraft:uniform":
		return max(n.lower.maximum(), n.upper.maximum())
	case "minecraft:binomial":
		return max(0, n.lower.maximum())
	default:
		return n.value
	}
}

// Roll samples integer counts, flooring constants and uniform bounds.
func (n *NumberProvider) Roll(rng *rand.Rand) int {
	switch n.provType {
	case "minecraft:uniform":
		lower, upper := n.lower.Roll(rng), n.upper.Roll(rng)
		if upper <= lower {
			return lower
		}
		return lower + int(rng.Int63n(int64(upper)-int64(lower)+1))
	case "minecraft:binomial":
		trials, probability := n.lower.Roll(rng), n.upper.RollFloat(rng)
		count := 0
		for range max(0, trials) {
			if rng.Float64() < probability {
				count++
			}
		}
		return count
	default:
		return int(math.Floor(n.value))
	}
}

// RollFloat samples a provider without discarding fractional loot bonuses.
func (n *NumberProvider) RollFloat(rng *rand.Rand) float64 {
	switch n.provType {
	case "minecraft:uniform":
		lower, upper := n.lower.RollFloat(rng), n.upper.RollFloat(rng)
		if upper <= lower {
			return lower
		}
		return lower + rng.Float64()*(upper-lower)
	case "minecraft:binomial":
		return float64(n.Roll(rng))
	default:
		return n.value
	}
}

type EntryType string

const (
	EntryTypeItem         EntryType = "minecraft:item"
	EntryTypeAlternatives EntryType = "minecraft:alternatives"
	EntryTypeSequence     EntryType = "minecraft:sequence"
	EntryTypeGroup        EntryType = "minecraft:group"
	EntryTypeEmpty        EntryType = "minecraft:empty"
	EntryTypeLootTable    EntryType = "minecraft:loot_table"
	EntryTypeTag          EntryType = "minecraft:tag"
	EntryTypeDynamic      EntryType = "minecraft:dynamic"
)

type ConditionType string

const (
	ConditionSurvivesExplosion ConditionType = "minecraft:survives_explosion"
	ConditionMatchTool         ConditionType = "minecraft:match_tool"
	ConditionTableBonus        ConditionType = "minecraft:table_bonus"
	ConditionBlockStateProps   ConditionType = "minecraft:block_state_property"
	ConditionInverted          ConditionType = "minecraft:inverted"
	ConditionAnyOf             ConditionType = "minecraft:any_of"
	ConditionAllOf             ConditionType = "minecraft:all_of"
	ConditionRandomChance      ConditionType = "minecraft:random_chance"
)

type FunctionType string

const (
	FunctionSetCount       FunctionType = "minecraft:set_count"
	FunctionApplyBonus     FunctionType = "minecraft:apply_bonus"
	FunctionExplosionDecay FunctionType = "minecraft:explosion_decay"
	FunctionLimitCount     FunctionType = "minecraft:limit_count"
	FunctionFurnaceSmelt   FunctionType = "minecraft:furnace_smelt"
	FunctionCopyState      FunctionType = "minecraft:copy_state"
	FunctionCopyNBT        FunctionType = "minecraft:copy_nbt"
)

type FormulaType string

const (
	FormulaOreDrops               FormulaType = "minecraft:ore_drops"
	FormulaBinomialWithBonusCount FormulaType = "minecraft:binomial_with_bonus_count"
	FormulaUniformBonusCount      FormulaType = "minecraft:uniform_bonus_count"
)
