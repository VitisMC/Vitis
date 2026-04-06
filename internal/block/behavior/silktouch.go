package behavior

import (
	"math/rand"

	"github.com/vitismc/vitis/internal/block"
)

// SilkTouchAlternateBehavior drops an alternate item normally but the block itself with silk touch.
type SilkTouchAlternateBehavior struct {
	DefaultBehavior
	AlternateItemName string
	AlternateCount    int32
}

func (b *SilkTouchAlternateBehavior) OnBreak(ctx *Context) []Drop {
	if ctx.PlayerGM == 1 {
		return nil
	}

	bid := block.BlockIDFromState(ctx.StateID)
	if bid <= 0 {
		return nil
	}
	info := block.Info(bid)
	if info == nil {
		return nil
	}

	if ctx.HasSilkTouch() {
		blockItemID := itemIDByName(info.Name)
		if blockItemID <= 0 {
			return nil
		}
		return []Drop{{ItemID: blockItemID, Count: 1}}
	}

	if b.AlternateItemName == "" {
		return nil
	}
	altID := itemIDByName(b.AlternateItemName)
	if altID <= 0 {
		return nil
	}
	count := b.AlternateCount
	if count <= 0 {
		count = 1
	}
	return []Drop{{ItemID: altID, Count: count}}
}

// SilkTouchOnlyBehavior drops nothing unless the player has silk touch.
type SilkTouchOnlyBehavior struct {
	DefaultBehavior
}

func (b *SilkTouchOnlyBehavior) OnBreak(ctx *Context) []Drop {
	if ctx.PlayerGM == 1 {
		return nil
	}
	if !ctx.HasSilkTouch() {
		return nil
	}
	bid := block.BlockIDFromState(ctx.StateID)
	if bid <= 0 {
		return nil
	}
	info := block.Info(bid)
	if info == nil {
		return nil
	}
	blockItemID := itemIDByName(info.Name)
	if blockItemID <= 0 {
		return nil
	}
	return []Drop{{ItemID: blockItemID, Count: 1}}
}

// GravelBehavior drops gravel normally, but has a chance to drop flint.
// Fortune increases flint chance: 10% base, 14% fortune 1, 25% fortune 2, 100% fortune 3.
type GravelBehavior struct {
	DefaultBehavior
}

func (b *GravelBehavior) OnBreak(ctx *Context) []Drop {
	if ctx.PlayerGM == 1 {
		return nil
	}

	if ctx.HasSilkTouch() {
		gravelID := itemIDByName("minecraft:gravel")
		if gravelID <= 0 {
			return nil
		}
		return []Drop{{ItemID: gravelID, Count: 1}}
	}

	flintChance := 0.10
	fortune := ctx.FortuneLevel()
	switch fortune {
	case 1:
		flintChance = 0.14
	case 2:
		flintChance = 0.25
	}
	if fortune >= 3 {
		flintChance = 1.0
	}

	if rand.Float64() < flintChance {
		flintID := itemIDByName("minecraft:flint")
		if flintID > 0 {
			return []Drop{{ItemID: flintID, Count: 1}}
		}
	}

	gravelID := itemIDByName("minecraft:gravel")
	if gravelID <= 0 {
		return nil
	}
	return []Drop{{ItemID: gravelID, Count: 1}}
}

// GlowstoneBehavior drops 2-4 glowstone dust, fortune increases max up to 4.
type GlowstoneBehavior struct {
	DefaultBehavior
}

func (b *GlowstoneBehavior) OnBreak(ctx *Context) []Drop {
	if ctx.PlayerGM == 1 {
		return nil
	}

	if ctx.HasSilkTouch() {
		glowID := itemIDByName("minecraft:glowstone")
		if glowID > 0 {
			return []Drop{{ItemID: glowID, Count: 1}}
		}
		return nil
	}

	dustID := itemIDByName("minecraft:glowstone_dust")
	if dustID <= 0 {
		return nil
	}

	count := int32(2) + rand.Int31n(3)
	fortune := ctx.FortuneLevel()
	if fortune > 0 {
		count += rand.Int31n(fortune + 1)
	}
	if count > 4 {
		count = 4
	}
	return []Drop{{ItemID: dustID, Count: count}}
}

func init() {
	grassAlternate := &SilkTouchAlternateBehavior{AlternateItemName: "minecraft:dirt", AlternateCount: 1}
	registerByName("minecraft:grass_block", grassAlternate)
	registerByName("minecraft:mycelium", grassAlternate)
	registerByName("minecraft:podzol", grassAlternate)

	stoneAlternate := &SilkTouchAlternateBehavior{AlternateItemName: "minecraft:cobblestone", AlternateCount: 1}
	registerByName("minecraft:stone", stoneAlternate)

	deepslateCobble := &SilkTouchAlternateBehavior{AlternateItemName: "minecraft:cobbled_deepslate", AlternateCount: 1}
	registerByName("minecraft:deepslate", deepslateCobble)

	bookshelfDrop := &SilkTouchAlternateBehavior{AlternateItemName: "minecraft:book", AlternateCount: 3}
	registerByName("minecraft:bookshelf", bookshelfDrop)

	clayDrop := &SilkTouchAlternateBehavior{AlternateItemName: "minecraft:clay_ball", AlternateCount: 4}
	registerByName("minecraft:clay", clayDrop)

	silkOnly := &SilkTouchOnlyBehavior{}
	glassNames := []string{
		"minecraft:glass", "minecraft:glass_pane",
		"minecraft:white_stained_glass", "minecraft:orange_stained_glass",
		"minecraft:magenta_stained_glass", "minecraft:light_blue_stained_glass",
		"minecraft:yellow_stained_glass", "minecraft:lime_stained_glass",
		"minecraft:pink_stained_glass", "minecraft:gray_stained_glass",
		"minecraft:light_gray_stained_glass", "minecraft:cyan_stained_glass",
		"minecraft:purple_stained_glass", "minecraft:blue_stained_glass",
		"minecraft:brown_stained_glass", "minecraft:green_stained_glass",
		"minecraft:red_stained_glass", "minecraft:black_stained_glass",
		"minecraft:white_stained_glass_pane", "minecraft:orange_stained_glass_pane",
		"minecraft:magenta_stained_glass_pane", "minecraft:light_blue_stained_glass_pane",
		"minecraft:yellow_stained_glass_pane", "minecraft:lime_stained_glass_pane",
		"minecraft:pink_stained_glass_pane", "minecraft:gray_stained_glass_pane",
		"minecraft:light_gray_stained_glass_pane", "minecraft:cyan_stained_glass_pane",
		"minecraft:purple_stained_glass_pane", "minecraft:blue_stained_glass_pane",
		"minecraft:brown_stained_glass_pane", "minecraft:green_stained_glass_pane",
		"minecraft:red_stained_glass_pane", "minecraft:black_stained_glass_pane",
	}
	for _, name := range glassNames {
		registerByName(name, silkOnly)
	}

	iceNames := []string{"minecraft:ice", "minecraft:packed_ice", "minecraft:blue_ice"}
	for _, name := range iceNames {
		registerByName(name, silkOnly)
	}

	registerByName("minecraft:gravel", &GravelBehavior{})
	registerByName("minecraft:glowstone", &GlowstoneBehavior{})
}
