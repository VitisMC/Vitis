package lighting

import (
	"reflect"
	"sync"
	"testing"

	genblock "github.com/vitismc/vitis/internal/data/generated/block"
)

// TestLightingScratchIsolation verifies that sequential and concurrent jobs own their light output.
func TestLightingScratchIsolation(t *testing.T) {
	chunk := newMockChunk(-16, 15)
	chunk.blocks[[3]int{7, -8, 7}] = findEmittingBlock(15)
	reference := NewEngine(2, -16, 15)
	reference.ComputeBlockLight(chunk)
	reference.ComputeSkyLight(chunk)
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 4 {
				engine := NewEngine(2, -16, 15)
				engine.ComputeSkyLight(chunk)
				engine.ComputeBlockLight(chunk)
				if !reflect.DeepEqual(engine.BlockLightSections, reference.BlockLightSections) || !reflect.DeepEqual(engine.SkyLightSections, reference.SkyLightSections) {
					t.Error("reused propagation scratch changed lighting")
					return
				}
			}
		}()
	}
	workers.Wait()
}

// TestSparseEngineMatchesDense checks light values and recomputation with absent sections.
func TestSparseEngineMatchesDense(t *testing.T) {
	chunk := newMockChunk(-32, 63)
	emitter := findEmittingBlock(15)
	solid := findOpaqueBlock()
	if emitter == 0 || solid == 0 {
		t.Skip("required block traits are unavailable")
	}
	chunk.blocks[[3]int{8, -16, 8}] = emitter
	chunk.blocks[[3]int{0, 20, 0}] = solid
	dense := NewEngine(chunk.numSections, chunk.minY, chunk.maxY)
	sparse := NewSparseEngine(chunk.numSections, chunk.minY, chunk.maxY)
	for pass := 0; pass < 2; pass++ {
		dense.ComputeAll(chunk)
		sparse.ComputeAll(chunk)
		for section := range dense.BlockLightSections {
			for _, pair := range [][2]*NibbleArray{{dense.BlockLightSections[section], sparse.BlockLightSections[section]}, {dense.SkyLightSections[section], sparse.SkyLightSections[section]}} {
				if pair[1] == nil {
					if !pair[0].IsEmpty() {
						t.Fatalf("missing nonempty sparse light section %d", section)
					}
				} else if *pair[0] != *pair[1] {
					t.Fatalf("light differs in section %d on pass %d", section, pass)
				}
			}
		}
		delete(chunk.blocks, [3]int{8, -16, 8})
		delete(chunk.blocks, [3]int{0, 20, 0})
	}
	if sparse.BlockLightSections[0] != nil {
		t.Fatal("unused sparse section was allocated")
	}
}

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

type paletteMockChunk struct {
	*mockChunk
	palettes  [][]int32
	reads     int
	darkReads int
}

func (m *paletteMockChunk) BlockPalette(section int) []int32 {
	return m.palettes[section]
}

func (m *paletteMockChunk) GetBlock(x, y, z int) int32 {
	m.reads++
	if y >= 16 {
		m.darkReads++
	}
	return m.mockChunk.GetBlock(x, y, z)
}

func TestEngineBlockLightSkipsSectionsWithoutEmitters(t *testing.T) {
	emitter := findEmittingBlock(15)
	if emitter == 0 {
		t.Skip("no block with emission=15 found in registry")
	}
	chunk := &paletteMockChunk{
		mockChunk: newMockChunk(0, 31),
		palettes:  [][]int32{{0, emitter}, {0}},
	}
	chunk.blocks[[3]int{8, 0, 8}] = emitter

	engine := NewEngine(2, 0, 31)
	engine.ComputeBlockLight(chunk)
	if got := engine.getBlockLight(8, 0, 8); got != 15 {
		t.Fatalf("emitter light = %d, want 15", got)
	}
	if chunk.darkReads != 0 {
		t.Fatalf("read %d blocks in dark section", chunk.darkReads)
	}
}

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
	roof := eng.getSkyLight(8, 20, 8)
	if roof != 0 {
		t.Errorf("sky light inside opaque block = %d, want 0", roof)
	}

	below := eng.getSkyLight(8, 19, 8)
	if below >= maxLight {
		t.Errorf("sky light below solid layer = %d, should be < %d", below, maxLight)
	}
}

