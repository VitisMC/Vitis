package behavior

import (
	"math/rand"

	"github.com/vitismc/vitis/internal/block"
)

// OreDropSpec defines what an ore drops when mined.
type OreDropSpec struct {
	DropItemName string
	MinCount     int32
	MaxCount     int32
}

// OreBehavior handles ore block breaking with fortune and silk touch support.
type OreBehavior struct {
	DefaultBehavior
	Spec OreDropSpec
}

func (b *OreBehavior) OnBreak(ctx *Context) []Drop {
	if ctx.PlayerGM == 1 {
		return nil
	}

	if ctx.HasSilkTouch() {
		bid := block.BlockIDFromState(ctx.StateID)
		if bid <= 0 {
			return nil
		}
		info := block.Info(bid)
		if info == nil {
			return nil
		}
		oreItemID := itemIDByName(info.Name)
		if oreItemID <= 0 {
			return nil
		}
		return []Drop{{ItemID: oreItemID, Count: 1}}
	}

	dropID := itemIDByName(b.Spec.DropItemName)
	if dropID <= 0 {
		return nil
	}

	count := b.Spec.MinCount
	if b.Spec.MaxCount > b.Spec.MinCount {
		count += rand.Int31n(b.Spec.MaxCount - b.Spec.MinCount + 1)
	}

	fortune := ctx.FortuneLevel()
	if fortune > 0 {
		count = applyOreFortuneMultiplier(count, fortune)
	}

	if count <= 0 {
		return nil
	}
	return []Drop{{ItemID: dropID, Count: count}}
}

// applyOreFortuneMultiplier applies the uniform_bonus_count fortune formula.
// multiplier = 1 + random(0..fortune), applied to the base count.
func applyOreFortuneMultiplier(baseCount, fortuneLevel int32) int32 {
	multiplier := 1 + rand.Int31n(fortuneLevel+1)
	return baseCount * multiplier
}

func init() {
	ores := []struct {
		BlockName string
		Spec      OreDropSpec
	}{
		{"minecraft:coal_ore", OreDropSpec{"minecraft:coal", 1, 1}},
		{"minecraft:deepslate_coal_ore", OreDropSpec{"minecraft:coal", 1, 1}},
		{"minecraft:iron_ore", OreDropSpec{"minecraft:raw_iron", 1, 1}},
		{"minecraft:deepslate_iron_ore", OreDropSpec{"minecraft:raw_iron", 1, 1}},
		{"minecraft:copper_ore", OreDropSpec{"minecraft:raw_copper", 2, 5}},
		{"minecraft:deepslate_copper_ore", OreDropSpec{"minecraft:raw_copper", 2, 5}},
		{"minecraft:gold_ore", OreDropSpec{"minecraft:raw_gold", 1, 1}},
		{"minecraft:deepslate_gold_ore", OreDropSpec{"minecraft:raw_gold", 1, 1}},
		{"minecraft:diamond_ore", OreDropSpec{"minecraft:diamond", 1, 1}},
		{"minecraft:deepslate_diamond_ore", OreDropSpec{"minecraft:diamond", 1, 1}},
		{"minecraft:lapis_ore", OreDropSpec{"minecraft:lapis_lazuli", 4, 9}},
		{"minecraft:deepslate_lapis_ore", OreDropSpec{"minecraft:lapis_lazuli", 4, 9}},
		{"minecraft:redstone_ore", OreDropSpec{"minecraft:redstone", 4, 5}},
		{"minecraft:deepslate_redstone_ore", OreDropSpec{"minecraft:redstone", 4, 5}},
		{"minecraft:emerald_ore", OreDropSpec{"minecraft:emerald", 1, 1}},
		{"minecraft:deepslate_emerald_ore", OreDropSpec{"minecraft:emerald", 1, 1}},
		{"minecraft:nether_quartz_ore", OreDropSpec{"minecraft:quartz", 1, 1}},
		{"minecraft:nether_gold_ore", OreDropSpec{"minecraft:gold_nugget", 2, 6}},
	}

	for _, ore := range ores {
		registerByName(ore.BlockName, &OreBehavior{Spec: ore.Spec})
	}
}
