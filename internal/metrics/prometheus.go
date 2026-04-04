package metrics

import (
	"runtime"
	"sync/atomic"
)

// Stats holds server-wide counters.
type Stats struct {
	PlayersOnline   atomic.Int64
	ChunksLoaded    atomic.Int64
	PacketsSent     atomic.Int64
	PacketsReceived atomic.Int64
}

// NewStats creates a new stats tracker.
func NewStats() *Stats {
	return &Stats{}
}

// MemoryUsage returns current heap allocation in bytes.
func MemoryUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}

// GoroutineCount returns the number of active goroutines.
func GoroutineCount() int {
	return runtime.NumGoroutine()
}

// Snapshot returns a point-in-time copy of stats for reporting.
type Snapshot struct {
	PlayersOnline   int64
	ChunksLoaded    int64
	PacketsSent     int64
	PacketsReceived int64
	HeapBytes       uint64
	Goroutines      int
}

// Snap takes a snapshot of the current stats.
func (s *Stats) Snap() Snapshot {
	return Snapshot{
		PlayersOnline:   s.PlayersOnline.Load(),
		ChunksLoaded:    s.ChunksLoaded.Load(),
		PacketsSent:     s.PacketsSent.Load(),
		PacketsReceived: s.PacketsReceived.Load(),
		HeapBytes:       MemoryUsage(),
		Goroutines:      GoroutineCount(),
	}
}
