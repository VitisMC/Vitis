package section

import "math"

// BitStorage implements the compacted data array used in chunk storage and heightmaps.
// Each entry stores a value of "bits" width. Compatible with Minecraft 1.16+ format.
type BitStorage struct {
	data          []uint64
	mask          uint64
	bits          int
	length        int
	valuesPerLong int
}

// NewBitStorage creates a new BitStorage with the given bits per entry and total entry count.
// If data is non-nil it must have the correct length.
func NewBitStorage(bits, length int, data []uint64) *BitStorage {
	if bits == 0 {
		return &BitStorage{length: length}
	}
	b := &BitStorage{
		mask:          1<<bits - 1,
		bits:          bits,
		length:        length,
		valuesPerLong: 64 / bits,
	}
	dataLen := BitStorageSize(bits, length)
	if data != nil {
		if len(data) != dataLen {
			panic("bitstorage: invalid data length")
		}
		b.data = make([]uint64, dataLen)
		copy(b.data, data)
	} else {
		b.data = make([]uint64, dataLen)
	}
	return b
}

// BitStorageSize returns the number of uint64 values needed to store length entries of bits width.
func BitStorageSize(bits, length int) int {
	if bits == 0 {
		return 0
	}
	valuesPerLong := 64 / bits
	return (length + valuesPerLong - 1) / valuesPerLong
}

func (b *BitStorage) index(n int) (cell, offset int) {
	cell = n / b.valuesPerLong
	offset = (n - cell*b.valuesPerLong) * b.bits
	return
}

// Get returns the value at index i.
func (b *BitStorage) Get(i int) int32 {
	if b.valuesPerLong == 0 {
		return 0
	}
	c, offset := b.index(i)
	return int32(b.data[c] >> offset & b.mask)
}

// Set stores value v at index i.
func (b *BitStorage) Set(i int, v int32) {
	if b.valuesPerLong == 0 {
		return
	}
	c, offset := b.index(i)
	b.data[c] = b.data[c]&(b.mask<<offset^math.MaxUint64) | (uint64(v)&b.mask)<<offset
}

// Len returns the number of stored values.
func (b *BitStorage) Len() int { return b.length }

// Bits returns the bits per entry.
func (b *BitStorage) Bits() int { return b.bits }

// Raw returns the underlying uint64 slice for encoding.
func (b *BitStorage) Raw() []uint64 {
	if b == nil {
		return nil
	}
	return b.data
}
