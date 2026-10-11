package section

import (
	"strings"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/lighting"
)

// HeightmapKind identifies a cached vertical block predicate.
type HeightmapKind uint8

const (
	HeightmapWorldSurface HeightmapKind = iota
	HeightmapOceanFloor
	HeightmapMotionBlocking
	HeightmapMotionBlockingNoLeaves
	heightmapKindCount
)

const (
	// OverworldMinY is the minimum Y coordinate for the overworld.
	OverworldMinY = -64
	// OverworldMaxY is the maximum Y coordinate for the overworld.
	OverworldMaxY = 319
	// OverworldSections is the number of sections in an overworld chunk column.
	OverworldSections = (OverworldMaxY - OverworldMinY + 1) / SectionHeight
	// LightSections is sections + 2 for light (one above, one below).
	LightSections = OverworldSections + 2
)

// Chunk represents a 16-wide column of sections.
type Chunk struct {
	X               int32
	Z               int32
	Sections        []*Section
	minY            int
	maxY            int
	BlockEntities   map[int64]entity.BlockEntity
	LightEngine     *lighting.Engine
	persistentNBT   *nbt.Compound
	cachedHeights   [heightmapKindCount][16 * 16]int16
	heightmapValid  [heightmapKindCount][4]uint64
	cacheHeightmaps bool
	borderBlocks    map[BorderPosition]int32
	borderEntities  map[BorderPosition]entity.BlockEntity
	denseBlocks     []int32
	featureRegion   *featureRegionState
}

// CloneRaw returns an independently mutable copy of generated section data.
func (c *Chunk) CloneRaw() *Chunk {
	if c == nil {
		return nil
	}
	clone := *c
	clone.Sections = make([]*Section, len(c.Sections))
	for i, original := range c.Sections {
		copySection := *original
		copySection.Blocks = original.Blocks.Clone()
		copySection.Biomes = original.Biomes.Clone()
		copySection.BlockLight = append([]byte(nil), original.BlockLight...)
		copySection.SkyLight = append([]byte(nil), original.SkyLight...)
		clone.Sections[i] = &copySection
	}
	clone.borderBlocks = nil
	clone.borderEntities = nil
	clone.featureRegion = nil
	clone.denseBlocks = nil
	if c.denseBlocks != nil {
		for index, current := range clone.Sections {
			current.FillBlocks((*[BlocksPerSection]int32)(c.denseBlocks[index*BlocksPerSection:]))
		}
	}
	clone.BlockEntities = nil
	clone.LightEngine = nil
	clone.persistentNBT = nil
	return &clone
}

// BorderPosition identifies a feature block outside its source chunk.
type BorderPosition struct{ X, Y, Z int }

// EnableBorderBlocks records feature writes beyond this chunk's horizontal bounds.
func (c *Chunk) EnableBorderBlocks() {
	if c != nil && c.borderBlocks == nil {
		c.borderBlocks = make(map[BorderPosition]int32)
	}
}

// BorderBlocks returns the feature writes outside this chunk.
func (c *Chunk) BorderBlocks() map[BorderPosition]int32 {
	if c == nil {
		return nil
	}
	if c.featureRegion == nil {
		return c.borderBlocks
	}
	states := make(map[BorderPosition]int32, len(c.borderBlocks))
	for position, state := range c.borderBlocks {
		states[position] = state
	}
	c.featureRegion.borderWrites.Range(func(position BorderPosition, write FeatureBorderWrite) { states[position] = write.State })
	return states
}

// BorderBlockEntities returns transient feature entities belonging to adjacent chunks.
func (c *Chunk) BorderBlockEntities() map[BorderPosition]entity.BlockEntity {
	if c == nil {
		return nil
	}
	return c.borderEntities
}

// SetBorderBlock records one feature block outside this chunk.
func (c *Chunk) SetBorderBlock(x, y, z int, state int32) {
	if c != nil && c.borderBlocks != nil && y >= c.minY && y <= c.maxY {
		if c.featureRegion != nil && !c.featureWriteBounds(x, z) {
			return
		}
		if region := c.featureRegion; region != nil {
			region.borderWrites.set(x, y, z, FeatureBorderWrite{State: state, Priority: region.priority})
			columns := region.columnWrites
			dx, dz := x-int(c.X)*16, z-int(c.Z)*16
			if dx >= -16 && dx < 32 && dz >= -16 && dz < 32 {
				column := (dz+16)*48 + dx + 16
				word, bit := column>>6, uint64(1)<<uint(column&63)
				if columns.valid[word]&bit == 0 || y > int(columns.heights[column]) {
					columns.heights[column] = int16(y)
					columns.valid[word] |= bit
				}
			} else {
				if columns.outer == nil {
					columns.outer = make(map[[2]int]int16)
				}
				key := [2]int{x, z}
				if previous, ok := columns.outer[key]; !ok || y > int(previous) {
					columns.outer[key] = int16(y)
				}
			}
		} else {
			c.borderBlocks[BorderPosition{X: x, Y: y, Z: z}] = state
		}
	}
}

