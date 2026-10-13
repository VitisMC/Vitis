package world

// playerMemoryJoined cancels idle reclamation before the player's chunk loading starts.
func (w *World) playerMemoryJoined() {
	if w.pregen == nil {
		return
	}
	p := w.pregen
	p.mu.Lock()
	defer p.mu.Unlock()
	p.idleChunksPending = false
	if p.releaseForIdle {
		p.cancelGenerationReleaseLocked()
	}
}

// playerMemoryLeft releases generation caches after the last player leaves.
func (w *World) playerMemoryLeft() {
	if w.pregen == nil {
		return
	}
	p := w.pregen
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(w.tracker.Players()) != 0 {
		return
	}
	p.idleChunksPending = true
	if p.state != "running" {
		p.startGenerationReleaseLocked(true)
	}
}

// releaseIdleChunksMemoryLocked returns chunk heap pages after normal unloads finish.
// The world tick retains its existing unload and durable-save policy.
func (p *pregeneration) releaseIdleChunksMemoryLocked() {
	if !p.idleChunksPending || p.w.chunks.Len() != 0 || p.w.chunks.PendingLoadRequests() != 0 || p.w.chunks.PendingLoadCompletions() != 0 || len(p.w.tracker.Players()) != 0 {
		return
	}
	p.idleChunksPending = false
	p.startGenerationReleaseLocked(true)
}
