package behavior

import "github.com/vitismc/vitis/internal/block"

// Context carries information about a block interaction.
type Context struct {
	X, Y, Z          int
	StateID          int32
	Face             int32
	PlayerGM         int32
	FurnaceType      int
	ToolName         string
	ToolEnchantments map[int32]int32
}

// HasSilkTouch returns true if the tool has the silk touch enchantment.
func (c *Context) HasSilkTouch() bool {
	return c.ToolEnchantments != nil && c.ToolEnchantments[33] > 0
}

// FortuneLevel returns the fortune enchantment level on the tool.
func (c *Context) FortuneLevel() int32 {
	if c.ToolEnchantments == nil {
		return 0
	}
	return c.ToolEnchantments[13]
}

// Drop describes one item drop from a block.
type Drop struct {
	ItemID int32
	Count  int32
}

// TickContext carries information for scheduled block ticks.
type TickContext struct {
	X, Y, Z     int
	StateID     int32
	CurrentTick uint64
}

// TickResult describes actions to take after a scheduled tick.
type TickResult struct {
	SpawnFallingBlock bool
	BlockStateID      int32
}

// Behavior defines per-block custom logic.
type Behavior interface {
	OnPlace(ctx *Context) int32
	OnBreak(ctx *Context) []Drop
	OnUse(ctx *Context) bool
	OnNeighborUpdate(ctx *Context)
	OnScheduledTick(ctx *TickContext) *TickResult
	ScheduleOnPlace() bool
	ScheduleOnNeighborUpdate() bool
	TickDelay() int
}

// DefaultBehavior is the fallback behavior for blocks without custom logic.
type DefaultBehavior struct{}

func (d *DefaultBehavior) OnPlace(ctx *Context) int32 {
	return ctx.StateID
}

func (d *DefaultBehavior) OnBreak(ctx *Context) []Drop {
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
	itemID := itemIDFromBlockName(info.Name)
	if itemID <= 0 {
		return nil
	}
	return []Drop{{ItemID: itemID, Count: 1}}
}

func (d *DefaultBehavior) OnUse(_ *Context) bool {
	return false
}

func (d *DefaultBehavior) OnNeighborUpdate(_ *Context) {}

func (d *DefaultBehavior) OnScheduledTick(_ *TickContext) *TickResult {
	return nil
}

func (d *DefaultBehavior) ScheduleOnPlace() bool {
	return false
}

func (d *DefaultBehavior) ScheduleOnNeighborUpdate() bool {
	return false
}

func (d *DefaultBehavior) TickDelay() int {
	return 0
}

var defaultBehavior = &DefaultBehavior{}

var registry = make(map[int32]Behavior)

// Register binds a Behavior to a block ID.
func Register(blockID int32, b Behavior) {
	registry[blockID] = b
}

// Get returns the behavior for a block ID, or the default behavior.
func Get(blockID int32) Behavior {
	if b, ok := registry[blockID]; ok {
		return b
	}
	return defaultBehavior
}

// GetByState returns the behavior for a block state ID.
func GetByState(stateID int32) Behavior {
	bid := block.BlockIDFromState(stateID)
	if bid < 0 {
		return defaultBehavior
	}
	return Get(bid)
}

func itemIDFromBlockName(name string) int32 {
	// Most block items share their name with the block.
	// Uses a simple lookup through the item registry.
	return itemIDByName(name)
}