// GetBorderBlock returns a previously recorded feature block.
func (c *Chunk) GetBorderBlock(x, y, z int) (int32, bool) {
	if c == nil {
		return 0, false
	}
	if c.featureRegion != nil {
		if write, ok := c.featureRegion.borderWrites.get(x, y, z); ok {
			return write.State, true
		}
	}
	state, ok := c.borderBlocks[BorderPosition{X: x, Y: y, Z: z}]
	if !ok && c.featureRegion != nil && c.featureRegion.readBlock != nil && y >= c.minY && y <= c.maxY {
		return c.featureRegion.readBlock(x, y, z), true
	}
	return state, ok
}

// BorderBlocksEnabled reports whether feature writes may leave this chunk.
func (c *Chunk) BorderBlocksEnabled() bool { return c != nil && c.borderBlocks != nil }

// DropBorderBlocks releases transient feature writes from a returned chunk.
func (c *Chunk) DropBorderBlocks() {
	if c != nil {
		c.borderBlocks = nil
		c.borderEntities = nil
		c.featureRegion = nil
	}
}

// PersistentNBT returns a copy of the source chunk NBT used for lossless persistence.
func (c *Chunk) PersistentNBT() *nbt.Compound {
	if c == nil {
		return nil
	}
	return c.persistentNBT.Clone()
}

// SetPersistentNBT stores a copy of the source chunk NBT for lossless persistence.
func (c *Chunk) SetPersistentNBT(root *nbt.Compound) {
	if c == nil {
		return
	}
	c.persistentNBT = root.Clone()
}

// NewChunk creates a chunk filled with air and the given biome.
func NewChunk(x, z int32, numSections int, defaultBiome int32) *Chunk {
	return NewChunkWithBounds(x, z, OverworldMinY, numSections*SectionHeight, defaultBiome)
}

// NewChunkWithBounds creates a chunk for a dimension with explicit vertical bounds.
func NewChunkWithBounds(x, z int32, minY, height int, defaultBiome int32) *Chunk {
	if height <= 0 || height%SectionHeight != 0 || minY%SectionHeight != 0 {
		minY, height = OverworldMinY, OverworldMaxY-OverworldMinY+1
	}
	numSections := height / SectionHeight
	sections := make([]*Section, numSections)
	minSectionY := int8(minY / SectionHeight)
	for i := range sections {
		sections[i] = NewSection(minSectionY+int8(i), 0, defaultBiome)
	}
	return &Chunk{X: x, Z: z, Sections: sections, minY: minY, maxY: minY + height - 1}
}

// MinY implements lighting.BlockAccess.
func (c *Chunk) MinY() int { return c.minY }

// MaxY implements lighting.BlockAccess.
func (c *Chunk) MaxY() int { return c.maxY }

// NumSections implements lighting.BlockAccess.
func (c *Chunk) NumSections() int { return len(c.Sections) }

// BlockPalette returns a section's block palette, or nil for direct storage.
func (c *Chunk) BlockPalette(section int) []int32 {
	if section < 0 || section >= len(c.Sections) {
		return nil
	}
	return c.Sections[section].Blocks.Palette()
}

// ComputeLight runs the lighting engine for this chunk column and stores the result.
func (c *Chunk) ComputeLight() {
	eng := lighting.NewSparseEngine(len(c.Sections), c.minY, c.maxY)
	eng.ComputeAll(c)
	c.LightEngine = eng
}

// GetBlock returns the block state ID at the given absolute coordinates.
func (c *Chunk) GetBlock(x, y, z int) int32 {
	if y < c.minY || y > c.maxY {
		return 0
	}
	if c.denseBlocks != nil {
		return c.denseBlocks[(y-c.minY)*256+(z&15)*16+(x&15)]
	}
	secIdx := (y - c.minY) / SectionHeight
	if secIdx >= len(c.Sections) {
		return 0
	}
	blocks := &c.Sections[secIdx].Blocks
	if blocks.mode == modeSingle {
		return blocks.palette[0]
	}
	return blocks.Get((y&15)*256 + (z&15)*16 + (x & 15))
}

