// Package loot implements Minecraft loot table evaluation.
// TODO: Wire loot tables into block breaking and entity drops.
package loot

import (
	"encoding/json"
	"math/rand"
)

type TableType string

const (
	TableTypeBlock  TableType = "minecraft:block"
	TableTypeEntity TableType = "minecraft:entity"
	TableTypeChest  TableType = "minecraft:chest"
)

type Table struct {
	Type           TableType `json:"type"`
	RandomSequence string    `json:"random_sequence,omitempty"`
	Pools          []Pool    `json:"pools,omitempty"`
}

type Pool struct {
	Rolls      NumberProvider `json:"rolls"`
	BonusRolls float64        `json:"bonus_rolls"`
	Entries    []Entry        `json:"entries"`
	Conditions []Condition    `json:"conditions,omitempty"`
}

type Entry struct {
	Type       string      `json:"type"`
	Name       string      `json:"name,omitempty"`
	Children   []Entry     `json:"children,omitempty"`
	Conditions []Condition `json:"conditions,omitempty"`
	Functions  []Function  `json:"functions,omitempty"`
	Weight     int         `json:"weight,omitempty"`
	Quality    int         `json:"quality,omitempty"`
}

type Condition struct {
	Condition   string          `json:"condition"`
	Enchantment string          `json:"enchantment,omitempty"`
	Chances     []float64       `json:"chances,omitempty"`
	Predicate   json.RawMessage `json:"predicate,omitempty"`
	Terms       []Condition     `json:"terms,omitempty"`
	Term        *Condition      `json:"term,omitempty"`
	Block       string          `json:"block,omitempty"`
	Properties  json.RawMessage `json:"properties,omitempty"`
}

type Function struct {
	Function    string               `json:"function"`
	Enchantment string               `json:"enchantment,omitempty"`
	Formula     string               `json:"formula,omitempty"`
	Parameters  *BonusParameters     `json:"parameters,omitempty"`
	Count       *NumberProvider      `json:"count,omitempty"`
	Add         bool                 `json:"add,omitempty"`
	Limit       *NumberProviderRange `json:"limit,omitempty"`
	Conditions  []Condition          `json:"conditions,omitempty"`
}

type BonusParameters struct {
	BonusMultiplier float64 `json:"bonusMultiplier,omitempty"`
	Extra           int     `json:"extra,omitempty"`
	Probability     float64 `json:"probability,omitempty"`
}

type NumberProvider struct {
	value    float64
	min      float64
	max      float64
	isRange  bool
	provType string
}

type NumberProviderRange struct {
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

func (n *NumberProvider) UnmarshalJSON(data []byte) error {
	var f float64
	if err := json.Unmarshal(data, &f); err == nil {
		n.value = f
		n.isRange = false
		return nil
	}

	var obj struct {
		Type string  `json:"type"`
		Min  float64 `json:"min"`
		Max  float64 `json:"max"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	n.provType = obj.Type
	n.min = obj.Min
	n.max = obj.Max
	n.isRange = true
	return nil
}

func (n *NumberProvider) Roll(rng *rand.Rand) int {
	if !n.isRange {
		return int(n.value)
	}
	if n.max <= n.min {
		return int(n.min)
	}
	return int(n.min) + rng.Intn(int(n.max-n.min)+1)
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
