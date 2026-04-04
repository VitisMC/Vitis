package behavior

import (
	"github.com/vitismc/vitis/internal/block"
)

const (
	FurnaceTypeNormal = 0
	FurnaceTypeBlast  = 1
	FurnaceTypeSmoker = 2
)

type FurnaceBehavior struct {
	DefaultBehavior
	WindowType  int32
	Title       string
	SlotCount   int
	FurnaceType int
}

func (b *FurnaceBehavior) OnUse(ctx *Context) bool {
	ctx.StateID = -1
	ctx.FurnaceType = b.FurnaceType
	return true
}

func (b *FurnaceBehavior) ContainerInfo() (windowType int32, title string, slots int) {
	return b.WindowType, b.Title, b.SlotCount
}

func (b *FurnaceBehavior) IsFurnace() bool {
	return true
}

func (b *FurnaceBehavior) GetFurnaceType() int {
	return b.FurnaceType
}

func IsFurnace(stateID int32) (furnaceType int, windowType int32, title string, ok bool) {
	bid := block.BlockIDFromState(stateID)
	if bid < 0 {
		return 0, 0, "", false
	}
	beh := Get(bid)
	if fb, isFurnace := beh.(*FurnaceBehavior); isFurnace {
		return fb.FurnaceType, fb.WindowType, fb.Title, true
	}
	return 0, 0, "", false
}

func init() {
	furnaceBehavior := &FurnaceBehavior{
		WindowType:  14,
		Title:       "Furnace",
		SlotCount:   3,
		FurnaceType: FurnaceTypeNormal,
	}
	blastFurnaceBehavior := &FurnaceBehavior{
		WindowType:  10,
		Title:       "Blast Furnace",
		SlotCount:   3,
		FurnaceType: FurnaceTypeBlast,
	}
	smokerBehavior := &FurnaceBehavior{
		WindowType:  22,
		Title:       "Smoker",
		SlotCount:   3,
		FurnaceType: FurnaceTypeSmoker,
	}

	registerByName("minecraft:furnace", furnaceBehavior)
	registerByName("minecraft:blast_furnace", blastFurnaceBehavior)
	registerByName("minecraft:smoker", smokerBehavior)
}
