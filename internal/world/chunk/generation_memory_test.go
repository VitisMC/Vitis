package chunk

import "testing"

// releasableGenerator tracks optional reclamation through the generator wrappers.
type releasableGenerator struct {
	*StubGenerator
	available bool
	releases  int
}

// TryReleaseGenerationCaches models a busy or idle source.
func (g *releasableGenerator) TryReleaseGenerationCaches() bool {
	if !g.available {
		return false
	}
	g.releases++
	return true
}

// TestGenerationCacheReleaseRespectsLoaderWork validates wrapper forwarding and dispatch accounting.
func TestGenerationCacheReleaseRespectsLoaderWork(t *testing.T) {
	source := &releasableGenerator{StubGenerator: NewStubGenerator()}
	wrapped := NewPersistentGenerator(nil, ObservedGenerator{Generator: source}, 0)
	loader := NewLoader(LoaderConfig{Generator: wrapped})
	defer func() {
		if err := loader.Close(nil); err != nil {
			t.Error(err)
		}
	}()
	manager := NewManager(ManagerConfig{Loader: loader})
	if manager.TryReleaseGenerationCaches() {
		t.Fatal("busy underlying generator was released")
	}
	source.available = true
	loader.submitted.Add(3)
	loader.completed.Add(2)
	if manager.TryReleaseGenerationCaches() {
		t.Fatal("pending worker task was released")
	}
	loader.failed.Add(1)
	if !manager.TryReleaseGenerationCaches() || source.releases != 1 {
		t.Fatal("idle source was not reached through both wrappers")
	}
	if !loader.Request(1, 2) || manager.TryReleaseGenerationCaches() {
		t.Fatal("queued generation was released")
	}
	if !tryReleaseGenerationCaches(NewStubGenerator()) {
		t.Fatal("generator without caches should need no reclamation")
	}
}
