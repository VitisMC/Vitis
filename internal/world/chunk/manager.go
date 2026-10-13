package chunk

import (
	"context"
	"errors"
	"sync/atomic"
)

const (
	defaultUnloadTTLTicks         = 20 * 30
	defaultUnloadBatch            = 64
	defaultCompletionApplyBatch   = 256
	defaultRequestPumpBatch       = 256
	defaultUnloadScanBatch        = 512
	defaultInitialStorageCapacity = 2048
)

// ChunkSaveFunc is called with a chunk that needs to be persisted before unload.
type ChunkSaveFunc func(c *Chunk) error

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
	storage         *Storage
	loader          *Loader
	onUnload        ChunkSaveFunc
	onLoad          func(int32, int32, error)
	retain          func(int32, int32) bool
	onEvict         func(*Chunk)
	onPublish       func(*Chunk)
	onValidate      func(*Chunk) error
	prepareUnload   func(*Chunk) (bool, error)
	deferredUnloads map[int64]uint64

	unloadTTL            uint64
	unloadBatch          int
	completionApplyBatch int
	requestPumpBatch     int
	unloadScanBatch      int

	unloadQueue *UnloadQueue

	tick atomic.Uint64

	completionScratch []LoadResult
	unloadScratch     []int64
	keyScratch        []int64
	visitScratch      []int64
	scanCursor        int
}

// SetLoadObserver receives load results on the world thread.
func (m *Manager) SetLoadObserver(observer func(int32, int32, error)) {
	if m != nil {
		m.onLoad = observer
	}
}

// VisitResident visits ready chunks, including chunks awaiting durable eviction.
func (m *Manager) VisitResident(visit func(*Chunk)) {
	if m == nil || visit == nil {
		return
	}
	for _, key := range m.storage.Keys(nil) {
		if c, ok := m.storage.GetByKey(key); ok && c != nil && c.Data() != nil && c.State() != StateLoading {
			visit(c)
		}
	}
}

// VisitLoaded calls visit for each loaded chunk on the world thread.
func (m *Manager) VisitLoaded(visit func(*Chunk)) {
	if m == nil || visit == nil {
		return
	}
	m.visitScratch = m.storage.Keys(m.visitScratch)
	for _, key := range m.visitScratch {
		if chunk, ok := m.storage.GetByKey(key); ok && chunk.State() == StateLoaded {
			visit(chunk)
		}
	}
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
		deferredUnloads:      make(map[int64]uint64),
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
	m.tick.Add(1)
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
	m.tick.Store(tick)
}

// CurrentTick returns the currently observed world tick.
func (m *Manager) CurrentTick() uint64 {
	if m == nil {
		return 0
	}
	return m.tick.Load()
}

// RequestLoad marks and enqueues one chunk for asynchronous loading.
func (m *Manager) RequestLoad(x, z int32) bool {
	return m.requestLoad(x, z, false)
}

// RequestPregeneration loads a chunk without acquiring it for runtime simulation.
func (m *Manager) RequestPregeneration(x, z int32) bool {
	return m.requestLoad(x, z, true)
}

