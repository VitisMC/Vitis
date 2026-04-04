package loot

import (
	"math/rand"
)

type Context struct {
	Rng             *rand.Rand
	Tool            *ToolInfo
	ExplosionRadius float64
	BlockState      map[string]string
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
