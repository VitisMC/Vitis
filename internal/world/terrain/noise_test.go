package terrain

import (
	"testing"

	"github.com/vitismc/vitis/internal/world/chunk/section"
)

func TestNoiseGeneratorProducesTerrain(t *testing.T) {
	gen := NewNoiseGenerator(12345, 0)
	c := gen.Generate(0, 0)

	hasStone := false
	hasDirt := false
	hasGrass := false
	for y := section.OverworldMinY; y <= section.OverworldMaxY; y++ {
		b := c.GetBlock(8, y, 8)
		if b != 0 {
			switch {
			case b == 1:
				hasStone = true
			case b > 0:
				hasDirt = true
				hasGrass = true
			}
		}
	}
	if !hasStone {
		t.Fatal("expected stone in chunk")
	}
	if !hasDirt || !hasGrass {
		t.Fatal("expected dirt/grass in chunk")
	}
}

func TestNoiseGeneratorBedrock(t *testing.T) {
	gen := NewNoiseGenerator(42, 0)
	c := gen.Generate(5, -3)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			b := c.GetBlock(x, section.OverworldMinY, z)
			if b == 0 {
				t.Fatalf("expected bedrock at (%d,minY,%d), got air", x, z)
			}
		}
	}
}

func TestNoiseGeneratorDeterministic(t *testing.T) {
	gen := NewNoiseGenerator(999, 0)
	c1 := gen.Generate(0, 0)
	c2 := gen.Generate(0, 0)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			for y := section.OverworldMinY; y <= 100; y++ {
				if c1.GetBlock(x, y, z) != c2.GetBlock(x, y, z) {
					t.Fatalf("non-deterministic at (%d,%d,%d)", x, y, z)
				}
			}
		}
	}
}

func TestNoiseGeneratorDifferentChunks(t *testing.T) {
	gen := NewNoiseGenerator(42, 0)
	c1 := gen.Generate(0, 0)
	c2 := gen.Generate(10, 10)
	same := true
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			if c1.GetBlock(x, 70, z) != c2.GetBlock(x, 70, z) {
				same = false
				break
			}
		}
	}
	if same {
		t.Fatal("different chunks should have different terrain")
	}
}

func TestNoiseGeneratorSpawnY(t *testing.T) {
	gen := NewNoiseGenerator(42, 0)
	y := gen.SpawnY()
	if y < section.OverworldMinY || y > section.OverworldMaxY {
		t.Fatalf("spawnY %d out of world range", y)
	}
}

func TestNoiseGeneratorEncode(t *testing.T) {
	gen := NewNoiseGenerator(42, 0)
	c := gen.Generate(0, 0)
	payload := c.EncodePacketPayload()
	if len(payload) == 0 {
		t.Fatal("expected non-empty payload")
	}
	t.Logf("noise chunk payload: %d bytes", len(payload))
}

func BenchmarkNoiseGenerate(b *testing.B) {
	gen := NewNoiseGenerator(42, 0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.Generate(int32(i), 0)
	}
}
