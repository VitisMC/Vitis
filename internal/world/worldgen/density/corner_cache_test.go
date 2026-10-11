package density

import (
	"sync"
	"sync/atomic"
	"testing"
)

// TestCornerCacheCoalescesRequests verifies one immutable publication for concurrent callers.
func TestCornerCacheCoalescesRequests(t *testing.T) {
	cache := &cornerColumnCache{limit: 2}
	var calls atomic.Int32
	var workers sync.WaitGroup
	key := cornerColumnKey{x: -16, z: -32, minY: -64, height: 320}
	for range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			values := make([]float64, 3)
			cache.fill(key, values, func(out []float64) { calls.Add(1); copy(out, []float64{1, 2, 3}) })
			if len(values) != 3 || values[2] != 3 {
				t.Error("incomplete publication")
			}
		}()
	}
	workers.Wait()
	if got := calls.Load(); got != 1 {
		t.Fatalf("evaluations %d, want 1", got)
	}
}

// TestCornerCacheEvictsCompletedOnly verifies bounded LRU ordering and protected pending work.
func TestCornerCacheEvictsCompletedOnly(t *testing.T) {
	cache := &cornerColumnCache{limit: 1}
	started, finish, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	pending := cornerColumnKey{x: 1}
	go func() {
		cache.fill(pending, make([]float64, 1), func(out []float64) { close(started); <-finish; out[0] = 1 })
		close(done)
	}()
	<-started
	for _, x := range []int{2, 3} {
		cache.fill(cornerColumnKey{x: x}, make([]float64, 1), func(out []float64) { out[0] = float64(x) })
	}
	cache.mu.Lock()
	entry := cache.entries[pending]
	length := len(cache.entries)
	cache.mu.Unlock()
	if entry == nil || entry.lru != nil || length != 2 {
		t.Fatalf("pending entry lost or completed limit exceeded: length %d", length)
	}
	close(finish)
	<-done
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if len(cache.entries) != 1 || cache.entries[pending] == nil {
		t.Fatal("completed LRU did not retain the most recent publication")
	}
}

// TestCornerCacheReuseKeepsOwnedCopies validates recycling during concurrent eviction.
func TestCornerCacheReuseKeepsOwnedCopies(t *testing.T) {
	cache := &cornerColumnCache{limit: 1}
	var workers sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			for iteration := 0; iteration < 64; iteration++ {
				key := cornerColumnKey{x: iteration % 3, z: worker % 2}
				want := float64(key.x*10 + key.z)
				values := make([]float64, 32)
				cache.fill(key, values, func(out []float64) {
					for i := range out {
						out[i] = want
					}
				})
				cache.fill(cornerColumnKey{x: worker + 100}, make([]float64, 32), func(out []float64) { clear(out) })
				for i, actual := range values {
					if actual != want {
						t.Errorf("owned sample %d = %v, want %v", i, actual, want)
						return
					}
				}
			}
		}(worker)
	}
	workers.Wait()
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if len(cache.spares) > 16 || len(cache.entries) != 1 {
		t.Fatalf("unbounded storage: %d spares, %d entries", len(cache.spares), len(cache.entries))
	}
}
