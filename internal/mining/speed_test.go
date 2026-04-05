package mining

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
)

func TestBreakTimeAirInstant(t *testing.T) {
	result := CalculateBreakTime(0, "", true)
	if !result.Instant {
		t.Error("air should break instantly")
	}
}

func TestBreakTimeStoneBareHand(t *testing.T) {
	stoneState := block.DefaultStateID("minecraft:stone")
	if stoneState < 0 {
		t.Skip("stone block not found")
	}

	result := CalculateBreakTime(stoneState, "", true)
	if result.Instant {
		t.Error("stone with bare hand should not be instant")
	}
	if result.Ticks <= 0 {
		t.Errorf("stone bare hand ticks = %d, want > 0", result.Ticks)
	}
}

func TestBreakTimeStoneWoodPickaxe(t *testing.T) {
	stoneState := block.DefaultStateID("minecraft:stone")
	if stoneState < 0 {
		t.Skip("stone block not found")
	}

	result := CalculateBreakTime(stoneState, "minecraft:wooden_pickaxe", true)
	if result.Instant {
		t.Error("stone with wooden pickaxe should not be instant")
	}
	if !result.CanHarvest {
		t.Error("wooden pickaxe should be able to harvest stone")
	}

	bareHand := CalculateBreakTime(stoneState, "", true)
	if result.Ticks >= bareHand.Ticks {
		t.Errorf("wooden pickaxe (%d ticks) should be faster than bare hand (%d ticks)", result.Ticks, bareHand.Ticks)
	}
}

func TestBreakTimeStoneDiamondPickaxe(t *testing.T) {
	stoneState := block.DefaultStateID("minecraft:stone")
	if stoneState < 0 {
		t.Skip("stone block not found")
	}

	diamond := CalculateBreakTime(stoneState, "minecraft:diamond_pickaxe", true)
	wooden := CalculateBreakTime(stoneState, "minecraft:wooden_pickaxe", true)

	if diamond.Ticks >= wooden.Ticks {
		t.Errorf("diamond pickaxe (%d) should be faster than wooden (%d)", diamond.Ticks, wooden.Ticks)
	}
}

func TestBreakTimeObsidianRequiresDiamond(t *testing.T) {
	obsState := block.DefaultStateID("minecraft:obsidian")
	if obsState < 0 {
		t.Skip("obsidian block not found")
	}

	iron := CalculateBreakTime(obsState, "minecraft:iron_pickaxe", true)
	if iron.CanHarvest {
		t.Error("iron pickaxe should not harvest obsidian")
	}

	diamond := CalculateBreakTime(obsState, "minecraft:diamond_pickaxe", true)
	if !diamond.CanHarvest {
		t.Error("diamond pickaxe should harvest obsidian")
	}
}

func TestBreakTimeDirtWithShovel(t *testing.T) {
	dirtState := block.DefaultStateID("minecraft:dirt")
	if dirtState < 0 {
		t.Skip("dirt block not found")
	}

	shovel := CalculateBreakTime(dirtState, "minecraft:iron_shovel", true)
	hand := CalculateBreakTime(dirtState, "", true)

	if shovel.Ticks >= hand.Ticks {
		t.Errorf("iron shovel (%d) should be faster than hand (%d) on dirt", shovel.Ticks, hand.Ticks)
	}
}

func TestBreakTimeBedrockUnbreakable(t *testing.T) {
	bedState := block.DefaultStateID("minecraft:bedrock")
	if bedState < 0 {
		t.Skip("bedrock block not found")
	}

	result := CalculateBreakTime(bedState, "minecraft:netherite_pickaxe", true)
	if result.CanHarvest {
		t.Error("bedrock should not be harvestable")
	}
	if result.Ticks != -1 {
		t.Errorf("bedrock ticks = %d, want -1", result.Ticks)
	}
}

func TestBreakTimeNotOnGround(t *testing.T) {
	stoneState := block.DefaultStateID("minecraft:stone")
	if stoneState < 0 {
		t.Skip("stone block not found")
	}

	ground := CalculateBreakTime(stoneState, "minecraft:iron_pickaxe", true)
	air := CalculateBreakTime(stoneState, "minecraft:iron_pickaxe", false)

	if air.Ticks <= ground.Ticks {
		t.Errorf("mining in air (%d) should be slower than on ground (%d)", air.Ticks, ground.Ticks)
	}
}

func TestBreakTimeGoldPickaxeFast(t *testing.T) {
	stoneState := block.DefaultStateID("minecraft:stone")
	if stoneState < 0 {
		t.Skip("stone block not found")
	}

	gold := CalculateBreakTime(stoneState, "minecraft:golden_pickaxe", true)
	diamond := CalculateBreakTime(stoneState, "minecraft:diamond_pickaxe", true)

	if gold.Ticks >= diamond.Ticks {
		t.Errorf("gold pickaxe (%d) should be faster than diamond (%d) on stone", gold.Ticks, diamond.Ticks)
	}
}
