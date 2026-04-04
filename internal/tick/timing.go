package tick

import (
	"math"
	"sync/atomic"
	"time"
)

const defaultTPSAlpha = 0.2

// TimingSnapshot is an immutable runtime view of tick-loop timing metrics.
type TimingSnapshot struct {
	Tick            uint64
	LastTickNanos   int64
	TickPeriodNanos int64
	SmoothedTPS     float64
	OverloadCount   uint64
}

// LastTickDuration returns the measured duration of the most recent tick.
func (s TimingSnapshot) LastTickDuration() time.Duration {
	return time.Duration(s.LastTickNanos)
}

// TickPeriod returns the measured period between the two latest tick starts.
func (s TimingSnapshot) TickPeriod() time.Duration {
	return time.Duration(s.TickPeriodNanos)
}

// Timing tracks tick-loop timing metrics in a lock-free form.
type Timing struct {
	tick          atomic.Uint64
	lastTickNanos atomic.Int64
	periodNanos   atomic.Int64
	overloadCount atomic.Uint64
	tpsBits       atomic.Uint64
	alpha         float64
}

// NewTiming creates a timing metrics tracker.
func NewTiming(alpha float64) *Timing {
	if alpha <= 0 || alpha > 1 {
		alpha = defaultTPSAlpha
	}
	t := &Timing{alpha: alpha}
	t.tpsBits.Store(math.Float64bits(0))
	return t
}

// RecordTick updates timing metrics for one completed tick.
func (t *Timing) RecordTick(tick uint64, tickDuration time.Duration, tickPeriod time.Duration) {
	if t == nil {
		return
	}

	t.tick.Store(tick)
	t.lastTickNanos.Store(int64(tickDuration))
	t.periodNanos.Store(int64(tickPeriod))

	periodNanos := tickPeriod.Nanoseconds()
	if periodNanos <= 0 {
		return
	}
	instantTPS := float64(time.Second) / float64(periodNanos)

	for {
		currentBits := t.tpsBits.Load()
		current := math.Float64frombits(currentBits)
		next := instantTPS
		if current > 0 {
			next = current + t.alpha*(instantTPS-current)
		}
		if t.tpsBits.CompareAndSwap(currentBits, math.Float64bits(next)) {
			return
		}
	}
}

// IncrementOverload increments the overload counter for an over-budget tick.
func (t *Timing) IncrementOverload() {
	if t == nil {
		return
	}
	t.overloadCount.Add(1)
}

// Snapshot returns a lock-free metrics snapshot.
func (t *Timing) Snapshot() TimingSnapshot {
	if t == nil {
		return TimingSnapshot{}
	}
	return TimingSnapshot{
		Tick:            t.tick.Load(),
		LastTickNanos:   t.lastTickNanos.Load(),
		TickPeriodNanos: t.periodNanos.Load(),
		SmoothedTPS:     math.Float64frombits(t.tpsBits.Load()),
		OverloadCount:   t.overloadCount.Load(),
	}
}
