package chunk

import (
	"fmt"

	"github.com/vitismc/vitis/internal/world/chunk/section"
)

type sectionGenerator interface {
	Generate(chunkX, chunkZ int32) (*section.Chunk, error)
}

// WorldgenGenerator adapts a section generator to the chunk lifecycle.
type WorldgenGenerator struct {
	source sectionGenerator
}

// NewWorldgenGenerator creates a lifecycle adapter for a worldgen source.
func NewWorldgenGenerator(source sectionGenerator) (*WorldgenGenerator, error) {
	if source == nil {
		return nil, fmt.Errorf("create worldgen generator: nil source")
	}
	return &WorldgenGenerator{source: source}, nil
}

// Generate creates a loaded chunk with lighting computed from generated states.
func (g *WorldgenGenerator) Generate(x, z int32) (*Chunk, error) {
	if g == nil || g.source == nil {
		return nil, fmt.Errorf("generate worldgen chunk: nil source")
	}
	gameData, err := g.source.Generate(x, z)
	if err != nil {
		return nil, err
	}
	if gameData == nil {
		return nil, fmt.Errorf("generate worldgen chunk: empty section data")
	}
	result := New(x, z)
	result.SetData(gameData)
	gameData.ComputeLight()
	result.MarkDirty()
	result.SetState(StateLoaded)
	return result, nil
}

// Bounds exposes the source dimension's vertical range when available.
func (g *WorldgenGenerator) Bounds() (int, int, error) {
	if g == nil || g.source == nil {
		return 0, 0, fmt.Errorf("worldgen source is unavailable")
	}
	source, ok := g.source.(interface{ Bounds() (int, int, error) })
	if !ok {
		return 0, 0, fmt.Errorf("worldgen source has no dimension bounds")
	}
	return source.Bounds()
}
