package chunk

import (
	"fmt"
	"math/bits"
	"sort"

	"github.com/vitismc/vitis/internal/block"
	blockentity "github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/world/chunk/section"
)

const (
	anvilDataVersion = protocol.CurrentDataVersion
	anvilMinSectionY = -4
)

// EncodeChunkNBT serializes an authoritative chunk into Anvil-compatible NBT.
func EncodeChunkNBT(c *Chunk) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("nil chunk")
	}
	data := c.Data()
	if data == nil {
		return nil, fmt.Errorf("chunk has no section data")
	}

	root := data.PersistentNBT()
	if root == nil {
		root = nbt.NewCompound()
	}
	root.PutInt("DataVersion", anvilDataVersion)
	root.PutInt("xPos", c.X())
	root.PutInt("zPos", c.Z())
	minSectionY := data.MinY() / section.SectionHeight
	root.PutInt("yPos", int32(minSectionY))
	root.PutString("Status", "minecraft:full")
	root.PutLong("InhabitedTime", c.InhabitedTime())
	// Runtime light is rebuilt from blocks; persisted light arrays are not authoritative.
	root.PutBool("isLightOn", false)
	if err := encodeScheduledTicksNBT(root, c); err != nil {
		return nil, err
	}

	sections := nbt.NewList(nbt.TagCompound)
	for i, sec := range data.Sections {
		sectionY := int8(minSectionY + i)
		sc, err := encodeSectionNBT(sec, sectionY, persistentSection(root, sectionY))
		if err != nil {
			return nil, fmt.Errorf("encode section %d: %w", i, err)
		}
		sections.Add(sc)
	}
	root.PutList("sections", sections)
	encodeBlockEntitiesNBT(root, data)

	enc := nbt.NewEncoder(32768)
	if err := enc.WriteNamedRootCompound("", root); err != nil {
		return nil, fmt.Errorf("encode root: %w", err)
	}
	data.SetPersistentNBT(root)
	return enc.Bytes(), nil
}

func persistentSection(root *nbt.Compound, y int8) *nbt.Compound {
	sections, ok := root.GetList("sections")
	if !ok {
		return nil
	}
	for _, element := range sections.Elements() {
		candidate, ok := element.(*nbt.Compound)
		if !ok {
			continue
		}
		candidateY, ok := candidate.GetByte("Y")
		if ok && candidateY == y {
			return candidate
		}
	}
	return nil
}

