package session

import (
	"encoding/binary"
	"math"

	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

const (
	overworldSections = 24
	lightSections     = overworldSections + 2
)

// buildEmptyChunkPayload builds a pre-serialized payload for an empty (all-air)
// overworld chunk at the given coordinates. The payload contains heightmaps NBT,
// chunk section data, block entities (none), and light data.
func buildEmptyChunkPayload(biomeID int32) []byte {
	buf := protocol.NewBuffer(2048)

	writeEmptyHeightmaps(buf)
	writeEmptyChunkSections(buf, biomeID)

	buf.WriteVarInt(0)

	writeEmptyLightData(buf)

	return copyBytes(buf.Bytes())
}

func writeEmptyHeightmaps(buf *protocol.Buffer) {
	zeros := make([]int64, 37)

	hm := nbt.NewCompound()
	hm.PutLongArray("MOTION_BLOCKING", zeros)
	hm.PutLongArray("WORLD_SURFACE", zeros)

	enc := nbt.NewEncoder(512)
	_ = enc.WriteRootCompound(hm)
	buf.WriteBytes(enc.Bytes())
}

func writeEmptyChunkSections(buf *protocol.Buffer, biomeID int32) {
	sectionBuf := protocol.NewBuffer(512)
	for i := 0; i < overworldSections; i++ {
		writeBigEndianInt16(sectionBuf, 0)

		_ = sectionBuf.WriteByte(0)
		sectionBuf.WriteVarInt(0)
		sectionBuf.WriteVarInt(0)

		_ = sectionBuf.WriteByte(0)
		sectionBuf.WriteVarInt(biomeID)
		sectionBuf.WriteVarInt(0)
	}

	sectionData := sectionBuf.Bytes()
	buf.WriteVarInt(int32(len(sectionData)))
	buf.WriteBytes(sectionData)
}

func writeEmptyLightData(buf *protocol.Buffer) {
	buf.WriteVarInt(1)
	writeBigEndianInt64(buf, 0)

	buf.WriteVarInt(1)
	writeBigEndianInt64(buf, 0)

	allSectionBits := int64((1 << lightSections) - 1)
	buf.WriteVarInt(1)
	writeBigEndianInt64(buf, allSectionBits)

	buf.WriteVarInt(1)
	writeBigEndianInt64(buf, allSectionBits)

	buf.WriteVarInt(0)
	buf.WriteVarInt(0)
}

func writeBigEndianInt16(buf *protocol.Buffer, v int16) {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], uint16(v))
	buf.WriteBytes(b[:])
}

func writeBigEndianInt64(buf *protocol.Buffer, v int64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(v))
	buf.WriteBytes(b[:])
}

func copyBytes(src []byte) []byte {
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}

// buildEmptyChunkSectionData builds the raw section data written in the middle
// of a chunk data payload. Exported for use in chunk streaming serialization.
func buildEmptyChunkSectionData(biomeID int32) []byte {
	sectionBuf := protocol.NewBuffer(512)
	for i := 0; i < overworldSections; i++ {
		writeBigEndianInt16(sectionBuf, 0)

		_ = sectionBuf.WriteByte(0)
		sectionBuf.WriteVarInt(0)
		sectionBuf.WriteVarInt(0)

		_ = sectionBuf.WriteByte(0)
		sectionBuf.WriteVarInt(biomeID)
		sectionBuf.WriteVarInt(0)
	}
	return copyBytes(sectionBuf.Bytes())
}

// Float32bits converts float32 to uint32 for binary encoding.
func float32bits(f float32) uint32 {
	return math.Float32bits(f)
}
