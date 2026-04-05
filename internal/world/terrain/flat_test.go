package terrain

import (
	"testing"

	genblock "github.com/vitismc/vitis/internal/data/generated/block"
	"github.com/vitismc/vitis/internal/world/chunk/section"
)

func TestFlatGeneratorDefault(t *testing.T) {
	gen := NewFlatGenerator(0)
	c := gen.Generate(0, 0)

	if c.GetBlock(0, section.OverworldMinY, 0) != genblock.BedrockDefaultState {
		t.Fatalf("expected bedrock at minY, got %d", c.GetBlock(0, section.OverworldMinY, 0))
	}
	if c.GetBlock(0, section.OverworldMinY+1, 0) != genblock.DirtDefaultState {
		t.Fatalf("expected dirt at minY+1, got %d", c.GetBlock(0, section.OverworldMinY+1, 0))
	}
	if c.GetBlock(0, section.OverworldMinY+2, 0) != genblock.DirtDefaultState {
		t.Fatalf("expected dirt at minY+2, got %d", c.GetBlock(0, section.OverworldMinY+2, 0))
	}
	if c.GetBlock(0, section.OverworldMinY+3, 0) != genblock.GrassBlockDefaultState {
		t.Fatalf("expected grass_block at minY+3, got %d", c.GetBlock(0, section.OverworldMinY+3, 0))
	}
	if c.GetBlock(0, section.OverworldMinY+4, 0) != 0 {
		t.Fatalf("expected air above layers, got %d", c.GetBlock(0, section.OverworldMinY+4, 0))
	}
}

func TestFlatGeneratorSpawnY(t *testing.T) {
	gen := NewFlatGenerator(0)
	y := gen.SpawnY()
	if y != section.OverworldMinY+4 {
		t.Fatalf("expected spawnY=%d, got %d", section.OverworldMinY+4, y)
	}
}

func TestFlatGeneratorAllPositions(t *testing.T) {
	gen := NewFlatGenerator(0)
	c := gen.Generate(3, -5)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			if c.GetBlock(x, section.OverworldMinY, z) != genblock.BedrockDefaultState {
				t.Fatalf("missing bedrock at (%d,minY,%d)", x, z)
			}
		}
	}
}

func TestFlatGeneratorEncode(t *testing.T) {
	gen := NewFlatGenerator(0)
	c := gen.Generate(0, 0)
	payload := c.EncodePacketPayload()
	if len(payload) == 0 {
		t.Fatal("expected non-empty payload")
	}
	t.Logf("flat chunk payload: %d bytes", len(payload))
}

func BenchmarkFlatGenerate(b *testing.B) {
	gen := NewFlatGenerator(0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.Generate(int32(i), 0)
	}
}
