package terrain

import (
	genblock "github.com/vitismc/vitis/internal/data/generated/block"
	"github.com/vitismc/vitis/internal/world/chunk/section"
)

// FlatGenerator produces flat world chunks with configurable layers.
type FlatGenerator struct {
	layers  []Layer
	biomeID int32
}

// Layer defines a horizontal layer of blocks in a flat world.
type Layer struct {
	StateID int32
	Height  int
}

// NewFlatGenerator creates a flat world generator with default layers:
// 1 bedrock, 3 dirt, 1 grass_block (Y=-64 to Y=-60).
func NewFlatGenerator(biomeID int32) *FlatGenerator {
	return &FlatGenerator{
		layers: []Layer{
			{StateID: genblock.BedrockDefaultState, Height: 1},
			{StateID: genblock.DirtDefaultState, Height: 2},
			{StateID: genblock.GrassBlockDefaultState, Height: 1},
		},
		biomeID: biomeID,
	}
}

// NewFlatGeneratorWithLayers creates a flat world generator with custom layers.
func NewFlatGeneratorWithLayers(biomeID int32, layers []Layer) *FlatGenerator {
	return &FlatGenerator{
		layers:  layers,
		biomeID: biomeID,
	}
}

// Generate produces a flat chunk at the given chunk coordinates.
func (g *FlatGenerator) Generate(cx, cz int32) *section.Chunk {
	c := section.NewChunk(cx, cz, section.OverworldSections, g.biomeID)

	y := section.OverworldMinY
	for _, layer := range g.layers {
		for i := 0; i < layer.Height; i++ {
			if y > section.OverworldMaxY {
				return c
			}
			for x := 0; x < 16; x++ {
				for z := 0; z < 16; z++ {
					c.SetBlock(x, y, z, layer.StateID)
				}
			}
			y++
		}
	}
	return c
}

// SpawnY returns the Y coordinate above the top layer.
func (g *FlatGenerator) SpawnY() int {
	total := 0
	for _, l := range g.layers {
		total += l.Height
	}
	return section.OverworldMinY + total
}
