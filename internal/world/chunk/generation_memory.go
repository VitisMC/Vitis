package chunk

// tryReleaseGenerationCaches forwards optional cache reclamation through adapters.
func tryReleaseGenerationCaches(source any) bool {
	releaser, ok := source.(interface{ TryReleaseGenerationCaches() bool })
	return !ok || releaser.TryReleaseGenerationCaches()
}

// TryReleaseGenerationCaches forwards reclamation to the underlying terrain source.
func (g *WorldgenGenerator) TryReleaseGenerationCaches() bool {
	return g == nil || tryReleaseGenerationCaches(g.source)
}

// TryReleaseGenerationCaches forwards reclamation to the observed generator.
func (g ObservedGenerator) TryReleaseGenerationCaches() bool {
	return tryReleaseGenerationCaches(g.Generator)
}

// TryReleaseGenerationCaches forwards reclamation without affecting persisted chunks.
func (g *PersistentGenerator) TryReleaseGenerationCaches() bool {
	return g == nil || tryReleaseGenerationCaches(g.fallback)
}

// TryReleaseGenerationCaches releases scratch state only after dispatched work drains.
func (l *Loader) TryReleaseGenerationCaches() bool {
	if l == nil {
		return true
	}
	if l.PendingRequests() != 0 || l.Submitted() != l.Completed()+l.Failed() {
		return false
	}
	return tryReleaseGenerationCaches(l.generator)
}

// TryReleaseGenerationCaches releases idle loader caches without unloading runtime chunks.
func (m *Manager) TryReleaseGenerationCaches() bool {
	return m == nil || m.loader.TryReleaseGenerationCaches()
}
