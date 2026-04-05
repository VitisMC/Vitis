package section

import (
	"encoding/binary"
)

// EncodeSectionData writes the network representation of a chunk section to dst.
// Format per section: int16(blockCount) + block_palette_container + biome_palette_container.
func EncodeSectionData(dst []byte, s *Section) []byte {
	nonAir := s.NonAirCount()
	dst = appendInt16BE(dst, nonAir)
	dst = encodePaletteContainer(dst, &s.Blocks, true)
	dst = encodePaletteContainer(dst, &s.Biomes, false)
	return dst
}

func encodePaletteContainer(dst []byte, pc *PaletteContainer, isBlock bool) []byte {
	bpe := pc.BitsPerEntry()

	switch pc.Mode() {
	case modeSingle:
		dst = append(dst, 0)
		dst = appendVarInt(dst, pc.Palette()[0])
		dst = appendVarInt(dst, 0)

	case modeIndirect:
		effectiveBPE := bpe
		if isBlock && effectiveBPE < MinBitsBlock {
			effectiveBPE = MinBitsBlock
		}
		dst = append(dst, byte(effectiveBPE))
		palette := pc.Palette()
		dst = appendVarInt(dst, int32(len(palette)))
		for _, v := range palette {
			dst = appendVarInt(dst, v)
		}
		raw := pc.RawData()
		dst = appendVarInt(dst, int32(len(raw)))
		for _, v := range raw {
			dst = appendInt64BE(dst, int64(v))
		}

	case modeDirect:
		directBits := DirectBitsBlock
		if !isBlock {
			directBits = DirectBitsBiome
		}
		dst = append(dst, byte(directBits))
		raw := pc.RawData()
		dst = appendVarInt(dst, int32(len(raw)))
		for _, v := range raw {
			dst = appendInt64BE(dst, int64(v))
		}
	}
	return dst
}

func appendInt16BE(dst []byte, v int16) []byte {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], uint16(v))
	return append(dst, b[:]...)
}

func appendInt64BE(dst []byte, v int64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(v))
	return append(dst, b[:]...)
}

func appendVarInt(dst []byte, value int32) []byte {
	uv := uint32(value)
	for uv >= 0x80 {
		dst = append(dst, byte(uv&0x7F)|0x80)
		uv >>= 7
	}
	dst = append(dst, byte(uv))
	return dst
}
