package chunk

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/world/chunk/section"
)

func TestChunkNBTRoundtrip(t *testing.T) {
	c := section.NewChunk(3, 7, section.OverworldSections, 40)

	stoneID := block.DefaultStateID("minecraft:stone")
	dirtID := block.DefaultStateID("minecraft:dirt")
	grassID := block.DefaultStateID("minecraft:grass_block")
	if stoneID < 0 || dirtID < 0 || grassID < 0 {
		t.Fatal("required block states not found in registry")
	}

	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			for y := -64; y < 0; y++ {
				c.SetBlock(x, y, z, stoneID)
			}
			for y := 0; y < 60; y++ {
				c.SetBlock(x, y, z, dirtID)
			}
			c.SetBlock(x, 60, z, grassID)
		}
	}

	c.SetBlock(5, 70, 5, stoneID)
	c.SetBlock(0, -64, 0, dirtID)

	data, err := EncodeChunkNBT(c)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("encoded data is empty")
	}

	decoded, err := DecodeChunkNBT(data, 3, 7, 40)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if decoded == nil {
		t.Fatal("decoded chunk is nil")
	}

	tests := []struct {
		x, y, z int
		expect  int32
		name    string
	}{
		{0, -64, 0, dirtID, "bedrock-level dirt"},
		{5, -32, 5, stoneID, "stone underground"},
		{8, 30, 8, dirtID, "dirt mid-level"},
		{8, 60, 8, grassID, "grass surface"},
		{5, 70, 5, stoneID, "isolated stone"},
		{0, 100, 0, 0, "air above surface"},
	}

	for _, tt := range tests {
		got := decoded.GetBlock(tt.x, tt.y, tt.z)
		if got != tt.expect {
			t.Errorf("%s: GetBlock(%d,%d,%d) = %d, want %d",
				tt.name, tt.x, tt.y, tt.z, got, tt.expect)
		}
	}
}

func TestChunkNBTRoundtripEmpty(t *testing.T) {
	c := section.NewChunk(0, 0, section.OverworldSections, 40)

	data, err := EncodeChunkNBT(c)
	if err != nil {
		t.Fatalf("encode empty chunk failed: %v", err)
	}

	decoded, err := DecodeChunkNBT(data, 0, 0, 40)
	if err != nil {
		t.Fatalf("decode empty chunk failed: %v", err)
	}

	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			got := decoded.GetBlock(x, 64, z)
			if got != 0 {
				t.Errorf("expected air at (%d,64,%d), got %d", x, z, got)
			}
		}
	}
}

func TestChunkNBTRoundtripManyBlockTypes(t *testing.T) {
	c := section.NewChunk(1, 1, section.OverworldSections, 40)

	blockNames := []string{
		"minecraft:stone", "minecraft:dirt", "minecraft:cobblestone",
		"minecraft:oak_planks", "minecraft:sand", "minecraft:gravel",
		"minecraft:gold_ore", "minecraft:iron_ore", "minecraft:coal_ore",
		"minecraft:oak_log", "minecraft:glass", "minecraft:lapis_ore",
		"minecraft:sandstone", "minecraft:white_wool", "minecraft:bricks",
		"minecraft:bookshelf",
	}

	stateIDs := make([]int32, len(blockNames))
	for i, name := range blockNames {
		sid := block.DefaultStateID(name)
		if sid < 0 {
			t.Fatalf("block %s not found", name)
		}
		stateIDs[i] = sid
	}

	for i, sid := range stateIDs {
		c.SetBlock(i, 64, 0, sid)
	}

	data, err := EncodeChunkNBT(c)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := DecodeChunkNBT(data, 1, 1, 40)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	for i, sid := range stateIDs {
		got := decoded.GetBlock(i, 64, 0)
		if got != sid {
			t.Errorf("block %d (%s): got state %d, want %d",
				i, blockNames[i], got, sid)
		}
	}
}
