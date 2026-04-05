package lighting

import (
	"testing"

	genblock "github.com/vitismc/vitis/internal/data/generated/block"
)

type mockChunk struct {
	blocks      map[[3]int]int32
	minY        int
	maxY        int
	numSections int
}

func newMockChunk(minY, maxY int) *mockChunk {
	return &mockChunk{
		blocks:      make(map[[3]int]int32),
		minY:        minY,
		maxY:        maxY,
		numSections: (maxY - minY + 1) / 16,
	}
}

func (m *mockChunk) GetBlock(x, y, z int) int32 {
	return m.blocks[[3]int{x, y, z}]
}

func (m *mockChunk) MinY() int        { return m.minY }
func (m *mockChunk) MaxY() int        { return m.maxY }
func (m *mockChunk) NumSections() int { return m.numSections }

func TestEngineBlockLightTorch(t *testing.T) {
	mc := newMockChunk(0, 15)

	torchState := findEmittingBlock(14)
	if torchState == 0 {
		t.Skip("no block with emission=14 found in registry")
	}

	mc.blocks[[3]int{8, 5, 8}] = torchState

	eng := NewEngine(mc.numSections, mc.minY, mc.maxY)
	eng.ComputeBlockLight(mc)

	center := eng.getBlockLight(8, 5, 8)
	if center != 14 {
		t.Errorf("block light at torch = %d, want 14", center)
	}

	adj := eng.getBlockLight(9, 5, 8)
	if adj < 12 || adj > 13 {
		t.Errorf("block light adjacent to torch = %d, want 12-13", adj)
	}

	far := eng.getBlockLight(0, 5, 8)
	if far >= center {
		t.Errorf("block light far from torch (%d) should be less than center (%d)", far, center)
	}
}

func TestEngineBlockLightGlowstone(t *testing.T) {
	mc := newMockChunk(0, 15)

	glowstoneState := findEmittingBlock(15)
	if glowstoneState == 0 {
		t.Skip("no block with emission=15 found in registry")
	}

	mc.blocks[[3]int{8, 5, 8}] = glowstoneState

	eng := NewEngine(mc.numSections, mc.minY, mc.maxY)
	eng.ComputeBlockLight(mc)

	center := eng.getBlockLight(8, 5, 8)
	if center != 15 {
		t.Errorf("block light at glowstone = %d, want 15", center)
	}
}

func TestEngineSkyLightOpenAir(t *testing.T) {
	mc := newMockChunk(0, 15)

	eng := NewEngine(mc.numSections, mc.minY, mc.maxY)
	eng.ComputeSkyLight(mc)

	for y := mc.minY; y <= mc.maxY; y++ {
		level := eng.getSkyLight(8, y, 8)
		if level != maxLight {
			t.Errorf("sky light at y=%d in open air = %d, want %d", y, level, maxLight)
			break
		}
	}
}

func TestEngineSkyLightBlockedBySolid(t *testing.T) {
	mc := newMockChunk(0, 31)

	solidState := findOpaqueBlock()
	if solidState == 0 {
		t.Skip("no opaque block found in registry")
	}

	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			mc.blocks[[3]int{x, 20, z}] = solidState
		}
	}

	eng := NewEngine(mc.numSections, mc.minY, mc.maxY)
	eng.ComputeSkyLight(mc)

	above := eng.getSkyLight(8, 21, 8)
	if above != maxLight {
		t.Errorf("sky light above solid layer = %d, want %d", above, maxLight)
	}

	below := eng.getSkyLight(8, 19, 8)
	if below >= maxLight {
		t.Errorf("sky light below solid layer = %d, should be < %d", below, maxLight)
	}
}

func TestEngineComputeAll(t *testing.T) {
	mc := newMockChunk(0, 15)

	eng := NewEngine(mc.numSections, mc.minY, mc.maxY)
	eng.ComputeAll(mc)

	sky := eng.getSkyLight(8, 8, 8)
	if sky == 0 {
		t.Error("sky light should be non-zero in open air after ComputeAll")
	}
}

func findEmittingBlock(emission int) int32 {
	for _, b := range genblock.Blocks {
		if int(b.EmitLight) == emission {
			return b.DefaultState
		}
	}
	return 0
}

func findOpaqueBlock() int32 {
	for _, b := range genblock.Blocks {
		if b.FilterLight == 15 && b.Solid {
			return b.DefaultState
		}
	}
	return 0
}
