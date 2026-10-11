package chunk

// EnsureSaveHandler supplies persistence when an injected manager has no save adapter.
func (m *Manager) EnsureSaveHandler(save ChunkSaveFunc) {
	if m != nil && m.onUnload == nil {
		m.onUnload = save
	}
}

// IsPregenerationOnly reports exclusive background ownership before runtime access.
func (c *Chunk) IsPregenerationOnly() bool {
	return c != nil && c.pregenerationOnly.Load()
}

// SetRetentionPredicate prevents eviction while runtime tickets retain a chunk.
func (m *Manager) SetRetentionPredicate(retain func(int32, int32) bool) {
	if m != nil {
		m.retain = retain
	}
}

// SetPublishHandler attaches world-owned state after a worker publishes a chunk.
func (m *Manager) SetPublishHandler(handler func(*Chunk)) {
	if m != nil {
		m.onPublish = handler
	}
}

// SetEvictionHandler releases runtime state after a chunk is durably saved.
func (m *Manager) SetEvictionHandler(handler func(*Chunk)) {
	if m != nil {
		m.onEvict = handler
	}
}

// PeekLoaded reads a resident chunk without renewing its inactivity timestamp.
func (m *Manager) PeekLoaded(x, z int32) (*Chunk, bool) {
	if m == nil {
		return nil, false
	}
	c, ok := m.storage.Get(x, z)
	return c, ok && c != nil && c.State() == StateLoaded && c.Data() != nil
}

// SetPublishValidator rejects invalid persisted state before publishing a worker result.
func (m *Manager) SetPublishValidator(validate func(*Chunk) error) {
	if m != nil {
		m.onValidate = validate
	}
}

// SetUnloadPreparer installs a nonblocking persistence adapter before runtime ticking.
// A false result keeps the chunk resident until a later durable acknowledgement.
func (m *Manager) SetUnloadPreparer(prepare func(*Chunk) (bool, error)) {
	if m != nil {
		m.prepareUnload = prepare
	}
}

// deferUnload retains accessible state while rechecking a bounded unload queue.
func (m *Manager) deferUnload(c *Chunk) {
	c.SetState(StateLoaded)
	if m.unloadQueue.Enqueue(c.Key()) {
		m.deferredUnloads[c.Key()] = c.LastAccessTick()
	} else {
		c.Touch(m.tick.Load())
	}
}
