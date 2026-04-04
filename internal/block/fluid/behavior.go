package fluid

import (
	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/block/behavior"
)

const (
	BlockIDWater int32 = 35
	BlockIDLava  int32 = 36
)

type FluidBehavior struct {
	behavior.DefaultBehavior
	config FlowConfig
}

func NewWaterBehavior() *FluidBehavior {
	return &FluidBehavior{config: WaterConfig}
}

func NewLavaBehavior() *FluidBehavior {
	return &FluidBehavior{config: LavaOverworldConfig}
}

func (b *FluidBehavior) OnNeighborUpdate(_ *behavior.Context) {}

func (b *FluidBehavior) OnScheduledTick(ctx *behavior.TickContext) *behavior.TickResult {
	return nil
}

func (b *FluidBehavior) ScheduleOnPlace() bool {
	return true
}

func (b *FluidBehavior) ScheduleOnNeighborUpdate() bool {
	return true
}

func (b *FluidBehavior) TickDelay() int {
	return b.config.FlowSpeed
}

func (b *FluidBehavior) Config() FlowConfig {
	return b.config
}

func IsFluidBlock(blockID int32) bool {
	return blockID == BlockIDWater || blockID == BlockIDLava
}

func GetFluidConfig(stateID int32) *FlowConfig {
	fluidType := GetFluidType(stateID)
	switch fluidType {
	case FluidTypeWater:
		return &WaterConfig
	case FluidTypeLava:
		return &LavaOverworldConfig
	}
	return nil
}

func init() {
	waterBehavior := NewWaterBehavior()
	lavaBehavior := NewLavaBehavior()

	waterInfo := block.InfoByName("minecraft:water")
	if waterInfo != nil {
		behavior.Register(waterInfo.ID, waterBehavior)
	}

	lavaInfo := block.InfoByName("minecraft:lava")
	if lavaInfo != nil {
		behavior.Register(lavaInfo.ID, lavaBehavior)
	}
}
