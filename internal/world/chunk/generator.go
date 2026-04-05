package chunk

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

// Generate returns a deterministic generated chunk with simplified section data.
func (g *StubGenerator) Generate(x, z int32) (*Chunk, error) {
	sectionCount := defaultGeneratedSectionCount
	if g != nil && g.SectionCount > 0 {
		sectionCount = g.SectionCount
	}

	blockValue := defaultGeneratedBlockValue
	if g != nil && g.BlockValue != 0 {
		blockValue = g.BlockValue
	}

	sections := make([]Section, sectionCount)
	for i := 0; i < sectionCount; i++ {
		blocks := make([]uint16, defaultSectionSize)
		for index := range blocks {
			blocks[index] = blockValue
		}
		sections[i] = Section{
			Y:      int32(i),
			Blocks: blocks,
		}
	}

	generated := New(x, z)
	generated.SetSections(sections)
	generated.SetState(StateLoaded)
	return generated, nil
}
