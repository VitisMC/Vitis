package loot

import (
	"encoding/json"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"math"
)

type Drop struct {
	ItemID string
	Count  int
	Stack  inventory.ItemStack
}

// GetDrops evaluates pool selections and applies pool then table functions.
func (t *Table) GetDrops(ctx *Context) []Drop {
	if t == nil || ctx == nil || ctx.Rng == nil {
		return nil
	}
	var drops []Drop

	for _, pool := range t.Pools {
		if !evaluateConditions(pool.Conditions, ctx) {
			continue
		}

		rolls := pool.Rolls.Roll(ctx.Rng)
		bonus := pool.BonusRolls
		if pool.bonusProvider != nil {
			bonus = pool.bonusProvider.RollFloat(ctx.Rng)
		}
		bonusRolls := int(math.Floor(bonus * ctx.Luck))
		totalRolls := rolls + bonusRolls

		for i := 0; i < totalRolls; i++ {
			entryDrops := evaluateEntries(pool.Entries, ctx)
			drops = append(drops, applyDropFunctions(entryDrops, pool.Functions, ctx)...)
		}
	}

	return applyDropFunctions(drops, t.Functions, ctx)
}

// evaluateEntryContent evaluates an entry whose own conditions already passed.
func evaluateEntryContent(entry *Entry, ctx *Context) []Drop {
	switch EntryType(entry.Type) {
	case EntryTypeItem:
		return evaluateItemEntry(entry, ctx)

	case EntryTypeEmpty:
		return nil

	case EntryTypeLootTable:
		if ctx.depth >= 64 {
			return nil
		}
		child := *ctx
		child.depth++
		if entry.Value.Inline != nil {
			return applyDropFunctions(entry.Value.Inline.GetDrops(&child), entry.Functions, ctx)
		}
		if ctx.tableResolver == nil || entry.Value.ID == "" {
			return nil
		}
		return applyDropFunctions(ctx.tableResolver(entry.Value.ID, &child), entry.Functions, ctx)

	case EntryTypeTag:
		if ctx.tagResolver == nil {
			return nil
		}
		names := ctx.tagResolver(entry.Name)
		if len(names) == 0 {
			return nil
		}
		drops := make([]Drop, 0, len(names))
		for _, name := range names {
			drops = append(drops, Drop{ItemID: name, Count: 1})
		}
		return applyDropFunctions(drops, entry.Functions, ctx)

	default:
		return nil
	}
}

// evaluateItemEntry creates one stack before applying the singleton's functions.
func evaluateItemEntry(entry *Entry, ctx *Context) []Drop {
	if entry.Name == "" {
		return nil
	}

	return applyDropFunctions([]Drop{{ItemID: entry.Name, Count: 1}}, entry.Functions, ctx)
}

// applyDropFunctions applies each function once per stack while preserving its components.
func applyDropFunctions(drops []Drop, functions []Function, ctx *Context) []Drop {
	result := drops[:0]
	for _, drop := range drops {
		stack := drop.Stack.Clone()
		if stack.ItemID <= 0 {
			stack = inventory.NewSlot(item.IDByName(drop.ItemID), int32(drop.Count))
		}
		count := drop.Count
		for _, fn := range functions {
			if !evaluateConditions(fn.Conditions, ctx) {
				continue
			}
			fn.Conditions = nil
			count = applyFunction(&fn, count, ctx)
			stack.ItemCount = int32(count)
			applyStackFunction(&fn, &stack, ctx)
		}
		if count <= 0 {
			continue
		}
		stack.ItemCount = int32(count)
		drop.Count, drop.ItemID = count, item.NameByID(stack.ItemID)
		if ctx.fullStacks {
			drop.Stack = stack
		}
		result = append(result, drop)
	}
	return result
}

func applyFunction(fn *Function, count int, ctx *Context) int {
	if !evaluateConditions(fn.Conditions, ctx) {
		return count
	}

	switch FunctionType(fn.Function) {
	case "minecraft:enchanted_count_increase":
		if level := ctx.EnchantmentLevel(fn.Enchantment); level > 0 && fn.Count != nil {
			count += int(math.Floor(fn.Count.RollFloat(ctx.Rng)*float64(level) + .5))
			if fn.Limit != nil && fn.Limit.Max != nil && *fn.Limit.Max > 0 {
				count = min(count, int(*fn.Limit.Max))
			}
			return count
		}
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
		return applyBonusFormula(fn.Formula, fn.Parameters, count, max(0, fortuneLevel), ctx)

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
		return false

	case ConditionAnyOf:
		for _, term := range cond.Terms {
			if evaluateCondition(&term, ctx) {
				return true
			}
		}
		return false

	case ConditionAllOf:
		for _, term := range cond.Terms {
			if !evaluateCondition(&term, ctx) {
				return false
			}
		}
		return true

	case ConditionRandomChance:
		if cond.Chance < 0 || cond.Chance > 1 || ctx == nil || ctx.Rng == nil {
			return false
		}
		return ctx.Rng.Float64() < cond.Chance

	case "minecraft:killed_by_player":
		return ctx.KilledByPlayer
	case "minecraft:random_chance_with_enchanted_bonus":
		chance := cond.UnenchantedChance
		if level := ctx.EnchantmentLevel(cond.Enchantment); level > 0 {
			if cond.EnchantedChance.Type != "minecraft:linear" {
				return false
			}
			chance = cond.EnchantedChance.Base + float64(level-1)*cond.EnchantedChance.PerLevel
		}
		return ctx.Rng.Float64() < chance
	case "minecraft:entity_properties":
		var subject *EntityInfo
		switch cond.Entity {
		case "", "this":
			subject = ctx.Entity
		case "attacker":
			subject = ctx.Attacker
		case "direct_attacker":
			subject = ctx.DirectAttacker
		default:
			return false
		}
		return matchesEntity(cond.Predicate, subject, ctx)
	case "minecraft:damage_source_properties":
		var predicate map[string]json.RawMessage
		if json.Unmarshal(cond.Predicate, &predicate) != nil || predicate == nil {
			return false
		}
		for key, value := range predicate {
			switch key {
			case "tags":
				if !matchesDamageTags(value, ctx) {
					return false
				}
			case "is_direct":
				var expected bool
				if ctx.DamageIsDirect == nil || string(value) == "null" || json.Unmarshal(value, &expected) != nil || *ctx.DamageIsDirect != expected {
					return false
				}
			case "source_entity":
				if !matchesEntity(value, ctx.Attacker, ctx) {
					return false
				}
			case "direct_entity":
				if !matchesEntity(value, ctx.DirectAttacker, ctx) {
					return false
				}
			default:
				return false
			}
		}
		return true

	default:
		return false
	}
}

func evaluateMatchToolCondition(cond *Condition, ctx *Context) bool {
	if ctx.Tool == nil {
		return false
	}

	if len(cond.Predicate) == 0 {
		return true
	}

	return matchesMainhand(cond.Predicate, ctx.Tool, ctx)
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