func TestEngineSkyLightFillsSectionsAboveTerrain(t *testing.T) {
	chunk := newMockChunk(-64, 63)
	solidState := findOpaqueBlock()
	if solidState == 0 {
		t.Skip("no opaque block found in registry")
	}
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			chunk.blocks[[3]int{x, 0, z}] = solidState
		}
	}
	engine := NewEngine(chunk.numSections, chunk.minY, chunk.maxY)
	engine.ComputeSkyLight(chunk)
	for _, y := range []int{1, 15, 16, 31, 32, 63} {
		if got := engine.getSkyLight(8, y, 8); got != maxLight {
			t.Fatalf("sky light at y=%d = %d, want %d", y, got, maxLight)
		}
	}
	if got := engine.getSkyLight(8, 0, 8); got != 0 {
		t.Fatalf("opaque roof light = %d, want 0", got)
	}
}

func TestEngineSkyLightSkipsTransparentSections(t *testing.T) {
	solidState := findOpaqueBlock()
	if solidState == 0 {
		t.Skip("no opaque block found in registry")
	}
	chunk := &paletteMockChunk{
		mockChunk: newMockChunk(0, 47),
		palettes:  [][]int32{{0, solidState}, {0}, {0}},
	}
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			chunk.blocks[[3]int{x, 0, z}] = solidState
		}
	}
	engine := NewEngine(chunk.numSections, chunk.minY, chunk.maxY)
	engine.ComputeSkyLight(chunk)
	if chunk.darkReads != 0 {
		t.Fatalf("read %d blocks in transparent sections", chunk.darkReads)
	}
	if got := engine.getSkyLight(8, 47, 8); got != maxLight {
		t.Fatalf("sky light above terrain = %d, want %d", got, maxLight)
	}
}

func TestEngineSkyLightPaletteScanMatchesDirectScan(t *testing.T) {
	solidState := findOpaqueBlock()
	if solidState == 0 {
		t.Skip("no opaque block found in registry")
	}
	chunk := newMockChunk(-16, 47)
	chunk.blocks[[3]int{0, -1, 0}] = solidState
	chunk.blocks[[3]int{8, 3, 8}] = solidState
	chunk.blocks[[3]int{15, 31, 15}] = solidState
	withPalette := &paletteMockChunk{
		mockChunk: chunk,
		palettes:  [][]int32{{0, solidState}, {0, solidState}, {0, solidState}, {0}},
	}
	direct := NewEngine(chunk.numSections, chunk.minY, chunk.maxY)
	fast := NewEngine(chunk.numSections, chunk.minY, chunk.maxY)
	direct.ComputeSkyLight(chunk)
	fast.ComputeSkyLight(withPalette)
	for section := range direct.SkyLightSections {
		if *direct.SkyLightSections[section] != *fast.SkyLightSections[section] {
			t.Fatalf("sky light differs in section %d", section)
		}
	}
}

func TestEngineSkyLightRecomputeRemovesObsoleteLight(t *testing.T) {
	mc := newMockChunk(0, 31)
	eng := NewEngine(mc.numSections, mc.minY, mc.maxY)
	eng.ComputeSkyLight(mc)
	if level := eng.getSkyLight(8, 19, 8); level != maxLight {
		t.Fatalf("open column sky light = %d, want %d", level, maxLight)
	}

	solidState := findOpaqueBlock()
	if solidState == 0 {
		t.Skip("no opaque block found in registry")
	}
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			mc.blocks[[3]int{x, 20, z}] = solidState
		}
	}
	eng.ComputeSkyLight(mc)
	if level := eng.getSkyLight(8, 20, 8); level != 0 {
		t.Errorf("opaque roof sky light = %d, want 0", level)
	}
	if level := eng.getSkyLight(8, 19, 8); level >= maxLight {
		t.Errorf("sky light below new roof = %d, want below %d", level, maxLight)
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