// requestLoad preserves exclusive pregeneration ownership until runtime access occurs.
func (m *Manager) requestLoad(x, z int32, pregeneration bool) bool {
	if m == nil {
		return false
	}

	key := ChunkKey(x, z)
	existing, exists := m.storage.GetByKey(key)
	createdPlaceholder := false
	previousState := StateLoading

	if exists {
		if !pregeneration && existing.pregenerationOnly.Load() {
			existing.pregenerationOnly.Store(false)
		}
		previousState = existing.State()
		existing.Touch(m.tick.Load())
		switch previousState {
		case StateLoaded, StateLoading:
			return true
		case StateUnloading:
			existing.SetState(StateLoading)
		}
	} else {
		placeholder := NewLoading(x, z)
		placeholder.pregenerationOnly.Store(pregeneration)
		placeholder.Touch(m.tick.Load())
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

// IsTracked reports whether a chunk is already owned by the runtime without touching it.
func (m *Manager) IsTracked(x, z int32) bool {
	if m == nil {
		return false
	}
	_, ok := m.storage.Get(x, z)
	return ok
}

// ReleaseSavedChunk removes an unused, unchanged chunk after a durable write.
// The caller must run on the world thread and exclude chunks needed by simulation.
func (m *Manager) ReleaseSavedChunk(c *Chunk, revision, accessTick uint64) bool {
	if m == nil || c == nil || !c.pregenerationOnly.Load() || c.State() != StateLoaded || c.Dirty() || c.SavePending() || c.Revision() != revision || c.LastAccessTick() != accessTick || len(c.Entities()) != 0 {
		return false
	}
	tracked, ok := m.storage.GetByKey(c.Key())
	if !ok || tracked != c {
		return false
	}
	if m.retain != nil && m.retain(c.X(), c.Z()) {
		return false
	}
	if m.onEvict != nil {
		m.onEvict(c)
	}
	m.storage.DeleteByKey(c.Key())
	return true
}

// GetChunk returns one loaded chunk and updates access metadata.
func (m *Manager) GetChunk(x, z int32) (*Chunk, bool) {
	return m.getChunk(x, z, false)
}

// GetPregenerationChunk reads a loaded chunk without acquiring it for simulation.
func (m *Manager) GetPregenerationChunk(x, z int32) (*Chunk, bool) {
	return m.getChunk(x, z, true)
}

// getChunk records access and relinquishes exclusive ownership for runtime callers.
func (m *Manager) getChunk(x, z int32, pregeneration bool) (*Chunk, bool) {
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
	if !pregeneration && loaded.pregenerationOnly.Load() {
		loaded.pregenerationOnly.Store(false)
	}
	loaded.Touch(m.tick.Load())
	return loaded, true
}

// Touch updates the access timestamp for a chunk without returning it.
// Used by the streaming system to keep player-visible chunks alive.
func (m *Manager) Touch(x, z int32) {
	if m == nil {
		return
	}
	loaded, ok := m.storage.Get(x, z)
	if !ok || loaded == nil {
		return
	}
	if loaded.State() == StateLoaded {
		if loaded.pregenerationOnly.Load() {
			loaded.pregenerationOnly.Store(false)
		}
		loaded.Touch(m.tick.Load())
	}
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
	if _, pending := m.deferredUnloads[key]; pending {
		return true
	}
	chunk.SetState(StateUnloading)
	if !m.unloadQueue.Enqueue(key) {
		chunk.SetState(StateLoaded)
		return false
	}
	return true
}

// PumpLoadRequests submits queued load requests to worker pool.
func (m *Manager) PumpLoadRequests() int {
	if m == nil {
		return 0
	}
	return m.loader.Pump(m.requestPumpBatch)
}

// SetLoadPriorityCenters prioritizes queued generation around active players.
func (m *Manager) SetLoadPriorityCenters(centers []LoadPriorityCenter) {
	if m == nil || m.loader == nil {
		return
	}
	m.loader.SetPriorityCenters(centers)
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
			if m.onLoad != nil {
				loadErr := result.Err
				if loadErr == nil {
					loadErr = errors.New("chunk loader returned no chunk")
				}
				m.onLoad(result.X, result.Z, loadErr)
			}
			continue
		}

		loaded := result.Chunk
		loaded.pregenerationOnly.Store(current.pregenerationOnly.Load())
		loaded.SetState(StateLoaded)
		loaded.Touch(m.tick.Load())
		if current.State() == StateUnloading {
			loaded.SetState(StateUnloading)
			m.unloadQueue.Enqueue(key)
		}
		if m.onValidate != nil {
			if err := m.onValidate(loaded); err != nil {
				m.storage.DeleteByKey(key)
				if m.onLoad != nil {
					m.onLoad(result.X, result.Z, err)
				}
				continue
			}
		}
		m.storage.SetByKey(key, loaded)
		if m.onPublish != nil {
			m.onPublish(loaded)
		}
		if m.onLoad != nil {
			m.onLoad(result.X, result.Z, nil)
		}
		applied++
	}
	clear(results)
	m.completionScratch = results[:0]
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
		if _, pending := m.deferredUnloads[key]; pending {
			continue
		}
		if m.retain != nil && m.retain(chunk.X(), chunk.Z()) {
			continue
		}
		currentTick := m.tick.Load()
		if currentTick < chunk.LastAccessTick() {
			chunk.Touch(currentTick)
			continue
		}
		if currentTick-chunk.LastAccessTick() < m.unloadTTL {
			continue
		}
		chunk.SetState(StateUnloading)
		if m.unloadQueue.Enqueue(key) {
			candidates++
		} else {
			chunk.SetState(StateLoaded)
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
		accessTick, deferred := m.deferredUnloads[key]
		delete(m.deferredUnloads, key)
		if chunk.State() != StateUnloading && !(deferred && chunk.State() == StateLoaded) {
			continue
		}
		if (deferred && chunk.LastAccessTick() != accessTick) || (m.retain != nil && m.retain(chunk.X(), chunk.Z())) {
			chunk.SetState(StateLoaded)
			continue
		}
		if !deferred && !chunk.Dirty() && chunk.HasScheduledTicks() {
			chunk.MarkDirty()
		}
		if chunk.SavePending() {
			m.deferUnload(chunk)
			continue
		}
		if chunk.Dirty() && m.prepareUnload != nil {
			ready, err := m.prepareUnload(chunk)
			if err != nil {
				chunk.SetState(StateLoaded)
				chunk.Touch(m.tick.Load())
				continue
			}
			if !ready || chunk.Dirty() || chunk.SavePending() {
				m.deferUnload(chunk)
				continue
			}
		} else if chunk.Dirty() && m.onUnload != nil {
			revision := chunk.Revision()
			if err := m.onUnload(chunk); err != nil {
				chunk.SetState(StateLoaded)
				chunk.Touch(m.tick.Load())
				continue
			}
			if !chunk.ClearDirtyIfRevision(revision) {
				m.deferUnload(chunk)
				continue
			}
		}
		if m.onEvict != nil {
			m.onEvict(chunk)
		}
		m.storage.DeleteByKey(key)
		removed++
	}
	return removed
}