// SetBlock sets the block state ID at the given absolute coordinates.
func (c *Chunk) SetBlock(x, y, z int, stateID int32) {
	if y < c.minY || y > c.maxY {
		return
	}
	secIdx := (y - c.minY) / SectionHeight
	if secIdx >= len(c.Sections) {
		return
	}
	if c.denseBlocks != nil {
		c.denseBlocks[(y-c.minY)*256+(z&15)*16+(x&15)] = stateID
	} else {
		c.Sections[secIdx].SetBlock(x, y, z, stateID)
	}
	c.syncBedBlockEntity(x, y, z, stateID)
	if c.cacheHeightmaps {
		index := (z&15)*16 + (x & 15)
		word, bit := index>>6, uint(index&63)
		for kind := HeightmapKind(0); kind < heightmapKindCount; kind++ {
			// Changes below the highest matching block cannot alter a column height.
			if c.heightmapValid[kind][word]&(uint64(1)<<bit) != 0 && y >= int(c.cachedHeights[kind][index])-1 {
				c.heightmapValid[kind][word] &^= uint64(1) << bit
			}
		}
	}
}

// EnableHeightmapCache enables incremental heightmap caching for subsequent reads.
func (c *Chunk) EnableHeightmapCache() {
	if c != nil {
		c.cacheHeightmaps = true
	}
}

// Heightmap returns the first free Y above the highest matching block in a column.
func (c *Chunk) Heightmap(x, z int, kind HeightmapKind) (int, bool) {
	if c == nil || kind >= heightmapKindCount {
		return 0, false
	}
	index := (z&15)*16 + (x & 15)
	word, bit := index>>6, uint(index&63)
	if c.cacheHeightmaps && c.heightmapValid[kind][word]&(uint64(1)<<bit) != 0 {
		value := int(c.cachedHeights[kind][index])
		return value, value > c.minY
	}
	value := c.minY
scan:
	for section := len(c.Sections) - 1; section >= 0; section-- {
		if palette := c.Sections[section].Blocks.Palette(); c.denseBlocks == nil && palette != nil {
			possible := false
			for _, state := range palette {
				switch kind {
				case HeightmapWorldSurface:
					possible = !block.IsAir(state)
				case HeightmapOceanFloor:
					possible = block.IsSolid(state)
				case HeightmapMotionBlocking:
					possible = blocksMotion(state)
				case HeightmapMotionBlockingNoLeaves:
					possible = (block.IsSolid(state) && !strings.HasSuffix(block.NameFromState(state), "_leaves")) || block.IsWater(state) || block.IsLava(state)
				}
				if possible {
					break
				}
			}
			if !possible {
				continue
			}
		}
		bottom := c.minY + section*SectionHeight
		top := min(c.maxY, bottom+SectionHeight-1)
		for y := top; y >= bottom; y-- {
			state := c.GetBlock(x, y, z)
			matched := false
			switch kind {
			case HeightmapWorldSurface:
				matched = !block.IsAir(state)
			case HeightmapOceanFloor:
				matched = block.IsSolid(state)
			case HeightmapMotionBlocking:
				matched = blocksMotion(state)
			case HeightmapMotionBlockingNoLeaves:
				matched = (block.IsSolid(state) && !strings.HasSuffix(block.NameFromState(state), "_leaves")) || block.IsWater(state) || block.IsLava(state)
			}
			if matched {
				value = y + 1
				break scan
			}
		}
	}
	if c.cacheHeightmaps {
		c.cachedHeights[kind][index] = int16(value)
		c.heightmapValid[kind][word] |= uint64(1) << bit
	}
	return value, value > c.minY
}

func blockEntityKey(x, y, z int32) int64 {
	return (int64(x)&0x3FFFFFF)<<38 | (int64(z)&0x3FFFFFF)<<12 | int64(y)&0xFFF
}

// AddBlockEntity routes generated entities beyond the chunk boundary to transient storage.
func (c *Chunk) AddBlockEntity(be entity.BlockEntity) {
	if be == nil {
		return
	}
	x, y, z := be.Position()
	if c.featureRegion != nil && !c.FeatureWriteAllowed(int(x), int(y), int(z)) {
		return
	}
	if c.borderBlocks != nil && (x>>4 != c.X || z>>4 != c.Z) {
		if int(y) < c.minY || int(y) > c.maxY {
			return
		}
		if c.borderEntities == nil {
			c.borderEntities = make(map[BorderPosition]entity.BlockEntity)
		}
		c.borderEntities[BorderPosition{int(x), int(y), int(z)}] = be
		return
	}
	if c.BlockEntities == nil {
		c.BlockEntities = make(map[int64]entity.BlockEntity)
	}
	c.BlockEntities[blockEntityKey(x, y, z)] = be
}

