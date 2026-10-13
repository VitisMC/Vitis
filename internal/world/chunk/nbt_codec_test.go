package chunk

import (
	"errors"
	"strings"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	genblock "github.com/vitismc/vitis/internal/data/generated/block"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	"github.com/vitismc/vitis/internal/world/persistence"
)

// TestBlockStateToNBTMatchesRegistry checks every generated block state.
func TestBlockStateToNBTMatchesRegistry(t *testing.T) {
	for stateID := int32(0); stateID < genblock.TotalStates; stateID++ {
		encoded := blockStateToNBT(stateID)
		if got, _ := encoded.GetString("Name"); got != block.NameFromState(stateID) {
			t.Fatalf("state %d name = %q", stateID, got)
		}
		want := block.PropertiesFromState(stateID)
		properties, hasProperties := encoded.GetCompound("Properties")
		if hasProperties != (len(want) != 0) {
			t.Fatalf("state %d properties presence = %t", stateID, hasProperties)
		}
		for name, value := range want {
			if got, ok := properties.GetString(name); !ok || got != value {
				t.Fatalf("state %d property %s = %q, %t; want %q", stateID, name, got, ok, value)
			}
		}
	}
	for _, stateID := range []int32{-1, genblock.TotalStates} {
		encoded := blockStateToNBT(stateID)
		if name, ok := encoded.GetString("Name"); !ok || name != "minecraft:air" {
			t.Fatalf("unknown state %d name = %q, %t", stateID, name, ok)
		}
		if _, ok := encoded.GetCompound("Properties"); ok {
			t.Fatalf("unknown state %d has properties", stateID)
		}
	}
}

type testBoundsFallback struct{ minY, height int }

// Bounds reports the expected vertical range for stored chunks.
func (f testBoundsFallback) Bounds() (int, int, error) { return f.minY, f.height, nil }

// Generate fails when a stored chunk unexpectedly falls back to generation.
func (testBoundsFallback) Generate(int32, int32) (*Chunk, error) {
	return nil, errors.New("fallback must not generate a stored chunk")
}

func newCodecTestChunk(x, z, biomeID int32) *Chunk {
	c := New(x, z)
	c.SetData(section.NewChunk(x, z, section.OverworldSections, biomeID))
	return c
}

