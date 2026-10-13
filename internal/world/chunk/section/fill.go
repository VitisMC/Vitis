package section

// FillBlocks replaces a section from a complete array in block-index order.
// Packed storage is owned by the section; the input may be reused immediately.
func (s *Section) FillBlocks(states *[BlocksPerSection]int32) {
	first := states[0]
	uniform := true
	for _, state := range states[1:] {
		if state != first {
			uniform = false
			break
		}
	}
	if uniform {
		s.Blocks = NewSinglePalette(BlocksPerSection, first)
		return
	}
	indices := make(map[int32]int32, 8)
	palette := make([]int32, 0, 8)
	var packed [BlocksPerSection]int32
	lastState, lastIndex := first, int32(0)
	indices[first] = 0
	palette = append(palette, first)
	for index, state := range states {
		if state != lastState {
			var found bool
			lastIndex, found = indices[state]
			if !found {
				lastIndex = int32(len(palette))
				indices[state] = lastIndex
				palette = append(palette, state)
			}
			lastState = state
		}
		packed[index] = lastIndex
	}
	width := max(MinBitsBlock, bitsFor(len(palette)))
	if width > MaxBitsBlock {
		storage := NewBitStorage(DirectBitsBlock, BlocksPerSection, nil)
		for index, state := range states {
			storage.Set(index, state)
		}
		s.Blocks = PaletteContainer{data: storage, length: BlocksPerSection, mode: modeDirect}
		return
	}
	storage := NewBitStorage(width, BlocksPerSection, nil)
	for index, value := range packed {
		storage.Set(index, value)
	}
	s.Blocks = PaletteContainer{palette: palette, ids: indices, data: storage, length: BlocksPerSection, mode: modeIndirect}
}
