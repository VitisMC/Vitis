package behavior

import (
	"github.com/vitismc/vitis/internal/block"
)

const (
	BlockIDSand     int32 = 37
	BlockIDRedSand  int32 = 39
	BlockIDGravel   int32 = 40
	
	FallingTickDelay = 2
)

type FallingBlockBehavior struct {
	DefaultBehavior
}

func (b *FallingBlockBehavior) OnNeighborUpdate(ctx *Context) {}

func (b *FallingBlockBehavior) OnScheduledTick(ctx *TickContext) *TickResult {
	return &TickResult{
		SpawnFallingBlock: true,
		BlockStateID:      ctx.StateID,
	}
}

func (b *FallingBlockBehavior) ScheduleOnPlace() bool {
	return true
}

func (b *FallingBlockBehavior) ScheduleOnNeighborUpdate() bool {
	return true
}

func (b *FallingBlockBehavior) TickDelay() int {
	return FallingTickDelay
}

func CanFallThrough(stateID int32) bool {
	if stateID == 0 {
		return true
	}
	name := block.NameFromState(stateID)
	switch name {
	case "minecraft:air", "minecraft:cave_air", "minecraft:void_air":
		return true
	case "minecraft:fire", "minecraft:soul_fire":
		return true
	case "minecraft:water", "minecraft:lava":
		return true
	}
	info := block.Info(block.BlockIDFromState(stateID))
	if info == nil {
		return false
	}
	if !info.Solid {
		return true
	}
	return false
}

func IsFallingBlock(blockID int32) bool {
	switch blockID {
	case BlockIDSand, BlockIDRedSand, BlockIDGravel:
		return true
	}
	return false
}

func init() {
	fallingBehavior := &FallingBlockBehavior{}
	Register(BlockIDSand, fallingBehavior)
	Register(BlockIDRedSand, fallingBehavior)
	Register(BlockIDGravel, fallingBehavior)
}
