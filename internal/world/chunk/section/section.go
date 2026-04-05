package section

const (
	// SectionWidth is the X/Z size of a chunk section.
	SectionWidth = 16
	// SectionHeight is the Y size of a chunk section.
	SectionHeight = 16
	// BlocksPerSection is the total number of blocks in a section.
	BlocksPerSection = SectionWidth * SectionWidth * SectionHeight
	// BiomesPerSection is the total number of biome entries in a section (4×4×4).
	BiomesPerSection = 4 * 4 * 4

	// MinBitsBlock is the minimum bits per entry for indirect block palettes.
	MinBitsBlock = 4
	// MaxBitsBlock is the maximum bits per entry before switching to direct (global) palette.
	MaxBitsBlock = 8
	// DirectBitsBlock is the bits per entry for direct/global block palette (ceil(log2(total_states))).
	DirectBitsBlock = 15

	// MaxBitsBiome is the maximum bits per entry for indirect biome palettes.
	MaxBitsBiome = 3
	// DirectBitsBiome is the bits per entry for direct/global biome palette.
	DirectBitsBiome = 6
)

// Section is one 16×16×16 vertical slice of a chunk.
type Section struct {
	Y          int8
	Blocks     PaletteContainer
	Biomes     PaletteContainer
	BlockLight []byte
	SkyLight   []byte
}

// NewSection creates a section filled with a single block state and biome.
func NewSection(y int8, defaultBlock int32, defaultBiome int32) *Section {
	return &Section{
		Y:      y,
		Blocks: NewSinglePalette(BlocksPerSection, defaultBlock),
		Biomes: NewSinglePalette(BiomesPerSection, defaultBiome),
	}
}

// GetBlock returns the block state ID at the given local coordinates.
func (s *Section) GetBlock(x, y, z int) int32 {
	return s.Blocks.Get(blockIndex(x, y, z))
}

// SetBlock sets the block state ID at the given local coordinates.
func (s *Section) SetBlock(x, y, z int, stateID int32) {
	s.Blocks.Set(blockIndex(x, y, z), stateID)
}

// GetBiome returns the biome ID at the given local biome coordinates (0-3 each).
func (s *Section) GetBiome(x, y, z int) int32 {
	return s.Biomes.Get(biomeIndex(x, y, z))
}

// SetBiome sets the biome ID at the given local biome coordinates (0-3 each).
func (s *Section) SetBiome(x, y, z int, biomeID int32) {
	s.Biomes.Set(biomeIndex(x, y, z), biomeID)
}

// NonAirCount returns the number of non-air blocks (state != 0) in the section.
func (s *Section) NonAirCount() int16 {
	var count int16
	for i := 0; i < BlocksPerSection; i++ {
		if s.Blocks.Get(i) != 0 {
			count++
		}
	}
	return count
}

func blockIndex(x, y, z int) int {
	return (y&0xF)*SectionWidth*SectionWidth + (z&0xF)*SectionWidth + (x & 0xF)
}

func biomeIndex(x, y, z int) int {
	return (y&3)*4*4 + (z&3)*4 + (x & 3)
}
