package loot

import (
	"encoding/json"
	"math"
)

type Drop struct {
	ItemID string
	Count  int
}

func (t *Table) GetDrops(ctx *Context) []Drop {
	var drops []Drop

	for _, pool := range t.Pools {
		if !evaluateConditions(pool.Conditions, ctx) {
			continue
		}

		rolls := pool.Rolls.Roll(ctx.Rng)
		bonusRolls := int(pool.BonusRolls * float64(ctx.FortuneLevel()))
		totalRolls := rolls + bonusRolls

		for i := 0; i < totalRolls; i++ {
			entryDrops := evaluateEntries(pool.Entries, ctx)
			drops = append(drops, entryDrops...)
		}
	}

	return drops
}

func evaluateEntries(entries []Entry, ctx *Context) []Drop {
	var drops []Drop

	for _, entry := range entries {
		entryDrops := evaluateEntry(&entry, ctx)
		if len(entryDrops) > 0 {
			drops = append(drops, entryDrops...)
		}
	}

	return drops
}

func evaluateEntry(entry *Entry, ctx *Context) []Drop {
	if !evaluateConditions(entry.Conditions, ctx) {
		return nil
	}

	switch EntryType(entry.Type) {
	case EntryTypeItem:
		return evaluateItemEntry(entry, ctx)

	case EntryTypeAlternatives:
		for _, child := range entry.Children {
			if evaluateConditions(child.Conditions, ctx) {
				return evaluateEntry(&child, ctx)
			}
		}
		return nil

	case EntryTypeSequence:
		var drops []Drop
		for _, child := range entry.Children {
			if !evaluateConditions(child.Conditions, ctx) {
				break
			}
			drops = append(drops, evaluateEntry(&child, ctx)...)
		}
		return drops

	case EntryTypeGroup:
		var drops []Drop
		for _, child := range entry.Children {
			drops = append(drops, evaluateEntry(&child, ctx)...)
		}
		return drops

	case EntryTypeEmpty:
		return nil

	default:
		return nil
	}
}

func evaluateItemEntry(entry *Entry, ctx *Context) []Drop {
	if entry.Name == "" {
		return nil
	}

	count := 1
	for _, fn := range entry.Functions {
		count = applyFunction(&fn, count, ctx)
	}

	if count <= 0 {
		return nil
	}

	return []Drop{{ItemID: entry.Name, Count: count}}
}

func applyFunction(fn *Function, count int, ctx *Context) int {
	if !evaluateConditions(fn.Conditions, ctx) {
		return count
	}

	switch FunctionType(fn.Function) {
	case FunctionSetCount:
		if fn.Count != nil {
			newCount := fn.Count.Roll(ctx.Rng)
			if fn.Add {
				return count + newCount
			}
			return newCount
		}

	case FunctionApplyBonus:
		fortuneLevel := ctx.EnchantmentLevel(fn.Enchantment)
		if fortuneLevel <= 0 {
			return count
		}
		return applyBonusFormula(fn.Formula, fn.Parameters, count, fortuneLevel, ctx)

	case FunctionExplosionDecay:
		if ctx.ExplosionRadius > 0 {
			survivalChance := 1.0 / ctx.ExplosionRadius
			surviving := 0
			for i := 0; i < count; i++ {
				if ctx.Rng.Float64() < survivalChance {
					surviving++
				}
			}
			return surviving
		}

	case FunctionLimitCount:
		if fn.Limit != nil {
			if fn.Limit.Min != nil && count < int(*fn.Limit.Min) {
				count = int(*fn.Limit.Min)
			}
			if fn.Limit.Max != nil && count > int(*fn.Limit.Max) {
				count = int(*fn.Limit.Max)
			}
		}
	}

	return count
}

func applyBonusFormula(formula string, params *BonusParameters, count, fortuneLevel int, ctx *Context) int {
	switch FormulaType(formula) {
	case FormulaOreDrops:
		if fortuneLevel > 0 {
			multiplier := ctx.Rng.Intn(fortuneLevel + 2)
			if multiplier > 1 {
				return count * multiplier
			}
		}
		return count

	case FormulaBinomialWithBonusCount:
		if params != nil {
			extra := params.Extra
			prob := params.Probability
			bonus := 0
			trials := fortuneLevel + extra
			for i := 0; i < trials; i++ {
				if ctx.Rng.Float64() < prob {
					bonus++
				}
			}
			return count + bonus
		}

	case FormulaUniformBonusCount:
		if params != nil {
			bonus := ctx.Rng.Intn(int(params.BonusMultiplier)*fortuneLevel + 1)
			return count + bonus
		}
	}

	return count
}

func evaluateConditions(conditions []Condition, ctx *Context) bool {
	for _, cond := range conditions {
		if !evaluateCondition(&cond, ctx) {
			return false
		}
	}
	return true
}

func evaluateCondition(cond *Condition, ctx *Context) bool {
	switch ConditionType(cond.Condition) {
	case ConditionSurvivesExplosion:
		if ctx.ExplosionRadius <= 0 {
			return true
		}
		return ctx.Rng.Float64() < (1.0 / ctx.ExplosionRadius)

	case ConditionMatchTool:
		return evaluateMatchToolCondition(cond, ctx)

	case ConditionTableBonus:
		return evaluateTableBonusCondition(cond, ctx)

	case ConditionBlockStateProps:
		return evaluateBlockStateCondition(cond, ctx)

	case ConditionInverted:
		if cond.Term != nil {
			return !evaluateCondition(cond.Term, ctx)
		}
		return true

	case ConditionAnyOf:
		for _, term := range cond.Terms {
			if evaluateCondition(&term, ctx) {
				return true
			}
		}
		return len(cond.Terms) == 0

	case ConditionAllOf:
		for _, term := range cond.Terms {
			if !evaluateCondition(&term, ctx) {
				return false
			}
		}
		return true

	case ConditionRandomChance:
		return true

	default:
		return true
	}
}

func evaluateMatchToolCondition(cond *Condition, ctx *Context) bool {
	if ctx.Tool == nil {
		return false
	}

	if len(cond.Predicate) == 0 {
		return true
	}

	var pred struct {
		Predicates struct {
			Enchantments []struct {
				Enchantments string `json:"enchantments"`
				Levels       struct {
					Min int `json:"min"`
				} `json:"levels"`
			} `json:"minecraft:enchantments"`
		} `json:"predicates"`
	}

	if err := json.Unmarshal(cond.Predicate, &pred); err != nil {
		return false
	}

	for _, enchReq := range pred.Predicates.Enchantments {
		level := ctx.EnchantmentLevel(enchReq.Enchantments)
		if level < enchReq.Levels.Min {
			return false
		}
	}

	return true
}

func evaluateTableBonusCondition(cond *Condition, ctx *Context) bool {
	if len(cond.Chances) == 0 {
		return true
	}

	level := ctx.EnchantmentLevel(cond.Enchantment)
	idx := level
	if idx >= len(cond.Chances) {
		idx = len(cond.Chances) - 1
	}
	if idx < 0 {
		idx = 0
	}

	chance := cond.Chances[idx]
	return ctx.Rng.Float64() < chance
}

func evaluateBlockStateCondition(cond *Condition, ctx *Context) bool {
	if len(cond.Properties) == 0 || ctx.BlockState == nil {
		return true
	}

	var props map[string]string
	if err := json.Unmarshal(cond.Properties, &props); err != nil {
		return true
	}

	for key, expected := range props {
		actual, ok := ctx.BlockState[key]
		if !ok || actual != expected {
			return false
		}
	}

	return true
}

var _ = math.Floor
