package chunk

import (
	"fmt"
	"math/bits"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/chunk/section"
)

const (
	anvilDataVersion = 3837
	anvilMinSectionY = -4
)

// EncodeChunkNBT serializes a section.Chunk into Anvil-compatible NBT bytes.
func EncodeChunkNBT(c *section.Chunk) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("nil chunk")
	}

	root := nbt.NewCompound()
	root.PutInt("DataVersion", anvilDataVersion)
	root.PutInt("xPos", c.X)
	root.PutInt("zPos", c.Z)
	root.PutInt("yPos", int32(anvilMinSectionY))
	root.PutString("Status", "minecraft:full")

	sections := nbt.NewList(nbt.TagCompound)
	for i, sec := range c.Sections {
		sectionY := int8(anvilMinSectionY) + int8(i)
		sc, err := encodeSectionNBT(sec, sectionY)
		if err != nil {
			return nil, fmt.Errorf("encode section %d: %w", i, err)
		}
		sections.Add(sc)
	}
	root.PutList("sections", sections)

	enc := nbt.NewEncoder(32768)
	if err := enc.WriteNamedRootCompound("", root); err != nil {
		return nil, fmt.Errorf("encode root: %w", err)
	}
	return enc.Bytes(), nil
}

func encodeSectionNBT(sec *section.Section, y int8) (*nbt.Compound, error) {
	sc := nbt.NewCompound()
	sc.PutByte("Y", y)

	blockStates := encodeBlockPaletteNBT(&sec.Blocks)
	sc.PutCompound("block_states", blockStates)

	biomes := encodeBiomePaletteNBT(&sec.Biomes)
	sc.PutCompound("biomes", biomes)

	return sc, nil
}

func encodeBlockPaletteNBT(pc *section.PaletteContainer) *nbt.Compound {
	comp := nbt.NewCompound()

	if pc.IsSingle() {
		pal := nbt.NewList(nbt.TagCompound)
		pal.Add(blockStateToNBT(pc.Palette()[0]))
		comp.PutList("palette", pal)
		return comp
	}

	palette := pc.Palette()
	if palette != nil {
		pal := nbt.NewList(nbt.TagCompound)
		for _, stateID := range palette {
			pal.Add(blockStateToNBT(stateID))
		}
		comp.PutList("palette", pal)

		raw := pc.RawData()
		if len(raw) > 0 {
			data := make([]int64, len(raw))
			for i, v := range raw {
				data[i] = int64(v)
			}
			comp.PutLongArray("data", data)
		}
	} else {
		uniqueMap := make(map[int32]int32)
		var palSlice []int32
		reindexed := make([]int32, section.BlocksPerSection)
		for i := 0; i < section.BlocksPerSection; i++ {
			val := pc.Get(i)
			idx, exists := uniqueMap[val]
			if !exists {
				idx = int32(len(palSlice))
				uniqueMap[val] = idx
				palSlice = append(palSlice, val)
			}
			reindexed[i] = idx
		}

		pal := nbt.NewList(nbt.TagCompound)
		for _, stateID := range palSlice {
			pal.Add(blockStateToNBT(stateID))
		}
		comp.PutList("palette", pal)

		bpe := bitsForPalette(len(palSlice))
		if bpe > 0 {
			storage := section.NewBitStorage(bpe, section.BlocksPerSection, nil)
			for i, idx := range reindexed {
				storage.Set(i, idx)
			}
			raw := storage.Raw()
			data := make([]int64, len(raw))
			for i, v := range raw {
				data[i] = int64(v)
			}
			comp.PutLongArray("data", data)
		}
	}

	return comp
}

