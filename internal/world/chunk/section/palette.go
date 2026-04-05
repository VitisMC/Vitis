package section

import "math/bits"

// PaletteContainer stores values using a palette + packed bit array.
// Used for both block states (4096 entries) and biomes (64 entries).
type PaletteContainer struct {
	palette []int32
	ids     map[int32]int32
	data    *BitStorage
	length  int
	mode    paletteMode
}

type paletteMode uint8

const (
	modeSingle   paletteMode = iota
	modeIndirect
	modeDirect
)

// NewSinglePalette creates a container where all entries have the same value.
func NewSinglePalette(length int, value int32) PaletteContainer {
	return PaletteContainer{
		palette: []int32{value},
		ids:     map[int32]int32{value: 0},
		data:    NewBitStorage(0, length, nil),
		length:  length,
		mode:    modeSingle,
	}
}

// NewIndirectPalette creates a container with an explicit palette and packed data.
func NewIndirectPalette(length int, bitsPerEntry int, palette []int32, data []uint64) PaletteContainer {
	ids := make(map[int32]int32, len(palette))
	for i, v := range palette {
		ids[v] = int32(i)
	}
	return PaletteContainer{
		palette: palette,
		ids:     ids,
		data:    NewBitStorage(bitsPerEntry, length, data),
		length:  length,
		mode:    modeIndirect,
	}
}

// NewDirectPalette creates a container that stores global IDs directly (no palette).
func NewDirectPalette(length int, bitsPerEntry int, data []uint64) PaletteContainer {
	return PaletteContainer{
		data:   NewBitStorage(bitsPerEntry, length, data),
		length: length,
		mode:   modeDirect,
	}
}

// Get returns the global value at index i.
func (pc *PaletteContainer) Get(i int) int32 {
	switch pc.mode {
	case modeSingle:
		return pc.palette[0]
	case modeDirect:
		return pc.data.Get(i)
	default:
		idx := pc.data.Get(i)
		if int(idx) < len(pc.palette) {
			return pc.palette[idx]
		}
		return 0
	}
}

// Set stores global value v at index i, resizing the palette if needed.
func (pc *PaletteContainer) Set(i int, v int32) {
	switch pc.mode {
	case modeSingle:
		if pc.palette[0] == v {
			return
		}
		pc.growFromSingle(v)
		pc.Set(i, v)

	case modeIndirect:
		if idx, ok := pc.ids[v]; ok {
			pc.data.Set(i, idx)
			return
		}
		newIdx := int32(len(pc.palette))
		pc.palette = append(pc.palette, v)
		pc.ids[v] = newIdx

		neededBits := bitsFor(len(pc.palette))
		maxBits := pc.maxIndirectBits()

		if neededBits > maxBits {
			pc.promoteToNext(maxBits)
			pc.Set(i, v)
			return
		}

		if neededBits > pc.data.Bits() {
			pc.resizeData(neededBits)
		}
		pc.data.Set(i, newIdx)

	case modeDirect:
		pc.data.Set(i, v)
	}
}

// Palette returns the current palette slice (nil for direct mode).
func (pc *PaletteContainer) Palette() []int32 {
	if pc.mode == modeDirect {
		return nil
	}
	return pc.palette
}

// BitsPerEntry returns the current bits per entry.
func (pc *PaletteContainer) BitsPerEntry() int {
	return pc.data.Bits()
}

// RawData returns the underlying packed uint64 data.
func (pc *PaletteContainer) RawData() []uint64 {
	return pc.data.Raw()
}

// Mode returns the current palette mode.
func (pc *PaletteContainer) Mode() paletteMode {
	return pc.mode
}

// IsSingle returns true if the container is in single-value mode.
func (pc *PaletteContainer) IsSingle() bool {
	return pc.mode == modeSingle
}

// IsBlock returns true if this container holds 4096 entries (block states).
func (pc *PaletteContainer) IsBlock() bool {
	return pc.length == BlocksPerSection
}

func (pc *PaletteContainer) maxIndirectBits() int {
	if pc.length == BlocksPerSection {
		return MaxBitsBlock
	}
	return MaxBitsBiome
}

func (pc *PaletteContainer) directBits() int {
	if pc.length == BlocksPerSection {
		return DirectBitsBlock
	}
	return DirectBitsBiome
}

func (pc *PaletteContainer) minIndirectBits() int {
	if pc.length == BlocksPerSection {
		return MinBitsBlock
	}
	return 1
}

func (pc *PaletteContainer) growFromSingle(newValue int32) {
	oldValue := pc.palette[0]
	minBits := pc.minIndirectBits()

	pc.palette = []int32{oldValue, newValue}
	pc.ids = map[int32]int32{oldValue: 0, newValue: 1}
	pc.mode = modeIndirect

	newData := NewBitStorage(minBits, pc.length, nil)
	pc.data = newData
}

func (pc *PaletteContainer) promoteToNext(currentMaxBits int) {
	if pc.mode == modeIndirect {
		directBits := pc.directBits()
		newData := NewBitStorage(directBits, pc.length, nil)
		for i := 0; i < pc.length; i++ {
			idx := pc.data.Get(i)
			if int(idx) < len(pc.palette) {
				newData.Set(i, pc.palette[idx])
			}
		}
		pc.data = newData
		pc.palette = nil
		pc.ids = nil
		pc.mode = modeDirect
	}
}

func (pc *PaletteContainer) resizeData(newBits int) {
	newData := NewBitStorage(newBits, pc.length, nil)
	for i := 0; i < pc.length; i++ {
		newData.Set(i, pc.data.Get(i))
	}
	pc.data = newData
}

func bitsFor(count int) int {
	if count <= 1 {
		return 0
	}
	return bits.Len(uint(count - 1))
}
