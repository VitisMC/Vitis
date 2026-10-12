package world

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/persistence"
)

// memoryTestGenerator controls availability without blocking the world tick.
type memoryTestGenerator struct {
	*chunk.StubGenerator
	available atomic.Bool
	releases  atomic.Int32
	attempted chan struct{}
}

// TryReleaseGenerationCaches reports busy workers until the test permits reclamation.
func (g *memoryTestGenerator) TryReleaseGenerationCaches() bool {
	select {
	case g.attempted <- struct{}{}:
	default:
	}
	if !g.available.Load() {
		return false
	}
	g.releases.Add(1)
	return true
}

// newMemoryTestWorld builds a persistent world with observable cache reclamation.
func newMemoryTestWorld(t *testing.T) (*World, *memoryTestGenerator) {
	t.Helper()
	g := &memoryTestGenerator{StubGenerator: chunk.NewStubGenerator(), attempted: make(chan struct{}, 1)}
	w, err := New(Config{Name: "memory", Generator: g, ChunkStore: persistence.NewChunkStore(t.TempDir()), ChunkLoadWorkers: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := w.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return w, g
}

// awaitMemoryRelease waits for a bounded worker lifecycle transition.
func awaitMemoryRelease(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("memory reclamation did not finish")
	}
}

// memoryReleaseDone snapshots the completion channel under the lifecycle mutex.
func memoryReleaseDone(p *pregeneration) <-chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.releaseDone
}

// TestPregenerationReleasesMemoryAfterCompletion retains durable chunk accounting.
func TestPregenerationReleasesMemoryAfterCompletion(t *testing.T) {
	w, g := newMemoryTestWorld(t)
	g.available.Store(true)
	if err := w.StartPregeneration("square", 0, 0, 16); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for w.PregenerationStatus().State == "running" && time.Now().Before(deadline) {
		w.Tick()
		time.Sleep(time.Millisecond)
	}
	if status := w.PregenerationStatus(); status.State != "completed" || status.Completed != 9 || status.Generated != 9 {
		t.Fatalf("completion: %+v", status)
	}
	awaitMemoryRelease(t, memoryReleaseDone(w.pregen))
	if g.releases.Load() != 1 || w.chunks.Len() != 0 {
		t.Fatal("completed generation retained caches or runtime chunks")
	}
	if err := w.StartPregeneration("square", 128, -128, 16); err != nil {
		t.Fatal(err)
	}
	if err := w.CancelPregeneration(); err != nil {
		t.Fatal(err)
	}
	awaitMemoryRelease(t, memoryReleaseDone(w.pregen))
	if g.releases.Load() != 2 {
		t.Fatal("subsequent task was not reclaimed")
	}
}

// TestPregenerationResumeCancelsStaleReclamation prevents clearing a resumed task.
func TestPregenerationResumeCancelsStaleReclamation(t *testing.T) {
	w, g := newMemoryTestWorld(t)
	if err := w.StartPregeneration("square", 0, 0, 16); err != nil {
		t.Fatal(err)
	}
	if err := w.PausePregeneration(); err != nil {
		t.Fatal(err)
	}
	awaitMemoryRelease(t, g.attempted)
	paused := memoryReleaseDone(w.pregen)
	if err := w.ContinuePregeneration(); err != nil {
		t.Fatal(err)
	}
	awaitMemoryRelease(t, paused)
	g.available.Store(true)
	if g.releases.Load() != 0 {
		t.Fatal("cancelled reclamation cleared the resumed job")
	}
	if err := w.CancelPregeneration(); err != nil {
		t.Fatal(err)
	}
	awaitMemoryRelease(t, memoryReleaseDone(w.pregen))
	if g.releases.Load() != 1 {
		t.Fatal("cancelled job retained generation caches")
	}
}

// TestPregenerationShutdownJoinsReclamation stops retries while a source stays busy.
func TestPregenerationShutdownJoinsReclamation(t *testing.T) {
	w, g := newMemoryTestWorld(t)
	if err := w.StartPregeneration("square", 0, 0, 16); err != nil {
		t.Fatal(err)
	}
	if err := w.PausePregeneration(); err != nil {
		t.Fatal(err)
	}
	awaitMemoryRelease(t, g.attempted)
	done := memoryReleaseDone(w.pregen)
	if err := w.pregen.stopGenerationRelease(nil); err != nil {
		t.Fatal(err)
	}
	awaitMemoryRelease(t, done)
	if g.releases.Load() != 0 {
		t.Fatal("shutdown released a busy source")
	}
}
