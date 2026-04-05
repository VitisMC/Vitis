package chunk

import "testing"

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
		if len(sectionA.Blocks) == 0 || len(sectionB.Blocks) == 0 {
			t.Fatalf("expected non-empty blocks")
		}
		if sectionA.Blocks[0] != 7 || sectionB.Blocks[0] != 7 {
			t.Fatalf("expected deterministic fill block value")
		}
	}
}