// RemoveBlockEntity removes a local or transient generated entity at the world position.
func (c *Chunk) RemoveBlockEntity(x, y, z int32) {
	delete(c.borderEntities, BorderPosition{int(x), int(y), int(z)})
	if c.BlockEntities == nil {
		return
	}
	delete(c.BlockEntities, blockEntityKey(x, y, z))
}

// GetBlockEntity returns a local entity or a previously recorded neighboring write.
func (c *Chunk) GetBlockEntity(x, y, z int32) entity.BlockEntity {
	if be := c.borderEntities[BorderPosition{int(x), int(y), int(z)}]; be != nil {
		return be
	}
	if c.BlockEntities == nil {
		return nil
	}
	return c.BlockEntities[blockEntityKey(x, y, z)]
}

// EncodePacketPayload builds the full ChunkDataAndUpdateLight payload (everything after chunkX/chunkZ).
func (c *Chunk) EncodePacketPayload() []byte {
	buf := make([]byte, 0, 16384)

	buf = c.encodeHeightmaps(buf)

	sectionData := make([]byte, 0, 8192)
	for _, s := range c.Sections {
		sectionData = EncodeSectionData(sectionData, s)
	}
	buf = appendVarInt(buf, int32(len(sectionData)))
	buf = append(buf, sectionData...)

	buf = c.encodeBlockEntities(buf)

	buf = c.encodeLightData(buf)

	return buf
}

func (c *Chunk) encodeBlockEntities(dst []byte) []byte {
	if len(c.BlockEntities) == 0 {
		return appendVarInt(dst, 0)
	}

	dst = appendVarInt(dst, int32(len(c.BlockEntities)))

	for _, be := range c.BlockEntities {
		x, y, z := be.Position()
		localXZ := byte(((x & 0xF) << 4) | (z & 0xF))
		dst = append(dst, localXZ)

		dst = append(dst, byte(y>>8), byte(y))

		dst = appendVarInt(dst, be.TypeID())

		nbtData := be.ChunkDataNBT()
		if len(nbtData) == 0 {
			dst = append(dst, 0x00)
		} else {
			dst = append(dst, nbtData...)
		}
	}

	return dst
}

func (c *Chunk) encodeHeightmaps(dst []byte) []byte {
	worldSurface, motionBlocking := c.heightmaps()

	hm := nbt.NewCompound()
	hm.PutLongArray("MOTION_BLOCKING", packHeightmap(motionBlocking))
	hm.PutLongArray("WORLD_SURFACE", packHeightmap(worldSurface))

	enc := nbt.NewEncoder(512)
	_ = enc.WriteRootCompound(hm)
	return append(dst, enc.Bytes()...)
}

func (c *Chunk) heightmaps() (worldSurface, motionBlocking [16 * 16]uint16) {
	for z := 0; z < 16; z++ {
		for x := 0; x < 16; x++ {
			index := z*16 + x
			word, bit := index>>6, uint(index&63)
			mask := uint64(1) << bit
			surfaceValid := c.cacheHeightmaps && c.heightmapValid[HeightmapWorldSurface][word]&mask != 0
			motionValid := c.cacheHeightmaps && c.heightmapValid[HeightmapMotionBlocking][word]&mask != 0
			surfaceY, motionY := c.minY, c.minY
			if surfaceValid {
				surfaceY = int(c.cachedHeights[HeightmapWorldSurface][index])
			}
			if motionValid {
				motionY = int(c.cachedHeights[HeightmapMotionBlocking][index])
			}
			for sectionIndex := len(c.Sections) - 1; sectionIndex >= 0 && (!surfaceValid || !motionValid); sectionIndex-- {
				section := c.Sections[sectionIndex]
				if section.Blocks.IsSingle() {
					state := section.Blocks.Palette()[0]
					if block.IsAir(state) {
						continue
					}
					topY := c.minY + (sectionIndex+1)*SectionHeight
					if !surfaceValid && !block.IsAir(state) {
						surfaceY = topY
						surfaceValid = true
					}
					if !motionValid && blocksMotion(state) {
						motionY = topY
						motionValid = true
					}
					continue
				}
				for y := c.minY + (sectionIndex+1)*SectionHeight - 1; y >= c.minY+sectionIndex*SectionHeight && (!surfaceValid || !motionValid); y-- {
					state := section.GetBlock(x, y, z)
					if !surfaceValid && !block.IsAir(state) {
						surfaceY = y + 1
						surfaceValid = true
					}
					if !motionValid && blocksMotion(state) {
						motionY = y + 1
						motionValid = true
					}
				}
			}
			if c.cacheHeightmaps {
				c.cachedHeights[HeightmapWorldSurface][index] = int16(surfaceY)
				c.cachedHeights[HeightmapMotionBlocking][index] = int16(motionY)
				c.heightmapValid[HeightmapWorldSurface][word] |= mask
				c.heightmapValid[HeightmapMotionBlocking][word] |= mask
			}
			if surfaceY > c.minY {
				worldSurface[index] = uint16(surfaceY - c.minY)
			}
			if motionY > c.minY {
				motionBlocking[index] = uint16(motionY - c.minY)
			}
		}
	}
	return worldSurface, motionBlocking
}