func encodeSectionNBT(sec *section.Section, y int8, persistent *nbt.Compound) (*nbt.Compound, error) {
	sc := persistent.Clone()
	if sc == nil {
		sc = nbt.NewCompound()
	}
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

// blockStateToNBT encodes registry properties in their declared order.
func blockStateToNBT(stateID int32) *nbt.Compound {
	c := nbt.NewCompound()
	info := block.Info(block.BlockIDFromState(stateID))
	if info == nil {
		c.PutString("Name", "minecraft:air")
		return c
	}
	c.PutString("Name", info.Name)

	if len(info.Properties) > 0 {
		pc := nbt.NewCompound()
		offset := stateID - info.MinStateID
		for i, property := range info.Properties {
			stride := int32(1)
			for j := i + 1; j < len(info.Properties); j++ {
				stride *= int32(len(info.Properties[j].Values))
			}
			index := offset / stride
			offset %= stride
			if index >= 0 && int(index) < len(property.Values) {
				pc.PutString(property.Name, property.Values[index])
			}
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

// DecodeChunkNBT parses Anvil-compatible NBT bytes into an authoritative chunk.
func DecodeChunkNBT(data []byte, x, z int32, defaultBiome int32) (*Chunk, error) {
	return decodeChunkNBT(data, x, z, defaultBiome, 0, 0)
}

// DecodeChunkNBTWithBounds uses dimension bounds when sections are omitted from NBT.
func DecodeChunkNBTWithBounds(data []byte, x, z int32, defaultBiome int32, minY, height int) (*Chunk, error) {
	if minY%section.SectionHeight != 0 || height < section.SectionHeight || height%section.SectionHeight != 0 || height > 4096 || minY/section.SectionHeight < -128 || (minY+height)/section.SectionHeight > 128 {
		return nil, fmt.Errorf("invalid chunk bounds %d+%d", minY, height)
	}
	return decodeChunkNBT(data, x, z, defaultBiome, minY, height)
}

func decodeChunkNBT(data []byte, x, z int32, defaultBiome int32, boundMinY, boundHeight int) (*Chunk, error) {
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
	if _, tag, present := root.Get("InhabitedTime"); present {
		if value, ok := root.GetLong("InhabitedTime"); tag != nbt.TagLong || !ok || value < 0 {
			return nil, fmt.Errorf("invalid chunk inhabited time")
		}
	}
	dataVersion, ok := root.GetInt("DataVersion")
	if !ok {
		return nil, fmt.Errorf("missing DataVersion")
	}
	if dataVersion != anvilDataVersion {
		return nil, fmt.Errorf("unsupported DataVersion %d", dataVersion)
	}

	sectionsTag, ok := root.GetList("sections")
	if !ok || sectionsTag == nil || sectionsTag.Len() == 0 {
		return nil, fmt.Errorf("no sections tag")
	}

	minSectionY, sectionCount := anvilMinSectionY, section.OverworldSections
	if boundHeight != 0 {
		minSectionY, sectionCount = boundMinY/section.SectionHeight, boundHeight/section.SectionHeight
		if storedMin, ok := root.GetInt("yPos"); ok && int(storedMin) != minSectionY {
			return nil, fmt.Errorf("chunk yPos %d differs from dimension minimum %d", storedMin, minSectionY)
		}
	} else if storedMin, ok := root.GetInt("yPos"); ok {
		if storedMin < -127 || storedMin > 127 {
			return nil, fmt.Errorf("invalid yPos %d", storedMin)
		}
		minSectionY = int(storedMin)
		maxSectionY := minSectionY - 1
		for _, elem := range sectionsTag.Elements() {
			if sc, ok := elem.(*nbt.Compound); ok {
				if y, ok := sc.GetByte("Y"); ok && int(y) > maxSectionY {
					maxSectionY = int(y)
				}
			}
		}
		sectionCount = maxSectionY - minSectionY + 1
		if sectionCount < 1 || sectionCount > 256 || minSectionY+sectionCount-1 > 127 {
			return nil, fmt.Errorf("invalid section range %d..%d", minSectionY, maxSectionY)
		}
	}
	chunkData := section.NewChunkWithBounds(x, z, minSectionY*section.SectionHeight, sectionCount*section.SectionHeight, defaultBiome)
	chunkData.SetPersistentNBT(root)

	for _, elem := range sectionsTag.Elements() {
		sc, ok := elem.(*nbt.Compound)
		if !ok || sc == nil {
			continue
		}

		yByte, ok := sc.GetByte("Y")
		if !ok {
			continue
		}
		sectionIdx := int(yByte) - minSectionY
		if sectionIdx < 0 || sectionIdx >= len(chunkData.Sections) {
			continue
		}

		sec := chunkData.Sections[sectionIdx]

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
	decodeBlockEntitiesNBT(root, chunkData)
	repairedBeds := chunkData.RepairBedBlockEntities()

	chunk := New(x, z)
	chunk.SetData(chunkData)
	if repairedBeds > 0 {
		chunk.MarkDirty()
	}
	return chunk, nil
}

type persistentBlockEntity interface {
	PersistentNBT() *nbt.Compound
}

func encodeBlockEntitiesNBT(root *nbt.Compound, data *section.Chunk) {
	if data.BlockEntities == nil {
		return
	}
	keys := make([]int64, 0, len(data.BlockEntities))
	for key := range data.BlockEntities {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	list := nbt.NewList(nbt.TagCompound)
	for _, key := range keys {
		blockEntity := data.BlockEntities[key]
		if blockEntity == nil {
			continue
		}
		var compound *nbt.Compound
		if persistent, ok := blockEntity.(persistentBlockEntity); ok {
			compound = persistent.PersistentNBT()
		}
		if compound == nil {
			x, y, z := blockEntity.Position()
			compound = nbt.NewCompound().
				PutString("id", blockEntity.TypeName()).
				PutInt("x", x).
				PutInt("y", y).
				PutInt("z", z)
		}
		list.Add(compound)
	}
	root.PutList("block_entities", list)
}

func decodeBlockEntitiesNBT(root *nbt.Compound, data *section.Chunk) {
	list, ok := root.GetList("block_entities")
	if !ok || list == nil {
		return
	}
	data.BlockEntities = make(map[int64]blockentity.BlockEntity, list.Len())
	for _, element := range list.Elements() {
		compound, ok := element.(*nbt.Compound)
		if !ok {
			continue
		}
		blockEntity := blockentity.NewRawBlockEntity(compound)
		if blockEntity != nil && blockEntity.TypeID() >= 0 {
			data.AddBlockEntity(blockEntity)
		}
	}
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
