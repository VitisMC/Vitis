package worldgen

import (
	"sync"
	"testing"
)

// TestTerrainBufferPoolBoundsReuse checks retention and exclusive concurrent ownership.
func TestTerrainBufferPoolBoundsReuse(t *testing.T) {
	pool := newTerrainBufferPool[int32](2)
	first := make([]int32, 8)
	second := make([]int32, 8)
	third := make([]int32, 8)
	pool.Put(first)
	pool.Put(second)
	pool.Put(third)
	if len(pool.buffers) != 2 {
		t.Fatalf("retained buffers = %d, want 2", len(pool.buffers))
	}
	var owners sync.WaitGroup
	results := make(chan *int32, 2)
	for range 2 {
		owners.Add(1)
		go func() {
			defer owners.Done()
			buffer := pool.Get()
			if buffer == nil {
				return
			}
			results <- &buffer[0]
		}()
	}
	owners.Wait()
	close(results)
	var addresses []*int32
	for address := range results {
		addresses = append(addresses, address)
	}
	if len(addresses) != 2 || addresses[0] == addresses[1] || pool.Get() != nil {
		t.Fatal("buffers were not independently owned")
	}
	var zero terrainBufferPool[int32]
	zero.Put(first)
	if zero.Get() != nil {
		t.Fatal("zero-value pool retained a buffer")
	}
}