func encodeBiomePaletteNBT(pc *section.PaletteContainer) *nbt.Compound {
	comp := nbt.NewCompound()

	if pc.IsSingle() {
		pal := nbt.NewList(nbt.TagString)
		pal.Add(biomeIDToName(pc.Palette()[0]))
		comp.PutList("palette", pal)
		return comp
	}

	palette := pc.Palette()
	if palette != nil {
		pal := nbt.NewList(nbt.TagString)
		for _, id := range palette {
			pal.Add(biomeIDToName(id))
		}
		comp.PutList("palette", pal)

		raw := pc.RawData()
		if len(raw) > 0 {
			data := make([]int64, len(raw))
			for i, v := range raw {
				data[i] = int64(v)
			}
			comp.PutLongArray("data", data)
		}
	} else {
		uniqueMap := make(map[int32]int32)
		var palSlice []int32
		reindexed := make([]int32, section.BiomesPerSection)
		for i := 0; i < section.BiomesPerSection; i++ {
			val := pc.Get(i)
			idx, exists := uniqueMap[val]
			if !exists {
				idx = int32(len(palSlice))
				uniqueMap[val] = idx
				palSlice = append(palSlice, val)
			}
			reindexed[i] = idx
		}

		pal := nbt.NewList(nbt.TagString)
		for _, id := range palSlice {
			pal.Add(biomeIDToName(id))
		}
		comp.PutList("palette", pal)

		bpe := bitsForPalette(len(palSlice))
		if bpe > 0 {
			storage := section.NewBitStorage(bpe, section.BiomesPerSection, nil)
			for i, idx := range reindexed {
				storage.Set(i, idx)
			}
			raw := storage.Raw()
			data := make([]int64, len(raw))
			for i, v := range raw {
				data[i] = int64(v)
			}
			comp.PutLongArray("data", data)
		}
	}

	return comp
}

func blockStateToNBT(stateID int32) *nbt.Compound {
	c := nbt.NewCompound()
	name := block.NameFromState(stateID)
	if name == "" {
		name = "minecraft:air"
	}
	c.PutString("Name", name)

	props := block.PropertiesFromState(stateID)
	if len(props) > 0 {
		pc := nbt.NewCompound()
		for k, v := range props {
			pc.PutString(k, v)
		}
		c.PutCompound("Properties", pc)
	}
	return c
}

func biomeIDToName(id int32) string {
	info := biome.BiomeByID(id)
	if info != nil {
		return info.Name
	}
	return "minecraft:plains"
}

// DecodeChunkNBT parses Anvil-compatible NBT bytes into a section.Chunk.
func DecodeChunkNBT(data []byte, x, z int32, defaultBiome int32) (*section.Chunk, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	dec := nbt.NewDecoder(data)
	_, root, err := dec.ReadNamedRootCompound()
	if err != nil {
		return nil, fmt.Errorf("decode root: %w", err)
	}
	if root == nil {
		return nil, fmt.Errorf("nil root compound")
	}

	sectionsTag, ok := root.GetList("sections")
	if !ok || sectionsTag == nil || sectionsTag.Len() == 0 {
		return nil, fmt.Errorf("no sections tag")
	}

	chunk := section.NewChunk(x, z, section.OverworldSections, defaultBiome)

	for _, elem := range sectionsTag.Elements() {
		sc, ok := elem.(*nbt.Compound)
		if !ok || sc == nil {
			continue
		}

		yByte, ok := sc.GetByte("Y")
		if !ok {
			continue
		}
		sectionIdx := int(yByte) - anvilMinSectionY
		if sectionIdx < 0 || sectionIdx >= len(chunk.Sections) {
			continue
		}

		sec := chunk.Sections[sectionIdx]

		blockStatesTag, ok := sc.GetCompound("block_states")
		if ok && blockStatesTag != nil {
			blocks, err := decodeBlockPaletteNBT(blockStatesTag)
			if err == nil {
				sec.Blocks = *blocks
			}
		}

		biomesTag, ok := sc.GetCompound("biomes")
		if ok && biomesTag != nil {
			biomes, err := decodeBiomePaletteNBT(biomesTag, defaultBiome)
			if err == nil {
				sec.Biomes = *biomes
			}
		}
	}

	return chunk, nil
}

