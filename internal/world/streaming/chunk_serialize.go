package streaming

import (
	"encoding/binary"

	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/lighting"
)

const (
	overworldSections = 24
	lightSections     = overworldSections + 2
	sectionBlockCount = 16 * 16 * 16
	sectionBiomeCount = 4 * 4 * 4
	defaultBiomeID    = 0
)

// SerializeChunkPayload encodes a full chunk data payload (heightmaps, sections, block entities, light).
func SerializeChunkPayload(c *chunk.Chunk) []byte {
	buf := protocol.NewBuffer(4096)

	writeHeightmaps(buf)

	sectionBuf := protocol.NewBuffer(2048)
	chunkSections := c.Sections()

	for i := 0; i < overworldSections; i++ {
		if i < len(chunkSections) {
			encodeSection(sectionBuf, chunkSections[i].Blocks)
		} else {
			encodeEmptySection(sectionBuf)
		}
	}
	sectionData := sectionBuf.Bytes()
	buf.WriteVarInt(int32(len(sectionData)))
	buf.WriteBytes(sectionData)

	buf.WriteVarInt(0)

	writeLightData(buf, c)

	result := make([]byte, len(buf.Bytes()))
	copy(result, buf.Bytes())
	return result
}

func encodeSection(buf *protocol.Buffer, blocks []uint16) {
	if len(blocks) == 0 {
		encodeEmptySection(buf)
		return
	}

	nonAir := int16(0)
	for _, b := range blocks {
		if b != 0 {
			nonAir++
		}
	}
	putInt16BE(buf, nonAir)

	if nonAir == 0 {
		_ = buf.WriteByte(0)
		buf.WriteVarInt(0)
		buf.WriteVarInt(0)
	} else {
		encodePalettedContainer(buf, blocks, sectionBlockCount)
	}

	_ = buf.WriteByte(0)
	buf.WriteVarInt(int32(defaultBiomeID))
	buf.WriteVarInt(0)
}

func encodeEmptySection(buf *protocol.Buffer) {
	putInt16BE(buf, 0)

	_ = buf.WriteByte(0)
	buf.WriteVarInt(0)
	buf.WriteVarInt(0)

	_ = buf.WriteByte(0)
	buf.WriteVarInt(int32(defaultBiomeID))
	buf.WriteVarInt(0)
}

func encodePalettedContainer(buf *protocol.Buffer, entries []uint16, count int) {
	palette := make(map[uint16]int32)
	var paletteList []uint16
	for _, v := range entries {
		if _, exists := palette[v]; !exists {
			palette[v] = int32(len(paletteList))
			paletteList = append(paletteList, v)
		}
	}

	if len(paletteList) == 1 {
		_ = buf.WriteByte(0)
		buf.WriteVarInt(int32(paletteList[0]))
		buf.WriteVarInt(0)
		return
	}

	bitsPerEntry := bitsNeeded(len(paletteList))
	if bitsPerEntry < 4 {
		bitsPerEntry = 4
	}

	_ = buf.WriteByte(byte(bitsPerEntry))
	buf.WriteVarInt(int32(len(paletteList)))
	for _, v := range paletteList {
		buf.WriteVarInt(int32(v))
	}

	entriesPerLong := 64 / bitsPerEntry
	longCount := (count + entriesPerLong - 1) / entriesPerLong
	buf.WriteVarInt(int32(longCount))

	mask := uint64((1 << bitsPerEntry) - 1)
	var currentLong uint64
	bitPos := 0

	for i := 0; i < count; i++ {
		var idx uint64
		if i < len(entries) {
			idx = uint64(palette[entries[i]])
		}
		currentLong |= (idx & mask) << bitPos
		bitPos += bitsPerEntry

		if bitPos >= 64 || i == count-1 {
			putInt64BE(buf, int64(currentLong))
			currentLong = 0
			bitPos = 0
		}
	}
}

func bitsNeeded(paletteSize int) int {
	if paletteSize <= 1 {
		return 0
	}
	bits := 0
	v := paletteSize - 1
	for v > 0 {
		bits++
		v >>= 1
	}
	return bits
}

func writeHeightmaps(buf *protocol.Buffer) {
	zeros := make([]int64, 37)
	hm := nbt.NewCompound()
	hm.PutLongArray("MOTION_BLOCKING", zeros)
	hm.PutLongArray("WORLD_SURFACE", zeros)
	enc := nbt.NewEncoder(512)
	_ = enc.WriteRootCompound(hm)
	buf.WriteBytes(enc.Bytes())
}

func writeLightData(buf *protocol.Buffer, c *chunk.Chunk) {
	gd := c.GameData()
	var eng *lighting.Engine
	if gd != nil {
		eng = gd.LightEngine
	}

	if eng == nil {
		writeLightDataEmpty(buf)
		return
	}

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

	writeBitSet(buf, skyMask)
	writeBitSet(buf, blockMask)
	writeBitSet(buf, emptySkyMask)
	writeBitSet(buf, emptyBlockMask)

	buf.WriteVarInt(int32(len(skyArrays)))
	for _, arr := range skyArrays {
		buf.WriteVarInt(int32(len(arr)))
		buf.WriteBytes(arr)
	}

	buf.WriteVarInt(int32(len(blockArrays)))
	for _, arr := range blockArrays {
		buf.WriteVarInt(int32(len(arr)))
		buf.WriteBytes(arr)
	}
}

func writeLightDataEmpty(buf *protocol.Buffer) {
	buf.WriteVarInt(1)
	putInt64BE(buf, 0)

	buf.WriteVarInt(1)
	putInt64BE(buf, 0)

	allBits := int64((1 << lightSections) - 1)
	buf.WriteVarInt(1)
	putInt64BE(buf, allBits)

	buf.WriteVarInt(1)
	putInt64BE(buf, allBits)

	buf.WriteVarInt(0)
	buf.WriteVarInt(0)
}

func writeBitSet(buf *protocol.Buffer, bits uint64) {
	if bits == 0 {
		buf.WriteVarInt(0)
		return
	}
	buf.WriteVarInt(1)
	putInt64BE(buf, int64(bits))
}

func putInt16BE(buf *protocol.Buffer, v int16) {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], uint16(v))
	buf.WriteBytes(b[:])
}

func putInt64BE(buf *protocol.Buffer, v int64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(v))
	buf.WriteBytes(b[:])
}