// DirtyKeys lists loaded dirty chunks without exposing their mutable state.
func (m *Manager) DirtyKeys(dst []int64) []int64 {
	if m == nil || m.storage == nil {
		return dst[:0]
	}
	dst = dst[:0]
	for _, key := range m.storage.Keys(nil) {
		c, ok := m.storage.GetByKey(key)
		if ok && c != nil && c.State() == StateLoaded && c.Dirty() && !c.SavePending() {
			dst = append(dst, key)
		}
	}
	return dst
}

// DirtyByKey returns the current loaded dirty chunk for an autosave snapshot.
func (m *Manager) DirtyByKey(key int64) *Chunk {
	if m == nil || m.storage == nil {
		return nil
	}
	c, ok := m.storage.GetByKey(key)
	if !ok || c == nil || c.State() != StateLoaded || !c.Dirty() || c.SavePending() {
		return nil
	}
	return c
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
func (m *Manager) SaveAllDirty() (int, error) {
	if m == nil || m.onUnload == nil || m.storage == nil {
		return 0, nil
	}
	saved := 0
	var saveErrs []error
	for _, key := range m.storage.Keys(nil) {
		c, ok := m.storage.GetByKey(key)
		if !ok || c == nil || !c.Dirty() || c.SavePending() {
			continue
		}
		revision := c.Revision()
		if err := m.onUnload(c); err != nil {
			saveErrs = append(saveErrs, err)
			continue
		}
		if !c.ClearDirtyIfRevision(revision) {
			continue
		}
		saved++
	}
	return saved, errors.Join(saveErrs...)
}

// Close closes owned async resources.
func (m *Manager) Close(ctx context.Context) error {
	if m == nil {
		return nil
	}
	return m.loader.Close(ctx)
}
