package behavior

import (
	"strconv"

	"github.com/vitismc/vitis/internal/block"
)

// CropBehavior grows crops from random ticks using light and farmland conditions.
type CropBehavior struct {
	DefaultBehavior
	MaxAge int
}

// OnNeighborCheck removes a crop only when resident support or light is invalid.
func (b *CropBehavior) OnNeighborCheck(ctx *NeighborContext) *NeighborResult {
	if ctx == nil || ctx.GetBlock == nil {
		return nil
	}
	below, available := ctx.GetBlock(ctx.X, ctx.Y-1, ctx.Z)
	if !available {
		return nil
	}
	if block.NameFromState(below) != "minecraft:farmland" || ctx.Light < 8 {
		return &NeighborResult{StateID: 0, Changed: true, Drop: true}
	}
	return nil
}

// CanRandomTick excludes mature crops from random sampling eligibility.
func (b *CropBehavior) CanRandomTick(state int32) bool {
	age, err := strconv.Atoi(block.PropertiesFromState(state)["age"])
	return err == nil && age >= 0 && age < b.MaxAge
}

// OnRandomTick evaluates growth without treating unloaded neighbors as air.
func (b *CropBehavior) OnRandomTick(ctx *RandomTickContext) *TickResult {
	if ctx == nil || ctx.Light < 9 || ctx.GetBlock == nil || ctx.RandomBound == nil || !b.CanRandomTick(ctx.StateID) {
		return nil
	}
	if block.NameFromState(ctx.StateID) == "minecraft:beetroots" && ctx.RandomBound(3) != 0 {
		return nil
	}
	speed, available := cropGrowthSpeed(ctx)
	if !available || ctx.RandomBound(int32(25/speed)+1) != 0 {
		return nil
	}
	props := block.PropertiesFromState(ctx.StateID)
	age, _ := strconv.Atoi(props["age"])
	props["age"] = strconv.Itoa(age + 1)
	state := block.StateID(block.NameFromState(ctx.StateID), props)
	if state < 0 {
		return nil
	}
	return &TickResult{SetBlock: true, NewBlockState: state}
}

// cropGrowthSpeed computes moisture bonuses and crowding penalties with float32 arithmetic.
func cropGrowthSpeed(ctx *RandomTickContext) (float32, bool) {
	speed := float32(1)
	for dx := -1; dx <= 1; dx++ {
		for dz := -1; dz <= 1; dz++ {
			state, available := ctx.GetBlock(ctx.X+dx, ctx.Y-1, ctx.Z+dz)
			if !available {
				return 0, false
			}
			bonus := float32(0)
			if block.NameFromState(state) == "minecraft:farmland" {
				bonus = 1
				moisture, _ := strconv.Atoi(block.PropertiesFromState(state)["moisture"])
				if moisture > 0 {
					bonus = 3
				}
			}
			if dx != 0 || dz != 0 {
				bonus /= 4
			}
			speed += bonus
		}
	}
	cropID := block.BlockIDFromState(ctx.StateID)
	var alongX, alongZ, diagonal bool
	for dx := -1; dx <= 1; dx++ {
		for dz := -1; dz <= 1; dz++ {
			if dx == 0 && dz == 0 {
				continue
			}
			state, available := ctx.GetBlock(ctx.X+dx, ctx.Y, ctx.Z+dz)
			if !available {
				return 0, false
			}
			if block.BlockIDFromState(state) != cropID {
				continue
			}
			if dx == 0 {
				alongZ = true
			} else if dz == 0 {
				alongX = true
			} else {
				diagonal = true
			}
		}
	}
	if alongX && alongZ || diagonal {
		speed /= 2
	}
	return speed, true
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

// OnNeighborCheck defers obstructed farmland conversion by one scheduled tick.
func (b *FarmlandBehavior) OnNeighborCheck(ctx *NeighborContext) *NeighborResult {
	if ctx == nil || ctx.GetBlock == nil {
		return nil
	}
	above, available := ctx.GetBlock(ctx.X, ctx.Y+1, ctx.Z)
	if !available || !block.IsSolid(above) || IsFenceGate(above) || block.NameFromState(above) == "minecraft:moving_piston" {
		return nil
	}
	if !ctx.Scheduled {
		return &NeighborResult{Delay: 1}
	}
	return &NeighborResult{StateID: block.DefaultStateID("minecraft:dirt"), Changed: true}
}

// OnBreak drops dirt when farmland is destroyed outside creative mode.
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
