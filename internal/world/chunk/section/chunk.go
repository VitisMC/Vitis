package section

import (
	"github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/lighting"
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
	X             int32
	Z             int32
	Sections      []*Section
	BlockEntities map[int64]entity.BlockEntity
	LightEngine   *lighting.Engine
}

// NewChunk creates a chunk filled with air and the given biome.
func NewChunk(x, z int32, numSections int, defaultBiome int32) *Chunk {
	sections := make([]*Section, numSections)
	minY := int8(OverworldMinY / SectionHeight)
	for i := range sections {
		sections[i] = NewSection(minY+int8(i), 0, defaultBiome)
	}
	return &Chunk{X: x, Z: z, Sections: sections}
}

// MinY implements lighting.BlockAccess.
func (c *Chunk) MinY() int { return OverworldMinY }

// MaxY implements lighting.BlockAccess.
func (c *Chunk) MaxY() int { return OverworldMaxY }

// NumSections implements lighting.BlockAccess.
func (c *Chunk) NumSections() int { return len(c.Sections) }

// ComputeLight runs the lighting engine for this chunk column and stores the result.
func (c *Chunk) ComputeLight() {
	eng := lighting.NewEngine(len(c.Sections), OverworldMinY, OverworldMaxY)
	eng.ComputeAll(c)
	c.LightEngine = eng
}

// GetBlock returns the block state ID at the given absolute coordinates.
func (c *Chunk) GetBlock(x, y, z int) int32 {
	if y < OverworldMinY || y > OverworldMaxY {
		return 0
	}
	secIdx := (y - OverworldMinY) / SectionHeight
	if secIdx >= len(c.Sections) {
		return 0
	}
	return c.Sections[secIdx].GetBlock(x, y, z)
}

// SetBlock sets the block state ID at the given absolute coordinates.
func (c *Chunk) SetBlock(x, y, z int, stateID int32) {
	if y < OverworldMinY || y > OverworldMaxY {
		return
	}
	secIdx := (y - OverworldMinY) / SectionHeight
	if secIdx >= len(c.Sections) {
		return
	}
	c.Sections[secIdx].SetBlock(x, y, z, stateID)
}

func blockEntityKey(x, y, z int32) int64 {
	return int64(x)&0xFFFFFFFF | (int64(z)&0xFFFFFFFF)<<16 | (int64(y)&0xFFFF)<<32
}

func (c *Chunk) AddBlockEntity(be entity.BlockEntity) {
	if c.BlockEntities == nil {
		c.BlockEntities = make(map[int64]entity.BlockEntity)
	}
	x, y, z := be.Position()
	c.BlockEntities[blockEntityKey(x, y, z)] = be
}

func (c *Chunk) RemoveBlockEntity(x, y, z int32) {
	if c.BlockEntities == nil {
		return
	}
	delete(c.BlockEntities, blockEntityKey(x, y, z))
}

func (c *Chunk) GetBlockEntity(x, y, z int32) entity.BlockEntity {
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
	zeros := make([]int64, 37)

	hm := nbt.NewCompound()
	hm.PutLongArray("MOTION_BLOCKING", zeros)
	hm.PutLongArray("WORLD_SURFACE", zeros)

	enc := nbt.NewEncoder(512)
	_ = enc.WriteRootCompound(hm)
	return append(dst, enc.Bytes()...)
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

	allBits := int64((1 << LightSections) - 1)
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
