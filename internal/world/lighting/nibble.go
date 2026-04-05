package lighting

const (
	// NibbleArraySize is the byte size of a nibble array for one chunk section (4096 entries × 4 bits / 8).
	NibbleArraySize = 2048
	// EntriesPerSection is the number of light entries in a section (16×16×16).
	EntriesPerSection = 4096
)

// NibbleArray stores 4096 4-bit values in 2048 bytes.
// Index layout: (y&0xF)<<8 | (z&0xF)<<4 | (x&0xF)
type NibbleArray [NibbleArraySize]byte

// Get returns the 4-bit light level at the given index (0-4095).
func (n *NibbleArray) Get(index int) uint8 {
	if index < 0 || index >= EntriesPerSection {
		return 0
	}
	byteIdx := index >> 1
	if index&1 == 0 {
		return n[byteIdx] & 0x0F
	}
	return (n[byteIdx] >> 4) & 0x0F
}

// Set stores a 4-bit light level at the given index (0-4095).
func (n *NibbleArray) Set(index int, value uint8) {
	if index < 0 || index >= EntriesPerSection {
		return
	}
	byteIdx := index >> 1
	if index&1 == 0 {
		n[byteIdx] = (n[byteIdx] & 0xF0) | (value & 0x0F)
	} else {
		n[byteIdx] = (n[byteIdx] & 0x0F) | ((value & 0x0F) << 4)
	}
}

// GetXYZ returns the light level at section-local coordinates.
func (n *NibbleArray) GetXYZ(x, y, z int) uint8 {
	return n.Get(nibbleIndex(x, y, z))
}

// SetXYZ stores a light level at section-local coordinates.
func (n *NibbleArray) SetXYZ(x, y, z int, value uint8) {
	n.Set(nibbleIndex(x, y, z), value)
}

// IsEmpty returns true if all entries are zero.
func (n *NibbleArray) IsEmpty() bool {
	for _, b := range n {
		if b != 0 {
			return false
		}
	}
	return true
}

// Clear zeroes all entries.
func (n *NibbleArray) Clear() {
	*n = NibbleArray{}
}

// Fill sets all entries to the given value.
func (n *NibbleArray) Fill(value uint8) {
	v := (value & 0x0F) | ((value & 0x0F) << 4)
	for i := range n {
		n[i] = v
	}
}

// Bytes returns the raw byte slice of the nibble array.
func (n *NibbleArray) Bytes() []byte {
	return n[:]
}

func nibbleIndex(x, y, z int) int {
	return (y&0xF)<<8 | (z&0xF)<<4 | (x & 0xF)
}
