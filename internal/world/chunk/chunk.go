package chunk

import "github.com/vitismc/vitis/internal/world/chunk/section"

const (
	defaultSectionY     = 0
	defaultSectionSize  = 16 * 16 * 16
	defaultSectionCount = 1
)

// State describes the current lifecycle stage of a chunk.
type State uint8

const (
	StateLoading State = iota
	StateLoaded
	StateUnloading
)

// Section contains simplified block storage for one vertical chunk segment.
type Section struct {
	Y      int32
	Blocks []uint16
}

// Chunk stores world-owned mutable chunk state.
type Chunk struct {
	x int32
	z int32

	key int64

	state State

	sections []Section
	entities []uint64

	gameData *section.Chunk
	dirty    bool

	lastAccessTick uint64
}

// ChunkKey packs x and z chunk coordinates into one int64 key.
func ChunkKey(x, z int32) int64 {
	return int64(uint64(uint32(x))<<32 | uint64(uint32(z)))
}

// New creates a chunk with default stub section storage.
func New(x, z int32) *Chunk {
	sections := make([]Section, defaultSectionCount)
	sections[0] = Section{
		Y:      defaultSectionY,
		Blocks: make([]uint16, defaultSectionSize),
	}

	return &Chunk{
		x:        x,
		z:        z,
		key:      ChunkKey(x, z),
		state:    StateLoaded,
		sections: sections,
	}
}

// NewLoading creates a chunk placeholder in loading state.
func NewLoading(x, z int32) *Chunk {
	chunk := New(x, z)
	chunk.state = StateLoading
	return chunk
}

// X returns the chunk x coordinate.
func (c *Chunk) X() int32 {
	if c == nil {
		return 0
	}
	return c.x
}

// Z returns the chunk z coordinate.
func (c *Chunk) Z() int32 {
	if c == nil {
		return 0
	}
	return c.z
}

// Key returns the packed chunk key.
func (c *Chunk) Key() int64 {
	if c == nil {
		return 0
	}
	return c.key
}

// State returns current chunk lifecycle state.
func (c *Chunk) State() State {
	if c == nil {
		return StateLoading
	}
	return c.state
}

// SetState updates chunk lifecycle state.
func (c *Chunk) SetState(state State) {
	if c == nil {
		return
	}
	c.state = state
}

// Touch marks the last world tick that accessed the chunk.
func (c *Chunk) Touch(tick uint64) {
	if c == nil {
		return
	}
	c.lastAccessTick = tick
}

// LastAccessTick returns the last recorded access tick.
func (c *Chunk) LastAccessTick() uint64 {
	if c == nil {
		return 0
	}
	return c.lastAccessTick
}

// Sections returns chunk sections.
func (c *Chunk) Sections() []Section {
	if c == nil {
		return nil
	}
	return c.sections
}

// SetSections replaces section storage.
func (c *Chunk) SetSections(sections []Section) {
	if c == nil {
		return
	}
	c.sections = sections
}

// Entities returns entity identifiers inside this chunk.
func (c *Chunk) Entities() []uint64 {
	if c == nil {
		return nil
	}
	return c.entities
}

// SetEntities replaces entity identifier storage.
func (c *Chunk) SetEntities(entities []uint64) {
	if c == nil {
		return
	}
	c.entities = entities
}

// GameData returns the full game-data chunk (section.Chunk) with PaletteContainer block storage.
func (c *Chunk) GameData() *section.Chunk {
	if c == nil {
		return nil
	}
	return c.gameData
}

// SetGameData replaces the game-data chunk.
func (c *Chunk) SetGameData(data *section.Chunk) {
	if c == nil {
		return
	}
	c.gameData = data
}

// Dirty returns whether the chunk has been modified since last save.
func (c *Chunk) Dirty() bool {
	if c == nil {
		return false
	}
	return c.dirty
}

// MarkDirty flags the chunk as modified.
func (c *Chunk) MarkDirty() {
	if c == nil {
		return
	}
	c.dirty = true
}

// ClearDirty resets the dirty flag after a save.
func (c *Chunk) ClearDirty() {
	if c == nil {
		return
	}
	c.dirty = false
}

// GetBlock returns the block state ID at absolute coordinates, delegating to GameData.
func (c *Chunk) GetBlock(x, y, z int) int32 {
	if c == nil || c.gameData == nil {
		return 0
	}
	return c.gameData.GetBlock(x, y, z)
}

// SetBlock sets the block state ID at absolute coordinates and marks dirty.
func (c *Chunk) SetBlock(x, y, z int, stateID int32) {
	if c == nil || c.gameData == nil {
		return
	}
	c.gameData.SetBlock(x, y, z, stateID)
	c.dirty = true
}

// EncodePacketPayload delegates to the game-data chunk's encoder.
func (c *Chunk) EncodePacketPayload() []byte {
	if c == nil || c.gameData == nil {
		return nil
	}
	return c.gameData.EncodePacketPayload()
}
