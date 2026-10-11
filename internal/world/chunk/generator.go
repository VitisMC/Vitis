package chunk

import "github.com/vitismc/vitis/internal/world/chunk/section"

const (
	defaultGeneratedSectionCount = 1
	defaultGeneratedBlockValue   = uint16(1)
)

// Generator produces chunk data off the world tick thread.
type Generator interface {
	Generate(x, z int32) (*Chunk, error)
}

// StubGenerator produces deterministic stub chunks for load/generation pipeline testing.
type StubGenerator struct {
	SectionCount int
	BlockValue   uint16
}

// NewStubGenerator creates a deterministic stub chunk generator.
func NewStubGenerator() *StubGenerator {
	return &StubGenerator{}
}

// Generate returns a deterministic generated palette-based chunk.
func (g *StubGenerator) Generate(x, z int32) (*Chunk, error) {
	sectionCount := defaultGeneratedSectionCount
	if g != nil && g.SectionCount > 0 {
		sectionCount = g.SectionCount
	}

	blockValue := defaultGeneratedBlockValue
	if g != nil && g.BlockValue != 0 {
		blockValue = g.BlockValue
	}

	data := section.NewChunk(x, z, sectionCount, 0)
	for _, chunkSection := range data.Sections {
		for index := 0; index < section.BlocksPerSection; index++ {
			chunkSection.Blocks.Set(index, int32(blockValue))
		}
	}

	generated := New(x, z)
	generated.SetData(data)
	generated.SetState(StateLoaded)
	return generated, nil
}
