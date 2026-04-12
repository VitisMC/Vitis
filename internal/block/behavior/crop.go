package behavior

import (
	"strconv"

	"github.com/vitismc/vitis/internal/block"
)

// CropBehavior handles crop growth via scheduled ticks.
// Crops advance one age stage per tick until they reach MaxAge.
type CropBehavior struct {
	DefaultBehavior
	MaxAge int
}

func (b *CropBehavior) ScheduleOnPlace() bool        { return true }
func (b *CropBehavior) ScheduleOnNeighborUpdate() bool { return false }
func (b *CropBehavior) TickDelay() int                { return 600 } // ~30 seconds average

func (b *CropBehavior) OnScheduledTick(ctx *TickContext) *TickResult {
	props := block.PropertiesFromState(ctx.StateID)
	if props == nil {
		return nil
	}
	ageStr, ok := props["age"]
	if !ok {
		return nil
	}
	age, err := strconv.Atoi(ageStr)
	if err != nil {
		return nil
	}
	if age >= b.MaxAge {
		return nil
	}

	// Advance one growth stage
	newAge := age + 1
	props["age"] = strconv.Itoa(newAge)

	bid := block.BlockIDFromState(ctx.StateID)
	info := block.Info(bid)
	if info == nil {
		return nil
	}
	newState := block.StateID(info.Name, props)
	if newState <= 0 {
		return nil
	}

	result := &TickResult{
		SetBlock:      true,
		NewBlockState: newState,
	}
	// Reschedule if not fully grown
	if newAge < b.MaxAge {
		result.Reschedule = true
	}
	return result
}

func (b *CropBehavior) OnBreak(ctx *Context) []Drop {
	if ctx.PlayerGM == 1 {
		return nil
	}
	props := block.PropertiesFromState(ctx.StateID)
	if props == nil {
		return nil
	}
	bid := block.BlockIDFromState(ctx.StateID)
	info := block.Info(bid)
	if info == nil {
		return nil
	}

	// Drop the seed item; if fully grown, also drop the crop
	seedItem := cropSeedItem(info.Name)
	if seedItem <= 0 {
		return nil
	}

	ageStr := props["age"]
	age, _ := strconv.Atoi(ageStr)
	drops := []Drop{{ItemID: seedItem, Count: 1}}
	if age >= b.MaxAge {
		cropItem := cropHarvestItem(info.Name)
		if cropItem > 0 {
			drops = append(drops, Drop{ItemID: cropItem, Count: 1})
		}
	}
	return drops
}

// FarmlandBehavior handles farmland block behavior.
type FarmlandBehavior struct {
	DefaultBehavior
}

func (b *FarmlandBehavior) OnBreak(ctx *Context) []Drop {
	if ctx.PlayerGM == 1 {
		return nil
	}
	dirtID := itemIDFromBlockName("minecraft:dirt")
	if dirtID <= 0 {
		return nil
	}
	return []Drop{{ItemID: dirtID, Count: 1}}
}

func cropSeedItem(blockName string) int32 {
	switch blockName {
	case "minecraft:wheat":
		return itemIDFromBlockName("minecraft:wheat_seeds")
	case "minecraft:carrots":
		return itemIDFromBlockName("minecraft:carrot")
	case "minecraft:potatoes":
		return itemIDFromBlockName("minecraft:potato")
	case "minecraft:beetroots":
		return itemIDFromBlockName("minecraft:beetroot_seeds")
	}
	return 0
}

func cropHarvestItem(blockName string) int32 {
	switch blockName {
	case "minecraft:wheat":
		return itemIDFromBlockName("minecraft:wheat")
	case "minecraft:carrots":
		return itemIDFromBlockName("minecraft:carrot")
	case "minecraft:potatoes":
		return itemIDFromBlockName("minecraft:potato")
	case "minecraft:beetroots":
		return itemIDFromBlockName("minecraft:beetroot")
	}
	return 0
}

func init() {
	wheatBehavior := &CropBehavior{MaxAge: 7}
	carrotBehavior := &CropBehavior{MaxAge: 7}
	potatoBehavior := &CropBehavior{MaxAge: 7}
	beetrootBehavior := &CropBehavior{MaxAge: 3}
	farmlandBehavior := &FarmlandBehavior{}

	registerByName("minecraft:wheat", wheatBehavior)
	registerByName("minecraft:carrots", carrotBehavior)
	registerByName("minecraft:potatoes", potatoBehavior)
	registerByName("minecraft:beetroots", beetrootBehavior)
	registerByName("minecraft:farmland", farmlandBehavior)
}
