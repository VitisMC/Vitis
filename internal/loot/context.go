package loot

import (
	"math/rand"
)

// Context supplies detached gameplay inputs and snapshot-bound resource resolvers.
type Context struct {
	Rng                 *rand.Rand
	Tool                *ToolInfo
	ExplosionRadius     float64
	BlockState          map[string]string
	Luck                float64
	OpenWater           bool
	KilledByPlayer      bool
	Entity              *EntityInfo
	Attacker            *EntityInfo
	DirectAttacker      *EntityInfo
	DamageType          string
	DamageIsDirect      *bool
	tableResolver       func(string, *Context) []Drop
	tagResolver         func(string) []string
	depth               int
	fullStacks          bool
	registryTagResolver func(string, string) []string
	smeltResolver       func(string) string
}

// EntityInfo is a detached view of entity state used by death predicates.
type EntityInfo struct {
	Type     string
	OnFire   bool
	Baby     bool
	Color    string
	Sheared  bool
	Size     int
	Captain  bool
	Mainhand *ToolInfo
}

// WithLuck sets the luck attribute used by weighted loot entries.
func (c *Context) WithLuck(luck float64) *Context {
	c.Luck = luck
	return c
}

// WithOpenWater sets whether a fishing hook satisfies open-water predicates.
func (c *Context) WithOpenWater(open bool) *Context {
	c.OpenWater = open
	return c
}

type ToolInfo struct {
	ItemID       string
	Enchantments map[string]int
}

func NewContext(rng *rand.Rand) *Context {
	if rng == nil {
		rng = rand.New(rand.NewSource(rand.Int63()))
	}
	return &Context{
		Rng:        rng,
		BlockState: make(map[string]string),
	}
}

func (c *Context) WithTool(itemID string, enchantments map[string]int) *Context {
	c.Tool = &ToolInfo{
		ItemID:       itemID,
		Enchantments: enchantments,
	}
	return c
}

func (c *Context) WithExplosion(radius float64) *Context {
	c.ExplosionRadius = radius
	return c
}

func (c *Context) WithBlockState(state map[string]string) *Context {
	c.BlockState = state
	return c
}

func (c *Context) HasSilkTouch() bool {
	if c.Tool == nil {
		return false
	}
	level, ok := c.Tool.Enchantments["minecraft:silk_touch"]
	return ok && level >= 1
}

func (c *Context) FortuneLevel() int {
	if c.Tool == nil {
		return 0
	}
	return c.Tool.Enchantments["minecraft:fortune"]
}

func (c *Context) EnchantmentLevel(enchantment string) int {
	if c.Tool == nil {
		return 0
	}
	return c.Tool.Enchantments[enchantment]
}