// TestChunkNBTRoundtripStructureData preserves starts and references across saves.
func TestChunkNBTRoundtripStructureData(t *testing.T) {
	original := newCodecTestChunk(-2, 3, 0)
	children := nbt.NewList(nbt.TagCompound)
	children.Add(nbt.NewCompound().PutString("id", "minecraft:btp").PutIntArray("BB", []int32{-23, 61, 57, -23, 61, 57}))
	structures := nbt.NewCompound().
		PutCompound("starts", nbt.NewCompound().PutCompound("minecraft:buried_treasure", nbt.NewCompound().
			PutString("id", "minecraft:buried_treasure").PutInt("ChunkX", -2).PutInt("ChunkZ", 3).PutList("Children", children))).
		PutCompound("References", nbt.NewCompound().PutLongArray("minecraft:buried_treasure", []int64{0x00000003fffffffe}))
	original.Data().SetPersistentNBT(nbt.NewCompound().PutCompound("structures", structures))
	encoded, err := EncodeChunkNBT(original)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := DecodeChunkNBT(encoded, -2, 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	encodedAgain, err := EncodeChunkNBT(loaded)
	if err != nil {
		t.Fatal(err)
	}
	_, root, err := nbt.NewDecoder(encodedAgain).ReadNamedRootCompound()
	if err != nil {
		t.Fatal(err)
	}
	stored, ok := root.GetCompound("structures")
	if !ok {
		t.Fatal("structure metadata was lost")
	}
	starts, _ := stored.GetCompound("starts")
	start, ok := starts.GetCompound("minecraft:buried_treasure")
	if !ok {
		t.Fatal("structure start was lost")
	}
	pieces, ok := start.GetList("Children")
	if !ok || pieces.Len() != 1 {
		t.Fatal("structure piece was lost")
	}
	references, _ := stored.GetCompound("References")
	if value, kind, ok := references.Get("minecraft:buried_treasure"); !ok || kind != nbt.TagLongArray || len(value.([]int64)) != 1 {
		t.Fatal("structure reference was lost")
	}
}

// TestChunkNBTRoundtripDimensionBounds verifies non-Overworld section ranges.
func TestChunkNBTRoundtripDimensionBounds(t *testing.T) {
	for _, bounds := range []struct {
		name         string
		minY, height int
	}{
		{name: "nether", minY: 0, height: 128},
		{name: "end", minY: 0, height: 256},
		{name: "custom", minY: -32, height: 128},
	} {
		t.Run(bounds.name, func(t *testing.T) {
			original := New(3, -2)
			original.SetData(section.NewChunkWithBounds(3, -2, bounds.minY, bounds.height, 40))
			stone := block.DefaultStateID("minecraft:stone")
			original.SetBlock(1, bounds.minY, 2, stone)
			original.SetBlock(3, bounds.minY+bounds.height-1, 4, stone)
			encoded, err := EncodeChunkNBT(original)
			if err != nil {
				t.Fatal(err)
			}
			_, root, err := nbt.NewDecoder(encoded).ReadNamedRootCompound()
			if err != nil {
				t.Fatal(err)
			}
			if yPos, ok := root.GetInt("yPos"); !ok || yPos != int32(bounds.minY/16) {
				t.Fatalf("stored yPos = %d, %t", yPos, ok)
			}
			decoded, err := DecodeChunkNBT(encoded, 3, -2, 40)
			if err != nil {
				t.Fatal(err)
			}
			data := decoded.Data()
			if data.MinY() != bounds.minY || data.MaxY() != bounds.minY+bounds.height-1 || len(data.Sections) != bounds.height/16 {
				t.Fatalf("decoded bounds = %d..%d with %d sections", data.MinY(), data.MaxY(), len(data.Sections))
			}
			if got := decoded.GetBlock(1, bounds.minY, 2); got != stone {
				t.Fatalf("bottom block = %d", got)
			}
			if got := decoded.GetBlock(3, bounds.minY+bounds.height-1, 4); got != stone {
				t.Fatalf("top block = %d", got)
			}
		})
	}
}

// TestDecodeChunkNBTWithBoundsPreservesSparseDimension verifies omitted air sections.
func TestDecodeChunkNBTWithBoundsPreservesSparseDimension(t *testing.T) {
	original := New(0, 0)
	original.SetData(section.NewChunkWithBounds(0, 0, 0, 256, 40))
	stone := block.DefaultStateID("minecraft:stone")
	original.SetBlock(1, 0, 1, stone)
	encoded, err := EncodeChunkNBT(original)
	if err != nil {
		t.Fatal(err)
	}
	_, root, err := nbt.NewDecoder(encoded).ReadNamedRootCompound()
	if err != nil {
		t.Fatal(err)
	}
	sections, _ := root.GetList("sections")
	sparse := nbt.NewList(nbt.TagCompound)
	sparse.Add(sections.Elements()[0])
	root.PutList("sections", sparse)
	encoder := nbt.NewEncoder(len(encoded))
	if err := encoder.WriteNamedRootCompound("", root); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeChunkNBTWithBounds(encoder.Bytes(), 0, 0, 40, 0, 256)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Data().MaxY() != 255 || len(decoded.Data().Sections) != 16 || decoded.GetBlock(1, 0, 1) != stone {
		t.Fatal("sparse chunk lost its dimension bounds or stored block")
	}
	if _, err := DecodeChunkNBTWithBounds(encoder.Bytes(), 0, 0, 40, -64, 384); err == nil {
		t.Fatal("chunk accepted a conflicting dimension minimum")
	}
	store := persistence.NewChunkStore(t.TempDir())
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close chunk store: %v", err)
		}
	})
	if err := store.WriteChunkNBT(0, 0, encoder.Bytes()); err != nil {
		t.Fatal(err)
	}
	loaded, err := NewPersistentGenerator(store, testBoundsFallback{minY: 0, height: 256}, 40).Generate(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Data().MaxY() != 255 || loaded.GetBlock(1, 0, 1) != stone {
		t.Fatal("persistent loader lost the dimension bounds or stored block")
	}
}

