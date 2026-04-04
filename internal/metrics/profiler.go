// Package metrics provides server performance tracking.
// TODO: Wire Profiler and Stats into the main server loop and expose via API.
package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// Profiler tracks server performance metrics.
type Profiler struct {
	tickCount    atomic.Int64
	lastTickTime atomic.Int64
	tpsWindow    []int64
	tpsMu        sync.Mutex
	startTime    time.Time
}

// NewProfiler creates a new performance profiler.
func NewProfiler() *Profiler {
	return &Profiler{
		tpsWindow: make([]int64, 0, 20),
		startTime: time.Now(),
	}
}

// RecordTick records that a tick has completed.
func (p *Profiler) RecordTick() {
	now := time.Now().UnixNano()
	p.lastTickTime.Store(now)
	p.tickCount.Add(1)

	p.tpsMu.Lock()
	p.tpsWindow = append(p.tpsWindow, now)
	cutoff := now - int64(time.Second)
	start := 0
	for start < len(p.tpsWindow) && p.tpsWindow[start] < cutoff {
		start++
	}
	if start > 0 {
		p.tpsWindow = p.tpsWindow[start:]
	}
	p.tpsMu.Unlock()
}

// TPS returns the current ticks per second measured over the last second.
func (p *Profiler) TPS() float64 {
	p.tpsMu.Lock()
	defer p.tpsMu.Unlock()

	now := time.Now().UnixNano()
	cutoff := now - int64(time.Second)
	count := 0
	for _, t := range p.tpsWindow {
		if t >= cutoff {
			count++
		}
	}
	return float64(count)
}

// TotalTicks returns the total number of ticks since start.
func (p *Profiler) TotalTicks() int64 {
	return p.tickCount.Load()
}

// Uptime returns how long the profiler has been running.
func (p *Profiler) Uptime() time.Duration {
	return time.Since(p.startTime)
}
