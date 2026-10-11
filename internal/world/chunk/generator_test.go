package chunk

import (
	"testing"

	"github.com/vitismc/vitis/internal/world/chunk/section"
)

type testSectionGenerator struct{}

func (testSectionGenerator) Generate(x, z int32) (*section.Chunk, error) {
	return section.NewChunk(x, z, section.OverworldSections, 0), nil
}

func TestWorldgenGeneratorMarksNewChunkForPersistence(t *testing.T) {
	generator, err := NewWorldgenGenerator(testSectionGenerator{})
	if err != nil {
		t.Fatal(err)
	}
	generated, err := generator.Generate(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !generated.Dirty() {
		t.Fatal("newly generated chunk is not scheduled for persistence")
	}
}

func TestStubGeneratorDeterministicOutput(t *testing.T) {
	generator := &StubGenerator{SectionCount: 2, BlockValue: 7}

	first, err := generator.Generate(8, 9)
	if err != nil {
		t.Fatalf("generate first chunk failed: %v", err)
	}
	second, err := generator.Generate(8, 9)
	if err != nil {
		t.Fatalf("generate second chunk failed: %v", err)
	}

	if first.X() != second.X() || first.Z() != second.Z() {
		t.Fatalf("expected deterministic coordinates")
	}
	if len(first.Sections()) != 2 || len(second.Sections()) != 2 {
		t.Fatalf("expected deterministic section count")
	}

	for index := range first.Sections() {
		sectionA := first.Sections()[index]
		sectionB := second.Sections()[index]
		if sectionA.Y != sectionB.Y {
			t.Fatalf("expected deterministic section y")
		}
		if sectionA.Blocks.Get(0) != 7 || sectionB.Blocks.Get(0) != 7 {
			t.Fatalf("expected deterministic fill block value")
		}
	}
}