func blocksMotion(stateID int32) bool {
	return block.IsSolid(stateID) || block.IsWater(stateID) || block.IsLava(stateID)
}

func packHeightmap(values [16 * 16]uint16) []int64 {
	const bitsPerEntry = 9
	const entriesPerLong = 64 / bitsPerEntry
	packed := make([]int64, (len(values)+entriesPerLong-1)/entriesPerLong)
	for index, value := range values {
		word := index / entriesPerLong
		shift := uint((index % entriesPerLong) * bitsPerEntry)
		packed[word] |= int64(value) << shift
	}
	return packed
}

func (c *Chunk) encodeLightData(dst []byte) []byte {
	if c.LightEngine == nil {
		return c.encodeLightDataEmpty(dst)
	}

	eng := c.LightEngine
	numLightSections := len(eng.SkyLightSections)

	var skyMask, blockMask uint64
	var emptySkyMask, emptyBlockMask uint64

	var skyArrays [][]byte
	var blockArrays [][]byte

	for i := 0; i < numLightSections; i++ {
		bit := uint64(1) << uint(i)

		sky := eng.GetSkyLightSection(i)
		if sky != nil && !sky.IsEmpty() {
			skyMask |= bit
			skyArrays = append(skyArrays, sky.Bytes())
		} else {
			emptySkyMask |= bit
		}

		bl := eng.GetBlockLightSection(i)
		if bl != nil && !bl.IsEmpty() {
			blockMask |= bit
			blockArrays = append(blockArrays, bl.Bytes())
		} else {
			emptyBlockMask |= bit
		}
	}

	dst = encodeBitSet(dst, skyMask)
	dst = encodeBitSet(dst, blockMask)
	dst = encodeBitSet(dst, emptySkyMask)
	dst = encodeBitSet(dst, emptyBlockMask)

	dst = appendVarInt(dst, int32(len(skyArrays)))
	for _, arr := range skyArrays {
		dst = appendVarInt(dst, int32(len(arr)))
		dst = append(dst, arr...)
	}

	dst = appendVarInt(dst, int32(len(blockArrays)))
	for _, arr := range blockArrays {
		dst = appendVarInt(dst, int32(len(arr)))
		dst = append(dst, arr...)
	}

	return dst
}

func (c *Chunk) encodeLightDataEmpty(dst []byte) []byte {
	dst = appendVarInt(dst, 1)
	dst = appendInt64BE(dst, 0)

	dst = appendVarInt(dst, 1)
	dst = appendInt64BE(dst, 0)

	allBits := int64((uint64(1) << uint(len(c.Sections)+2)) - 1)
	dst = appendVarInt(dst, 1)
	dst = appendInt64BE(dst, allBits)

	dst = appendVarInt(dst, 1)
	dst = appendInt64BE(dst, allBits)

	dst = appendVarInt(dst, 0)

	dst = appendVarInt(dst, 0)

	return dst
}

func encodeBitSet(dst []byte, bits uint64) []byte {
	if bits == 0 {
		dst = appendVarInt(dst, 0)
		return dst
	}
	dst = appendVarInt(dst, 1)
	dst = appendInt64BE(dst, int64(bits))
	return dst
}

// EncodeLightPayload copies the light masks and arrays for a standalone light update.
func (c *Chunk) EncodeLightPayload() []byte { return c.encodeLightData(nil) }
