package chunk

import "context"

const (
	defaultUnloadTTLTicks         = 20 * 30
	defaultUnloadBatch            = 64
	defaultCompletionApplyBatch   = 256
	defaultRequestPumpBatch       = 256
	defaultUnloadScanBatch        = 512
	defaultInitialStorageCapacity = 2048
)

// ChunkSaveFunc is called with a chunk that needs to be persisted before unload.
type ChunkSaveFunc func(c *Chunk)

// ManagerConfig configures world-owned chunk manager behavior.
type ManagerConfig struct {
	Storage                *Storage
	Loader                 *Loader
	Generator              Generator
	OnUnload               ChunkSaveFunc
	InitialStorageCapacity int
	UnloadTTL              uint64
	UnloadBatch            int
	CompletionApplyBatch   int
	RequestPumpBatch       int
	UnloadScanBatch        int
	UnloadQueueCapacity    int
}

// Manager owns in-memory chunk lifecycle and applies async results on world thread.
type Manager struct {
	storage  *Storage
	loader   *Loader
	onUnload ChunkSaveFunc

	unloadTTL            uint64
	unloadBatch          int
	completionApplyBatch int
	requestPumpBatch     int
	unloadScanBatch      int

	unloadQueue *UnloadQueue

	tick uint64

	completionScratch []LoadResult
	unloadScratch     []int64
	keyScratch        []int64
	scanCursor        int
}

// NewManager creates a chunk manager with internal production defaults.
func NewManager(config ManagerConfig) *Manager {
	storage := config.Storage
	if storage == nil {
		initialCapacity := config.InitialStorageCapacity
		if initialCapacity <= 0 {
			initialCapacity = defaultInitialStorageCapacity
		}
		storage = NewStorage(initialCapacity)
	}

	loader := config.Loader
	if loader == nil {
		loader = NewLoader(LoaderConfig{Generator: config.Generator})
	}

	unloadTTL := config.UnloadTTL
	if unloadTTL == 0 {
		unloadTTL = defaultUnloadTTLTicks
	}

	unloadBatch := config.UnloadBatch
	if unloadBatch <= 0 {
		unloadBatch = defaultUnloadBatch
	}

	completionApplyBatch := config.CompletionApplyBatch
	if completionApplyBatch <= 0 {
		completionApplyBatch = defaultCompletionApplyBatch
	}

	requestPumpBatch := config.RequestPumpBatch
	if requestPumpBatch <= 0 {
		requestPumpBatch = defaultRequestPumpBatch
	}

	unloadScanBatch := config.UnloadScanBatch
	if unloadScanBatch <= 0 {
		unloadScanBatch = defaultUnloadScanBatch
	}

	return &Manager{
		storage:              storage,
		loader:               loader,
		onUnload:             config.OnUnload,
		unloadTTL:            unloadTTL,
		unloadBatch:          unloadBatch,
		completionApplyBatch: completionApplyBatch,
		requestPumpBatch:     requestPumpBatch,
		unloadScanBatch:      unloadScanBatch,
		unloadQueue:          NewUnloadQueue(config.UnloadQueueCapacity),
		completionScratch:    make([]LoadResult, 0, completionApplyBatch),
		unloadScratch:        make([]int64, 0, unloadBatch),
		keyScratch:           make([]int64, 0, defaultInitialStorageCapacity),
	}
}

// Tick advances one world tick and executes chunk pipeline stages.
func (m *Manager) Tick() {
	if m == nil {
		return
	}
	m.tick++
	m.PumpLoadRequests()
	m.ApplyLoadCompletions(m.completionApplyBatch)
	m.UpdateActiveChunks()
	m.ProcessUnloads(m.unloadBatch)
}

// SetTick sets current world tick used by unload policy and access tracking.
func (m *Manager) SetTick(tick uint64) {
	if m == nil {
		return
	}
	m.tick = tick
}

// CurrentTick returns the currently observed world tick.
func (m *Manager) CurrentTick() uint64 {
	if m == nil {
		return 0
	}
	return m.tick
}

// RequestLoad marks and enqueues one chunk for asynchronous loading.
func (m *Manager) RequestLoad(x, z int32) bool {
	if m == nil {
		return false
	}

	key := ChunkKey(x, z)
	existing, exists := m.storage.GetByKey(key)
	createdPlaceholder := false
	previousState := StateLoading

	if exists {
		previousState = existing.State()
		existing.Touch(m.tick)
		switch previousState {
		case StateLoaded, StateLoading:
			return true
		case StateUnloading:
			existing.SetState(StateLoading)
		}
	} else {
		placeholder := NewLoading(x, z)
		placeholder.Touch(m.tick)
		m.storage.SetByKey(key, placeholder)
		createdPlaceholder = true
	}

	if m.loader.Request(x, z) {
		return true
	}

	if createdPlaceholder {
		m.storage.DeleteByKey(key)
		return false
	}

	if existing != nil {
		existing.SetState(previousState)
	}
	return false
}

// GetChunk returns one loaded chunk and updates access metadata.
func (m *Manager) GetChunk(x, z int32) (*Chunk, bool) {
	if m == nil {
		return nil, false
	}
	loaded, ok := m.storage.Get(x, z)
	if !ok || loaded == nil {
		return nil, false
	}
	if loaded.State() != StateLoaded {
		return nil, false
	}
	loaded.Touch(m.tick)
	return loaded, true
}

