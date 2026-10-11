package density

import (
	"container/list"
	"sync"
)

// cornerColumnKey identifies immutable channel samples within one resource snapshot.
type cornerColumnKey struct{ x, z, minY, height int }

// cornerColumnEntry coordinates concurrent requests and owns published column values.
type cornerColumnEntry struct {
	ready   chan struct{}
	values  []float64
	lru     *list.Element
	readers int
	retired bool
}

// cornerColumnCache bounds completed columns and never evicts running evaluations.
type cornerColumnCache struct {
	mu      sync.Mutex
	entries map[cornerColumnKey]*cornerColumnEntry
	order   list.List
	limit   int
	spares  [][]float64
}

// fill copies immutable samples into owned storage and coalesces simultaneous requests.
func (c *cornerColumnCache) fill(key cornerColumnKey, destination []float64, compute func([]float64)) {
	c.mu.Lock()
	if c.entries == nil {
		c.entries = make(map[cornerColumnKey]*cornerColumnEntry)
	}
	if entry := c.entries[key]; entry != nil {
		if entry.lru != nil {
			c.order.MoveToBack(entry.lru)
			copy(destination, entry.values)
			c.mu.Unlock()
			return
		}
		entry.readers++
		c.mu.Unlock()
		<-entry.ready
		copy(destination, entry.values)
		c.release(entry)
		return
	}
	entry := &cornerColumnEntry{ready: make(chan struct{}), readers: 1}
	c.entries[key] = entry
	var values []float64
	if last := len(c.spares) - 1; last >= 0 {
		values = c.spares[last]
		c.spares[last] = nil
		c.spares = c.spares[:last]
	}
	c.mu.Unlock()
	values = resizeFloats(values, len(destination))
	compute(values)
	c.mu.Lock()
	entry.values = values
	entry.lru = c.order.PushBack(key)
	close(entry.ready)
	for c.order.Len() > c.limit {
		oldest := c.order.Front()
		key := oldest.Value.(cornerColumnKey)
		retired := c.entries[key]
		retired.retired = true
		if retired.readers == 0 {
			c.recycle(retired)
		}
		delete(c.entries, key)
		c.order.Remove(oldest)
	}
	c.mu.Unlock()
	copy(destination, values)
	c.release(entry)
}

// release permits recycling only after every pending reader has copied its values.
func (c *cornerColumnCache) release(entry *cornerColumnEntry) {
	c.mu.Lock()
	entry.readers--
	if entry.retired && entry.readers == 0 {
		c.recycle(entry)
	}
	c.mu.Unlock()
}

// recycle retains bounded scratch storage while the cache mutex is held.
func (c *cornerColumnCache) recycle(entry *cornerColumnEntry) {
	if len(c.spares) < 16 {
		c.spares = append(c.spares, entry.values)
	}
	entry.values = nil
}
