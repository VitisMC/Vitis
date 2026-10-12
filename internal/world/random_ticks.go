package world

import (
	"strconv"

	"github.com/vitismc/vitis/internal/block/behavior"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	"github.com/vitismc/vitis/internal/world/worldgen/random"
)

// initRandomTicks isolates gameplay randomness from concurrent world generation.
func (w *World) initRandomTicks(seed int64) {
	w.randomTicks = random.NewLegacy(seed)
	w.randomTickPosition = w.randomTicks.NextInt()
}

// LookupBlockState distinguishes resident blocks from unavailable chunk data.
func (w *World) LookupBlockState(x, y, z int) (int32, bool) {
	if w == nil || w.chunks == nil {
		return 0, false
	}
	c, ok := w.chunks.PeekLoaded(int32(x>>4), int32(z>>4))
	if !ok || y < c.Data().MinY() || y > c.Data().MaxY() {
		return 0, false
	}
	return c.GetBlock(x, y, z), true
}

// RawBrightness returns available local light without applying daylight darkening.
func (w *World) RawBrightness(x, y, z int) uint8 {
	c, ok := w.chunks.PeekLoaded(int32(x>>4), int32(z>>4))
	if !ok || y < c.Data().MinY() || y > c.Data().MaxY() {
		return 0
	}
	c.EnsureLight()
	return c.Data().LightEngine.RawBrightness(x&15, y, z&15, w.dimension != "minecraft:the_nether" && w.dimension != "minecraft:the_end")
}

// LightLevels reads resident block and sky light without loading chunks.
func (w *World) LightLevels(x, y, z int) (uint8, uint8, bool) {
	c, ok := w.chunks.PeekLoaded(int32(x>>4), int32(z>>4))
	if !ok || y < c.Data().MinY() || y > c.Data().MaxY() {
		return 0, 0, false
	}
	c.EnsureLight()
	b, s := c.Data().LightEngine.Levels(x&15, y, z&15)
	if w.dimension == "minecraft:the_nether" || w.dimension == "minecraft:the_end" {
		s = 0
	}
	return b, s, true
}

// tickRandomBlocks samples eligible sections using the world's randomTickSpeed rule.
func (w *World) tickRandomBlocks() {
	speed, err := strconv.Atoi(w.GameRule("randomTickSpeed"))
	if err != nil || speed <= 0 {
		return
	}
	w.chunks.VisitLoaded(func(c *chunk.Chunk) {
		if !w.IsBlockTicking(c.X(), c.Z()) {
			return
		}
		for _, s := range c.Sections() {
			if !hasRandomTicks(s) {
				continue
			}
			for range speed {
				w.randomTickPosition = w.randomTickPosition*3 + 1013904223
				bits := w.randomTickPosition >> 2
				x, y, z := int(bits&15), int(bits>>16&15), int(bits>>8&15)
				state := s.GetBlock(x, y, z)
				b, ok := behavior.GetByState(state).(behavior.RandomTickBehavior)
				if !ok || !b.CanRandomTick(state) {
					continue
				}
				x, y, z = int(c.X())*16+x, int(s.Y)*16+y, int(c.Z())*16+z
				result := b.OnRandomTick(&behavior.RandomTickContext{
					X: x, Y: y, Z: z, StateID: state, Light: w.RawBrightness(x, y, z),
					GetBlock: w.LookupBlockState, RandomBound: w.randomTicks.NextIntBound,
				})
				if result != nil && result.SetBlock && result.NewBlockState != state {
					w.setBlockAt(x, y, z, result.NewBlockState)
					w.broadcastBlockUpdate(x, y, z, result.NewBlockState)
				}
			}
		}
	})
}

// hasRandomTicks skips section sampling when no implemented random behavior is present.
func hasRandomTicks(s *section.Section) bool {
	states := s.Blocks.Palette()
	if states == nil {
		for index := 0; index < section.BlocksPerSection; index++ {
			state := s.Blocks.Get(index)
			if b, ok := behavior.GetByState(state).(behavior.RandomTickBehavior); ok && b.CanRandomTick(state) {
				return true
			}
		}
		return false
	}
	for _, state := range states {
		if b, ok := behavior.GetByState(state).(behavior.RandomTickBehavior); ok && b.CanRandomTick(state) {
			return true
		}
	}
	return false
}