func TestChunkNBTRoundtrip(t *testing.T) {
	c := newCodecTestChunk(3, 7, 40)

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

func TestEncodeChunkNBTUsesCurrentDataVersion(t *testing.T) {
	data, err := EncodeChunkNBT(newCodecTestChunk(0, 0, 40))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	_, root, err := nbt.NewDecoder(data).ReadNamedRootCompound()
	if err != nil {
		t.Fatalf("decode encoded chunk: %v", err)
	}
	version, ok := root.GetInt("DataVersion")
	if !ok || version != protocol.CurrentDataVersion {
		t.Fatalf("DataVersion = %d, %t; want %d", version, ok, protocol.CurrentDataVersion)
	}
}

func TestDecodeChunkNBTRejectsUnsupportedDataVersion(t *testing.T) {
	root := nbt.NewCompound()
	root.PutInt("DataVersion", protocol.CurrentDataVersion-1)
	encoder := nbt.NewEncoder(64)
	if err := encoder.WriteNamedRootCompound("", root); err != nil {
		t.Fatalf("encode: %v", err)
	}
	_, err := DecodeChunkNBT(encoder.Bytes(), 0, 0, 40)
	if err == nil || !strings.Contains(err.Error(), "unsupported DataVersion") {
		t.Fatalf("expected unsupported version error, got %v", err)
	}
}

func TestChunkNBTRoundtripEmpty(t *testing.T) {
	c := newCodecTestChunk(0, 0, 40)

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
	c := newCodecTestChunk(1, 1, 40)

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

func TestChunkNBTRoundtripPreservesUnknownNBT(t *testing.T) {
	root := nbt.NewCompound().
		PutInt("DataVersion", protocol.CurrentDataVersion).
		PutInt("xPos", 0).
		PutInt("zPos", 0).
		PutString("UnknownChunkTag", "preserve")
	sectionNBT := nbt.NewCompound().
		PutByte("Y", int8(anvilMinSectionY)).
		PutString("UnknownSectionTag", "preserve").
		PutCompound("block_states", nbt.NewCompound().PutList("palette", nbt.NewList(nbt.TagCompound).
			Add(nbt.NewCompound().PutString("Name", "minecraft:air")))).
		PutCompound("biomes", nbt.NewCompound().PutList("palette", nbt.NewList(nbt.TagString).
			Add("minecraft:plains")))
	root.PutList("sections", nbt.NewList(nbt.TagCompound).Add(sectionNBT))
	root.PutList("block_entities", nbt.NewList(nbt.TagCompound).Add(nbt.NewCompound().
		PutString("id", "minecraft:chest").
		PutInt("x", 1).
		PutInt("y", 64).
		PutInt("z", 2).
		PutString("UnknownBlockEntityTag", "preserve")))
	encoder := nbt.NewEncoder(1024)
	if err := encoder.WriteNamedRootCompound("", root); err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodeChunkNBT(encoder.Bytes(), 0, 0, 40)
	if err != nil {
		t.Fatalf("DecodeChunkNBT: %v", err)
	}
	if blockEntity := decoded.Data().GetBlockEntity(1, 64, 2); blockEntity == nil {
		t.Fatal("block entity was not loaded into chunk ownership")
	}
	encoded, err := EncodeChunkNBT(decoded)
	if err != nil {
		t.Fatalf("EncodeChunkNBT: %v", err)
	}
	decoder := nbt.NewDecoder(encoded)
	_, roundTrip, err := decoder.ReadNamedRootCompound()
	if err != nil {
		t.Fatal(err)
	}
	if value, _ := roundTrip.GetString("UnknownChunkTag"); value != "preserve" {
		t.Fatalf("UnknownChunkTag = %q; want preserve", value)
	}
	sections, _ := roundTrip.GetList("sections")
	first := sections.Elements()[0].(*nbt.Compound)
	if value, _ := first.GetString("UnknownSectionTag"); value != "preserve" {
		t.Fatalf("UnknownSectionTag = %q; want preserve", value)
	}
	blockEntities, _ := roundTrip.GetList("block_entities")
	blockEntity := blockEntities.Elements()[0].(*nbt.Compound)
	if value, _ := blockEntity.GetString("UnknownBlockEntityTag"); value != "preserve" {
		t.Fatalf("UnknownBlockEntityTag = %q; want preserve", value)
	}
}
