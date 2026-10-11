package chunk

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/vitismc/vitis/internal/world/chunk/section"
	"github.com/vitismc/vitis/internal/world/tick"
)

// State describes the current lifecycle stage of a chunk.
type State uint8

const (
	StateLoading State = iota
	StateLoaded
	StateUnloading
)

// Chunk stores world-owned mutable chunk state.
type Chunk struct {
	x int32
	z int32

	key int64

	state atomic.Uint32

	entities          []uint64
	entityNBT         []byte
	entitySaveErr     error
	tickScheduler     *tick.Scheduler
	tickClock         func() uint64
	pregenerationOnly atomic.Bool // Runtime access permanently relinquishes early eviction.
	data              atomic.Pointer[section.Chunk]
	dirty             atomic.Bool
	savePending       atomic.Bool
	lightDirty        atomic.Bool
	noSkyLight        bool
	lightObserver     func(*Chunk)
	revision          atomic.Uint64
	packetRevision    atomic.Uint64
	packetMu          sync.Mutex
	packetCache       struct {
		data     *section.Chunk
		revision uint64
		payload  []byte
	}

	lastAccessTick    atomic.Uint64
	inhabitedTime     atomic.Int64
	generationStart   time.Time
	firstAcknowledged atomic.Bool
}

// MarkGenerationStart stores the start time before the generated chunk is published.
func (c *Chunk) MarkGenerationStart(start time.Time) {
	if c != nil {
		c.generationStart = start
	}
}

// FirstAcknowledgedDuration claims the first client acknowledgement of a generated chunk.
func (c *Chunk) FirstAcknowledgedDuration() (time.Duration, bool) {
	if c == nil || c.generationStart.IsZero() || !c.firstAcknowledged.CompareAndSwap(false, true) {
		return 0, false
	}
	return time.Since(c.generationStart), true
}

// ChunkKey packs x and z chunk coordinates into one int64 key.
func ChunkKey(x, z int32) int64 {
	return int64(uint64(uint32(x))<<32 | uint64(uint32(z)))
}

// New creates a loaded chunk with authoritative palette-based section data.
func New(x, z int32) *Chunk {
	chunk := &Chunk{
		x:   x,
		z:   z,
		key: ChunkKey(x, z),
	}
	chunk.data.Store(section.NewChunk(x, z, section.OverworldSections, 0))
	chunk.state.Store(uint32(StateLoaded))
	return chunk
}

// NewLoading creates a chunk placeholder in loading state.
func NewLoading(x, z int32) *Chunk {
	chunk := &Chunk{x: x, z: z, key: ChunkKey(x, z)}
	chunk.state.Store(uint32(StateLoading))
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
	return State(c.state.Load())
}

// SetState updates chunk lifecycle state.
func (c *Chunk) SetState(state State) {
	if c == nil {
		return
	}
	c.state.Store(uint32(state))
}

// Touch marks the last world tick that accessed the chunk.
func (c *Chunk) Touch(tick uint64) {
	if c == nil {
		return
	}
	c.lastAccessTick.Store(tick)
}

// LastAccessTick returns the last recorded access tick.
func (c *Chunk) LastAccessTick() uint64 {
	if c == nil {
		return 0
	}
	return c.lastAccessTick.Load()
}

// Sections returns the authoritative palette-based chunk sections.
func (c *Chunk) Sections() []*section.Section {
	if c == nil {
		return nil
	}
	data := c.data.Load()
	if data == nil {
		return nil
	}
	return data.Sections
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

// Data returns the authoritative palette-based chunk state.
func (c *Chunk) Data() *section.Chunk {
	if c == nil {
		return nil
	}
	return c.data.Load()
}

// SetData replaces the authoritative palette-based chunk state.
func (c *Chunk) SetData(data *section.Chunk) {
	if c == nil {
		return
	}
	inhabited := int64(0)
	if data != nil {
		if root := data.PersistentNBT(); root != nil {
			inhabited, _ = root.GetLong("InhabitedTime")
		}
	}
	c.inhabitedTime.Store(max(0, inhabited))
	c.data.Store(data)
	c.lightDirty.Store(data != nil && data.LightEngine == nil)
	c.revision.Add(1)
	c.packetRevision.Add(1)
}

// Dirty returns whether the chunk has been modified since last save.
func (c *Chunk) Dirty() bool {
	if c == nil {
		return false
	}
	return c.dirty.Load()
}

// SavePending reports whether an immutable autosave snapshot is being written.
func (c *Chunk) SavePending() bool {
	return c != nil && c.savePending.Load()
}

// BeginSave reserves the chunk until the background write is acknowledged.
func (c *Chunk) BeginSave() bool {
	return c != nil && c.savePending.CompareAndSwap(false, true)
}

// FinishSave releases the autosave reservation on the world tick.
func (c *Chunk) FinishSave() {
	if c != nil {
		c.savePending.Store(false)
	}
}

// MarkDirty flags the chunk as modified.
func (c *Chunk) MarkDirty() {
	if c == nil {
		return
	}
	c.revision.Add(1)
	c.packetRevision.Add(1)
	c.dirty.Store(true)
}

// ClearDirty resets the dirty flag after a save.
func (c *Chunk) ClearDirty() {
	if c == nil {
		return
	}
	c.dirty.Store(false)
}

// Revision returns the current mutation revision.
func (c *Chunk) Revision() uint64 {
	if c == nil {
		return 0
	}
	return c.revision.Load()
}

// ClearDirtyIfRevision acknowledges a save only when no later mutation exists.
func (c *Chunk) ClearDirtyIfRevision(revision uint64) bool {
	if c == nil || c.revision.Load() != revision {
		return false
	}
	c.dirty.Store(false)
	if c.revision.Load() != revision {
		c.dirty.Store(true)
		return false
	}
	return true
}

// GetBlock returns the block state ID at absolute coordinates, delegating to GameData.
func (c *Chunk) GetBlock(x, y, z int) int32 {
	if c == nil {
		return 0
	}
	data := c.data.Load()
	if data == nil {
		return 0
	}
	return data.GetBlock(x, y, z)
}

// SetBlock sets the block state ID at absolute coordinates and marks dirty.
func (c *Chunk) SetBlock(x, y, z int, stateID int32) {
	if c == nil {
		return
	}
	data := c.data.Load()
	if data == nil {
		return
	}
	data.SetBlock(x, y, z, stateID)
	c.lightDirty.Store(true)
	c.MarkDirty()
	if c.lightObserver != nil {
		c.lightObserver(c)
	}
}

// EnsureLight recomputes lighting once after one or more block mutations.
func (c *Chunk) EnsureLight() {
	if c == nil {
		return
	}
	data := c.data.Load()
	if data == nil {
		return
	}
	if data.LightEngine != nil && !c.lightDirty.Swap(false) {
		return
	}
	c.RefreshLight()
}

// EncodePacketPayload delegates to the game-data chunk's encoder.
func (c *Chunk) EncodePacketPayload() []byte {
	if c == nil {
		return nil
	}
	c.packetMu.Lock()
	defer c.packetMu.Unlock()
	data := c.data.Load()
	if data == nil {
		return nil
	}
	c.EnsureLight()
	revision := c.packetRevision.Load()
	if c.packetCache.data == data && c.packetCache.revision == revision && c.packetCache.payload != nil {
		return c.packetCache.payload
	}
	payload := data.EncodePacketPayload()
	if c.packetRevision.Load() == revision && c.data.Load() == data {
		c.packetCache.data = data
		c.packetCache.revision = revision
		c.packetCache.payload = payload
	}
	return payload
}
