package protocol

import "errors"

const maxVarIntBytes = 5

var (
	ErrVarIntIncomplete = errors.New("incomplete varint")
	ErrVarIntTooBig     = errors.New("varint too big")
)

// DecodeVarInt decodes one Minecraft VarInt from src.
func DecodeVarInt(src []byte) (value int32, consumed int, err error) {
	var result int32
	for i := 0; i < maxVarIntBytes; i++ {
		if i >= len(src) {
			return 0, 0, ErrVarIntIncomplete
		}

		b := src[i]
		result |= int32(b&0x7F) << (7 * i)
		if b < 0x80 {
			return result, i + 1, nil
		}
	}

	return 0, 0, ErrVarIntTooBig
}

// EncodeVarInt writes value as Minecraft VarInt into dst and returns bytes written.
func EncodeVarInt(dst []byte, value int32) int {
	uv := uint32(value)
	idx := 0
	for {
		if uv&^uint32(0x7F) == 0 {
			dst[idx] = byte(uv)
			idx++
			return idx
		}

		dst[idx] = byte((uv & 0x7F) | 0x80)
		uv >>= 7
		idx++
	}
}

// AppendVarInt appends value encoded as VarInt to dst.
func AppendVarInt(dst []byte, value int32) []byte {
	buf := [maxVarIntBytes]byte{}
	n := EncodeVarInt(buf[:], value)
	return append(dst, buf[:n]...)
}

// VarIntSize returns how many bytes are required to encode value as VarInt.
func VarIntSize(value int32) int {
	uv := uint32(value)
	if uv == 0 {
		return 1
	}

	size := 0
	for uv != 0 {
		size++
		uv >>= 7
	}

	return size
}
