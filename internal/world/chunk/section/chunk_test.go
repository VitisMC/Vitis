package section

import (
	"testing"

	genblock "github.com/vitismc/vitis/internal/data/generated/block"
)

func TestNewChunk(t *testing.T) {
	c := NewChunk(0, 0, OverworldSections, 0)
	if len(c.Sections) != OverworldSections {
		t.Fatalf("expected %d sections, got %d", OverworldSections, len(c.Sections))
	}
	if got := c.GetBlock(0, 0, 0); got != 0 {
		t.Fatalf("expected air at (0,0,0), got %d", got)
	}
}

func TestChunkSetGetBlock(t *testing.T) {
	c := NewChunk(0, 0, OverworldSections, 0)
	c.SetBlock(0, 64, 0, genblock.StoneDefaultState)
	if got := c.GetBlock(0, 64, 0); got != genblock.StoneDefaultState {
		t.Fatalf("expected stone at (0,64,0), got %d", got)
	}
	if got := c.GetBlock(1, 64, 0); got != 0 {
		t.Fatalf("expected air at (1,64,0), got %d", got)
	}
}

func TestChunkSetBlockMinY(t *testing.T) {
	c := NewChunk(0, 0, OverworldSections, 0)
	c.SetBlock(0, OverworldMinY, 0, genblock.BedrockDefaultState)
	if got := c.GetBlock(0, OverworldMinY, 0); got != genblock.BedrockDefaultState {
		t.Fatalf("expected bedrock at minY, got %d", got)
	}
}

func TestChunkSetBlockMaxY(t *testing.T) {
	c := NewChunk(0, 0, OverworldSections, 0)
	c.SetBlock(0, OverworldMaxY, 0, genblock.StoneDefaultState)
	if got := c.GetBlock(0, OverworldMaxY, 0); got != genblock.StoneDefaultState {
		t.Fatalf("expected stone at maxY, got %d", got)
	}
}

func TestChunkOutOfBounds(t *testing.T) {
	c := NewChunk(0, 0, OverworldSections, 0)
	c.SetBlock(0, OverworldMinY-1, 0, 1)
	if got := c.GetBlock(0, OverworldMinY-1, 0); got != 0 {
		t.Fatalf("expected 0 for out-of-bounds, got %d", got)
	}
	c.SetBlock(0, OverworldMaxY+1, 0, 1)
	if got := c.GetBlock(0, OverworldMaxY+1, 0); got != 0 {
		t.Fatalf("expected 0 for out-of-bounds, got %d", got)
	}
}

func TestEncodeEmptyChunk(t *testing.T) {
	c := NewChunk(0, 0, OverworldSections, 0)
	payload := c.EncodePacketPayload()
	if len(payload) == 0 {
		t.Fatal("expected non-empty payload")
	}
	if len(payload) < 100 {
		t.Fatalf("payload too small: %d bytes", len(payload))
	}
}

func TestEncodeChunkWithBlocks(t *testing.T) {
	c := NewChunk(0, 0, OverworldSections, 0)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			c.SetBlock(x, -64, z, genblock.BedrockDefaultState)
			c.SetBlock(x, 64, z, genblock.GrassBlockDefaultState)
			for y := -63; y < 64; y++ {
				c.SetBlock(x, y, z, genblock.DirtDefaultState)
			}
		}
	}
	payload := c.EncodePacketPayload()
	if len(payload) == 0 {
		t.Fatal("expected non-empty payload")
	}
	t.Logf("chunk payload size: %d bytes", len(payload))
}

func TestEncodeSingleSection(t *testing.T) {
	s := NewSection(0, 0, 0)
	s.SetBlock(0, 0, 0, 1)

	data := EncodeSectionData(nil, s)
	if len(data) == 0 {
		t.Fatal("expected non-empty section data")
	}
}

func TestEncodeAllAirSection(t *testing.T) {
	s := NewSection(0, 0, 0)
	data := EncodeSectionData(nil, s)
	if len(data) < 4 {
		t.Fatalf("section data too small: %d bytes", len(data))
	}
	if data[0] != 0 || data[1] != 0 {
		t.Fatalf("expected blockCount=0, got %d,%d", data[0], data[1])
	}
}

func BenchmarkEncodeChunkPayload(b *testing.B) {
	c := NewChunk(0, 0, OverworldSections, 0)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			c.SetBlock(x, -64, z, genblock.BedrockDefaultState)
			for y := -63; y < 64; y++ {
				c.SetBlock(x, y, z, genblock.DirtDefaultState)
			}
			c.SetBlock(x, 64, z, genblock.GrassBlockDefaultState)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.EncodePacketPayload()
	}
}
