package protocol

import (
	"errors"
	"math"
)

var (
	ErrBufferUnderflow = errors.New("buffer underflow")
	ErrInvalidLength   = errors.New("invalid length")
)

// Buffer is a high-performance read/write buffer with separate read and write indices.
type Buffer struct {
	data []byte
	r    int
	w    int
}

// NewBuffer creates an empty buffer with the provided initial capacity.
func NewBuffer(capacity int) *Buffer {
	if capacity < 0 {
		capacity = 0
	}
	return &Buffer{data: make([]byte, capacity)}
}

// WrapBuffer wraps an existing byte slice for read operations without copying.
func WrapBuffer(data []byte) *Buffer {
	return &Buffer{data: data, w: len(data)}
}

// Reset clears read and write indices while reusing the underlying allocation.
func (b *Buffer) Reset() {
	b.r = 0
	b.w = 0
}

// Bytes returns the written data slice.
func (b *Buffer) Bytes() []byte {
	return b.data[:b.w]
}

// Len returns the number of written bytes.
func (b *Buffer) Len() int {
	return b.w
}

// Cap returns the backing slice capacity.
func (b *Buffer) Cap() int {
	return cap(b.data)
}

// ReadIndex returns the current read index.
func (b *Buffer) ReadIndex() int {
	return b.r
}

// WriteIndex returns the current write index.
func (b *Buffer) WriteIndex() int {
	return b.w
}

// Remaining returns unread bytes count.
func (b *Buffer) Remaining() int {
	return b.w - b.r
}

// RemainingBytes returns a copy of unread bytes.
func (b *Buffer) RemainingBytes() []byte {
	if b.r >= b.w {
		return nil
	}
	out := make([]byte, b.w-b.r)
	copy(out, b.data[b.r:b.w])
	return out
}

// Exhausted reports whether all written data was read.
func (b *Buffer) Exhausted() bool {
	return b.r >= b.w
}

// Ensure grows the underlying storage to fit n additional bytes.
func (b *Buffer) Ensure(n int) {
	required := b.w + n
	if required <= len(b.data) {
		return
	}

	if required <= cap(b.data) {
		b.data = b.data[:required]
		return
	}

	newCap := cap(b.data) << 1
	if newCap < required {
		newCap = required
	}
	if newCap == 0 {
		newCap = required
	}

	next := make([]byte, required, newCap)
	copy(next, b.data[:b.w])
	b.data = next
}

// ReadVarInt reads one Minecraft VarInt from the current read index.
func (b *Buffer) ReadVarInt() (int32, error) {
	value, consumed, err := DecodeVarInt(b.data[b.r:b.w])
	if err != nil {
		if errors.Is(err, ErrVarIntIncomplete) {
			return 0, ErrBufferUnderflow
		}
		return 0, err
	}
	b.r += consumed
	return value, nil
}

// WriteVarInt writes one Minecraft VarInt to the current write index.
func (b *Buffer) WriteVarInt(value int32) {
	b.Ensure(maxVarIntBytes)
	n := EncodeVarInt(b.data[b.w:], value)
	b.w += n
}

// ReadVarLong reads one Minecraft VarLong from the current read index.
func (b *Buffer) ReadVarLong() (int64, error) {
	value, consumed, err := DecodeVarLong(b.data[b.r:b.w])
	if err != nil {
		if errors.Is(err, ErrVarLongIncomplete) {
			return 0, ErrBufferUnderflow
		}
		return 0, err
	}
	b.r += consumed
	return value, nil
}

// WriteVarLong writes one Minecraft VarLong to the current write index.
func (b *Buffer) WriteVarLong(value int64) {
	b.Ensure(maxVarLongBytes)
	n := EncodeVarLong(b.data[b.w:], value)
	b.w += n
}

// ReadBytes reads n bytes without copying.
func (b *Buffer) ReadBytes(n int) ([]byte, error) {
	if n < 0 {
		return nil, ErrInvalidLength
	}
	if n > b.Remaining() {
		return nil, ErrBufferUnderflow
	}
	out := b.data[b.r : b.r+n]
	b.r += n
	return out, nil
}

// WriteBytes writes src to the current write index.
func (b *Buffer) WriteBytes(src []byte) {
	if len(src) == 0 {
		return
	}
	b.Ensure(len(src))
	copy(b.data[b.w:], src)
	b.w += len(src)
}