func decodeBlockPaletteNBT(comp *nbt.Compound) (*section.PaletteContainer, error) {
	palTag, ok := comp.GetList("palette")
	if !ok || palTag == nil || palTag.Len() == 0 {
		pc := section.NewSinglePalette(section.BlocksPerSection, 0)
		return &pc, nil
	}

	palette := make([]int32, palTag.Len())
	for i, elem := range palTag.Elements() {
		ec, ok := elem.(*nbt.Compound)
		if !ok {
			continue
		}
		name, ok := ec.GetString("Name")
		if !ok {
			continue
		}

		propsComp, hasProps := ec.GetCompound("Properties")
		if hasProps && propsComp != nil {
			propsMap := propsComp.GetAllStrings()
			sid := block.StateID(name, propsMap)
			if sid >= 0 {
				palette[i] = sid
			} else {
				palette[i] = block.DefaultStateID(name)
				if palette[i] < 0 {
					palette[i] = 0
				}
			}
		} else {
			sid := block.DefaultStateID(name)
			if sid < 0 {
				sid = 0
			}
			palette[i] = sid
		}
	}

	if len(palette) == 1 {
		pc := section.NewSinglePalette(section.BlocksPerSection, palette[0])
		return &pc, nil
	}

	dataRaw, hasData := getLongArray(comp, "data")
	if !hasData || len(dataRaw) == 0 {
		pc := section.NewSinglePalette(section.BlocksPerSection, palette[0])
		return &pc, nil
	}

	bpe := bitsPerEntryFromData(len(dataRaw), section.BlocksPerSection)
	if bpe <= 0 {
		pc := section.NewSinglePalette(section.BlocksPerSection, palette[0])
		return &pc, nil
	}

	u64 := make([]uint64, len(dataRaw))
	for i, v := range dataRaw {
		u64[i] = uint64(v)
	}

	pc := section.NewIndirectPalette(section.BlocksPerSection, bpe, palette, u64)
	return &pc, nil
}

func decodeBiomePaletteNBT(comp *nbt.Compound, defaultBiome int32) (*section.PaletteContainer, error) {
	palTag, ok := comp.GetList("palette")
	if !ok || palTag == nil || palTag.Len() == 0 {
		pc := section.NewSinglePalette(section.BiomesPerSection, defaultBiome)
		return &pc, nil
	}

	palette := make([]int32, palTag.Len())
	for i, elem := range palTag.Elements() {
		name, ok := elem.(string)
		if !ok {
			palette[i] = defaultBiome
			continue
		}
		id := biome.BiomeIDByName(name)
		if id < 0 {
			id = defaultBiome
		}
		palette[i] = id
	}

	if len(palette) == 1 {
		pc := section.NewSinglePalette(section.BiomesPerSection, palette[0])
		return &pc, nil
	}

	dataRaw, hasData := getLongArray(comp, "data")
	if !hasData || len(dataRaw) == 0 {
		pc := section.NewSinglePalette(section.BiomesPerSection, palette[0])
		return &pc, nil
	}

	bpe := bitsPerEntryFromData(len(dataRaw), section.BiomesPerSection)
	if bpe <= 0 {
		pc := section.NewSinglePalette(section.BiomesPerSection, palette[0])
		return &pc, nil
	}

	u64 := make([]uint64, len(dataRaw))
	for i, v := range dataRaw {
		u64[i] = uint64(v)
	}

	pc := section.NewIndirectPalette(section.BiomesPerSection, bpe, palette, u64)
	return &pc, nil
}

func getLongArray(comp *nbt.Compound, name string) ([]int64, bool) {
	v, tid, ok := comp.Get(name)
	if !ok || tid != nbt.TagLongArray {
		return nil, false
	}
	arr, ok := v.([]int64)
	return arr, ok
}

func bitsPerEntryFromData(dataLen, totalEntries int) int {
	if dataLen == 0 || totalEntries == 0 {
		return 0
	}
	for bpe := 1; bpe <= 15; bpe++ {
		vpl := 64 / bpe
		needed := (totalEntries + vpl - 1) / vpl
		if needed == dataLen {
			return bpe
		}
	}
	return 0
}

func bitsForPalette(count int) int {
	if count <= 1 {
		return 0
	}
	return bits.Len(uint(count - 1))
}
