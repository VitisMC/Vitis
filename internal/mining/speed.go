package mining

import (
	"math"

	"github.com/vitismc/vitis/internal/block"
	genblock "github.com/vitismc/vitis/internal/data/generated/block"
)

const (
	ticksPerSecond = 20.0
)

// BreakResult contains the computed mining parameters for a block+tool combination.
type BreakResult struct {
	CanHarvest bool
	Ticks      int
	Instant    bool
}

// CalculateBreakTime computes the number of ticks required to break a block.
// itemName is the namespaced name of the held tool (empty string for hand).
// onGround indicates whether the player is standing on the ground.
func CalculateBreakTime(stateID int32, itemName string, onGround bool) BreakResult {
	blockID := block.BlockIDFromState(stateID)
	if blockID < 0 {
		return BreakResult{CanHarvest: true, Ticks: 0, Instant: true}
	}

	info := &genblock.Blocks[blockID]

	if info.Hardness < 0 {
		return BreakResult{CanHarvest: false, Ticks: -1}
	}

	if info.Hardness == 0 {
		return BreakResult{CanHarvest: true, Ticks: 0, Instant: true}
	}

	tool := GetToolInfo(itemName)
	rule := GetHarvestRule(info.Name)

	canHarvest := true
	if rule.RequiresTool {
		if tool.Type != rule.EffectiveTool || TierLevel(tool.Tier) < TierLevel(rule.MinTier) {
			canHarvest = false
		}
	}

	speedMultiplier := 1.0
	if tool.Type != ToolNone && tool.Type == rule.EffectiveTool {
		speedMultiplier = tool.Speed
	}

	damage := speedMultiplier / info.Hardness
	if canHarvest {
		damage /= 30.0
	} else {
		damage /= 100.0
	}

	if !onGround {
		damage /= 5.0
	}

	if damage >= 1.0 {
		return BreakResult{CanHarvest: canHarvest, Ticks: 0, Instant: true}
	}

	ticks := int(math.Ceil(1.0 / damage))
	return BreakResult{CanHarvest: canHarvest, Ticks: ticks, Instant: false}
}