// ReadString reads a VarInt-prefixed UTF-8 string.
func (b *Buffer) ReadString() (string, error) {
	length, err := b.ReadVarInt()
	if err != nil {
		return "", err
	}
	if length < 0 {
		return "", ErrInvalidLength
	}

	raw, err := b.ReadBytes(int(length))
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// WriteString writes a VarInt-prefixed UTF-8 string.
func (b *Buffer) WriteString(value string) error {
	if len(value) > math.MaxInt32 {
		return ErrInvalidLength
	}
	b.WriteVarInt(int32(len(value)))
	b.Ensure(len(value))
	copy(b.data[b.w:], value)
	b.w += len(value)
	return nil
}

// ReadUint16 reads a big-endian uint16 value.
func (b *Buffer) ReadUint16() (uint16, error) {
	raw, err := b.ReadBytes(2)
	if err != nil {
		return 0, err
	}
	return uint16(raw[0])<<8 | uint16(raw[1]), nil
}

// WriteUint16 writes a big-endian uint16 value.
func (b *Buffer) WriteUint16(value uint16) {
	b.Ensure(2)
	b.data[b.w] = byte(value >> 8)
	b.data[b.w+1] = byte(value)
	b.w += 2
}

// ReadInt64 reads a big-endian int64 value.
func (b *Buffer) ReadInt64() (int64, error) {
	raw, err := b.ReadBytes(8)
	if err != nil {
		return 0, err
	}
	value := int64(raw[0])<<56 |
		int64(raw[1])<<48 |
		int64(raw[2])<<40 |
		int64(raw[3])<<32 |
		int64(raw[4])<<24 |
		int64(raw[5])<<16 |
		int64(raw[6])<<8 |
		int64(raw[7])
	return value, nil
}

// WriteInt64 writes a big-endian int64 value.
func (b *Buffer) WriteInt64(value int64) {
	b.Ensure(8)
	b.data[b.w] = byte(uint64(value) >> 56)
	b.data[b.w+1] = byte(uint64(value) >> 48)
	b.data[b.w+2] = byte(uint64(value) >> 40)
	b.data[b.w+3] = byte(uint64(value) >> 32)
	b.data[b.w+4] = byte(uint64(value) >> 24)
	b.data[b.w+5] = byte(uint64(value) >> 16)
	b.data[b.w+6] = byte(uint64(value) >> 8)
	b.data[b.w+7] = byte(uint64(value))
	b.w += 8
}

// ReadByte reads a single byte from the current read index.
func (b *Buffer) ReadByte() (byte, error) {
	if b.r >= b.w {
		return 0, ErrBufferUnderflow
	}
	v := b.data[b.r]
	b.r++
	return v, nil
}

// WriteByte writes a single byte to the current write index.
func (b *Buffer) WriteByte(value byte) error {
	b.Ensure(1)
	b.data[b.w] = value
	b.w++
	return nil
}

// ReadInt16 reads a big-endian int16 value.
func (b *Buffer) ReadInt16() (int16, error) {
	raw, err := b.ReadBytes(2)
	if err != nil {
		return 0, err
	}
	return int16(uint16(raw[0])<<8 | uint16(raw[1])), nil
}

// WriteInt16 writes a big-endian int16 value.
func (b *Buffer) WriteInt16(value int16) {
	b.Ensure(2)
	b.data[b.w] = byte(uint16(value) >> 8)
	b.data[b.w+1] = byte(value)
	b.w += 2
}

// ReadInt32 reads a big-endian int32 value.
func (b *Buffer) ReadInt32() (int32, error) {
	raw, err := b.ReadBytes(4)
	if err != nil {
		return 0, err
	}
	return int32(uint32(raw[0])<<24 | uint32(raw[1])<<16 | uint32(raw[2])<<8 | uint32(raw[3])), nil
}

// WriteInt32 writes a big-endian int32 value.
func (b *Buffer) WriteInt32(value int32) {
	b.Ensure(4)
	b.data[b.w] = byte(uint32(value) >> 24)
	b.data[b.w+1] = byte(uint32(value) >> 16)
	b.data[b.w+2] = byte(uint32(value) >> 8)
	b.data[b.w+3] = byte(value)
	b.w += 4
}

// ReadFloat32 reads a big-endian IEEE 754 float32 value.
func (b *Buffer) ReadFloat32() (float32, error) {
	raw, err := b.ReadBytes(4)
	if err != nil {
		return 0, err
	}
	bits := uint32(raw[0])<<24 | uint32(raw[1])<<16 | uint32(raw[2])<<8 | uint32(raw[3])
	return math.Float32frombits(bits), nil
}

// WriteFloat32 writes a big-endian IEEE 754 float32 value.
func (b *Buffer) WriteFloat32(value float32) {
	bits := math.Float32bits(value)
	b.Ensure(4)
	b.data[b.w] = byte(bits >> 24)
	b.data[b.w+1] = byte(bits >> 16)
	b.data[b.w+2] = byte(bits >> 8)
	b.data[b.w+3] = byte(bits)
	b.w += 4
}

// ReadFloat64 reads a big-endian IEEE 754 float64 value.
func (b *Buffer) ReadFloat64() (float64, error) {
	raw, err := b.ReadBytes(8)
	if err != nil {
		return 0, err
	}
	bits := uint64(raw[0])<<56 | uint64(raw[1])<<48 | uint64(raw[2])<<40 | uint64(raw[3])<<32 |
		uint64(raw[4])<<24 | uint64(raw[5])<<16 | uint64(raw[6])<<8 | uint64(raw[7])
	return math.Float64frombits(bits), nil
}

// WriteFloat64 writes a big-endian IEEE 754 float64 value.
func (b *Buffer) WriteFloat64(value float64) {
	bits := math.Float64bits(value)
	b.Ensure(8)
	b.data[b.w] = byte(bits >> 56)
	b.data[b.w+1] = byte(bits >> 48)
	b.data[b.w+2] = byte(bits >> 40)
	b.data[b.w+3] = byte(bits >> 32)
	b.data[b.w+4] = byte(bits >> 24)
	b.data[b.w+5] = byte(bits >> 16)
	b.data[b.w+6] = byte(bits >> 8)
	b.data[b.w+7] = byte(bits)
	b.w += 8
}

// ReadBool reads a single byte as a boolean value.
func (b *Buffer) ReadBool() (bool, error) {
	raw, err := b.ReadBytes(1)
	if err != nil {
		return false, err
	}
	return raw[0] != 0, nil
}

// WriteBool writes a boolean as a single byte (0x01=true, 0x00=false).
func (b *Buffer) WriteBool(value bool) {
	b.Ensure(1)
	if value {
		b.data[b.w] = 0x01
	} else {
		b.data[b.w] = 0x00
	}
	b.w++
}

// ReadUUID reads a 128-bit UUID as two big-endian uint64 values (most significant first).
func (b *Buffer) ReadUUID() (UUID, error) {
	raw, err := b.ReadBytes(16)
	if err != nil {
		return UUID{}, err
	}
	var u UUID
	u[0] = uint64(raw[0])<<56 | uint64(raw[1])<<48 | uint64(raw[2])<<40 | uint64(raw[3])<<32 |
		uint64(raw[4])<<24 | uint64(raw[5])<<16 | uint64(raw[6])<<8 | uint64(raw[7])
	u[1] = uint64(raw[8])<<56 | uint64(raw[9])<<48 | uint64(raw[10])<<40 | uint64(raw[11])<<32 |
		uint64(raw[12])<<24 | uint64(raw[13])<<16 | uint64(raw[14])<<8 | uint64(raw[15])
	return u, nil
}

// WriteUUID writes a 128-bit UUID as two big-endian uint64 values (most significant first).
func (b *Buffer) WriteUUID(u UUID) {
	b.Ensure(16)
	b.data[b.w] = byte(u[0] >> 56)
	b.data[b.w+1] = byte(u[0] >> 48)
	b.data[b.w+2] = byte(u[0] >> 40)
	b.data[b.w+3] = byte(u[0] >> 32)
	b.data[b.w+4] = byte(u[0] >> 24)
	b.data[b.w+5] = byte(u[0] >> 16)
	b.data[b.w+6] = byte(u[0] >> 8)
	b.data[b.w+7] = byte(u[0])
	b.data[b.w+8] = byte(u[1] >> 56)
	b.data[b.w+9] = byte(u[1] >> 48)
	b.data[b.w+10] = byte(u[1] >> 40)
	b.data[b.w+11] = byte(u[1] >> 32)
	b.data[b.w+12] = byte(u[1] >> 24)
	b.data[b.w+13] = byte(u[1] >> 16)
	b.data[b.w+14] = byte(u[1] >> 8)
	b.data[b.w+15] = byte(u[1])
	b.w += 16
}