// MarkForUnload marks one chunk for batched unload processing.
func (m *Manager) MarkForUnload(x, z int32) bool {
	if m == nil {
		return false
	}
	key := ChunkKey(x, z)
	chunk, ok := m.storage.GetByKey(key)
	if !ok || chunk == nil {
		return false
	}
	if chunk.State() == StateUnloading {
		return true
	}
	chunk.SetState(StateUnloading)
	return m.unloadQueue.Enqueue(key)
}

// PumpLoadRequests submits queued load requests to worker pool.
func (m *Manager) PumpLoadRequests() int {
	if m == nil {
		return 0
	}
	return m.loader.Pump(m.requestPumpBatch)
}

// ApplyLoadCompletions applies generated chunks on world thread.
func (m *Manager) ApplyLoadCompletions(max int) int {
	if m == nil {
		return 0
	}
	if max <= 0 {
		max = m.completionApplyBatch
	}
	results := m.loader.DrainCompletions(m.completionScratch[:0], max)
	applied := 0
	for index := range results {
		result := results[index]
		key := ChunkKey(result.X, result.Z)
		current, ok := m.storage.GetByKey(key)
		if !ok || current == nil {
			continue
		}
		if current.State() != StateLoading && current.State() != StateUnloading {
			continue
		}
		if result.Err != nil || result.Chunk == nil {
			m.storage.DeleteByKey(key)
			continue
		}

		loaded := result.Chunk
		loaded.SetState(StateLoaded)
		loaded.Touch(m.tick)
		if current.State() == StateUnloading {
			loaded.SetState(StateUnloading)
			m.unloadQueue.Enqueue(key)
		}
		m.storage.SetByKey(key, loaded)
		applied++
	}
	return applied
}

// UpdateActiveChunks scans loaded chunks and schedules stale chunks for unload.
func (m *Manager) UpdateActiveChunks() int {
	if m == nil || m.unloadTTL == 0 || m.storage.Len() == 0 {
		return 0
	}
	if len(m.keyScratch) == 0 || m.scanCursor >= len(m.keyScratch) {
		m.keyScratch = m.storage.Keys(m.keyScratch)
		m.scanCursor = 0
	}
	if len(m.keyScratch) == 0 {
		return 0
	}

	candidates := 0
	inspected := 0
	for m.scanCursor < len(m.keyScratch) && inspected < m.unloadScanBatch {
		key := m.keyScratch[m.scanCursor]
		m.scanCursor++
		inspected++

		chunk, ok := m.storage.GetByKey(key)
		if !ok || chunk == nil {
			continue
		}
		if chunk.State() != StateLoaded {
			continue
		}
		if m.tick < chunk.LastAccessTick() {
			chunk.Touch(m.tick)
			continue
		}
		if m.tick-chunk.LastAccessTick() < m.unloadTTL {
			continue
		}
		chunk.SetState(StateUnloading)
		if m.unloadQueue.Enqueue(key) {
			candidates++
		}
	}

	if m.scanCursor >= len(m.keyScratch) {
		m.keyScratch = m.keyScratch[:0]
		m.scanCursor = 0
	}
	return candidates
}

// ProcessUnloads removes queued unloading chunks in one batch.
func (m *Manager) ProcessUnloads(max int) int {
	if m == nil {
		return 0
	}
	if max <= 0 {
		max = m.unloadBatch
	}
	keys := m.unloadQueue.Drain(m.unloadScratch[:0], max)
	removed := 0
	for index := range keys {
		key := keys[index]
		chunk, ok := m.storage.GetByKey(key)
		if !ok || chunk == nil {
			continue
		}
		if chunk.State() != StateUnloading {
			continue
		}
		if chunk.Dirty() && m.onUnload != nil {
			m.onUnload(chunk)
			chunk.ClearDirty()
		}
		m.storage.DeleteByKey(key)
		removed++
	}
	return removed
}

// Len returns number of chunks currently tracked in memory.
func (m *Manager) Len() int {
	if m == nil {
		return 0
	}
	return m.storage.Len()
}

// PendingLoadRequests returns request queue depth.
func (m *Manager) PendingLoadRequests() int {
	if m == nil {
		return 0
	}
	return m.loader.PendingRequests()
}

// PendingLoadCompletions returns completion queue depth.
func (m *Manager) PendingLoadCompletions() int {
	if m == nil {
		return 0
	}
	return m.loader.PendingCompletions()
}

// SaveAllDirty persists every in-memory chunk that has been modified.
func (m *Manager) SaveAllDirty() int {
	if m == nil || m.onUnload == nil || m.storage == nil {
		return 0
	}
	saved := 0
	for _, key := range m.storage.Keys(nil) {
		c, ok := m.storage.GetByKey(key)
		if !ok || c == nil || !c.Dirty() {
			continue
		}
		m.onUnload(c)
		c.ClearDirty()
		saved++
	}
	return saved
}

// Close closes owned async resources.
func (m *Manager) Close(ctx context.Context) error {
	if m == nil {
		return nil
	}
	return m.loader.Close(ctx)
}
