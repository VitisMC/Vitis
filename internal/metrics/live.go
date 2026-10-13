package metrics

import (
	"os"
	"runtime"
	runtimemetrics "runtime/metrics"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ChunkActivity holds counters shared by generation workers and chunk streamers.
type ChunkActivity struct {
	mu            sync.Mutex
	generated     uint64
	genNanos      uint64
	lastNanos     uint64
	fullCount     uint64
	fullNanos     uint64
	fullLastNanos uint64
	sent          atomic.Uint64
}

// RecordGeneration records a successfully generated chunk.
func (a *ChunkActivity) RecordGeneration(elapsed time.Duration) {
	if a == nil {
		return
	}
	nanos := uint64(max(elapsed.Nanoseconds(), 0))
	a.mu.Lock()
	a.genNanos += nanos
	a.lastNanos = nanos
	a.generated++
	a.mu.Unlock()
}

// RecordSent records a successfully queued chunk packet.
func (a *ChunkActivity) RecordSent() {
	if a != nil {
		a.sent.Add(1)
	}
}

// RecordFullPath records generation through the first client acknowledgement.
func (a *ChunkActivity) RecordFullPath(elapsed time.Duration) {
	if a == nil {
		return
	}
	nanos := uint64(max(elapsed.Nanoseconds(), 0))
	a.mu.Lock()
	a.fullNanos += nanos
	a.fullLastNanos = nanos
	a.fullCount++
	a.mu.Unlock()
}

// LiveSnapshot is one reporting interval of process and chunk activity.
type LiveSnapshot struct {
	CPUPercent      float64
	RAMBytes        uint64
	GeneratedPerSec float64
	SentPerSec      float64
	AvgChunkMS      float64
	LastChunkMS     float64
	FullPathPerSec  float64
	AvgFullPathMS   float64
	LastFullPathMS  float64
}

// LiveSampler samples process and chunk counters without work on hot paths.
type LiveSampler struct {
	activity      *ChunkActivity
	lastTime      time.Time
	lastCPU       float64
	lastCount     uint64
	lastNanos     uint64
	lastSent      uint64
	lastFullCount uint64
	lastFullNanos uint64
}

// NewLiveSampler creates a sampler for server-wide counters.
func NewLiveSampler(activity *ChunkActivity) *LiveSampler {
	return &LiveSampler{activity: activity}
}

// Sample returns rates since the previous call; the first sample establishes a baseline.
func (s *LiveSampler) Sample() LiveSnapshot {
	now := time.Now()
	cpu := processCPUSeconds()
	result := LiveSnapshot{RAMBytes: processRAMBytes()}
	if s.activity == nil {
		return result
	}
	s.activity.mu.Lock()
	count := s.activity.generated
	nanos := s.activity.genNanos
	last := s.activity.lastNanos
	fullCount := s.activity.fullCount
	fullNanos := s.activity.fullNanos
	fullLast := s.activity.fullLastNanos
	s.activity.mu.Unlock()
	sent := s.activity.sent.Load()
	result.LastChunkMS = float64(last) / 1e6
	result.LastFullPathMS = float64(fullLast) / 1e6
	if !s.lastTime.IsZero() {
		seconds := now.Sub(s.lastTime).Seconds()
		if seconds > 0 {
			result.CPUPercent = max(0, (cpu-s.lastCPU)/seconds*100)
			result.GeneratedPerSec = float64(count-s.lastCount) / seconds
			result.SentPerSec = float64(sent-s.lastSent) / seconds
			result.FullPathPerSec = float64(fullCount-s.lastFullCount) / seconds
			if delta := count - s.lastCount; delta > 0 {
				result.AvgChunkMS = float64(nanos-s.lastNanos) / float64(delta) / 1e6
			}
			if delta := fullCount - s.lastFullCount; delta > 0 {
				result.AvgFullPathMS = float64(fullNanos-s.lastFullNanos) / float64(delta) / 1e6
			}
		}
	}
	s.lastTime, s.lastCPU = now, cpu
	s.lastCount, s.lastNanos, s.lastSent = count, nanos, sent
	s.lastFullCount, s.lastFullNanos = fullCount, fullNanos
	return result
}

// processCPUSeconds returns estimated CPU time consumed by the Go process.
func processCPUSeconds() float64 {
	samples := []runtimemetrics.Sample{{Name: "/cpu/classes/total:cpu-seconds"}, {Name: "/cpu/classes/idle:cpu-seconds"}}
	runtimemetrics.Read(samples)
	if samples[0].Value.Kind() != runtimemetrics.KindFloat64 || samples[1].Value.Kind() != runtimemetrics.KindFloat64 {
		return 0
	}
	return samples[0].Value.Float64() - samples[1].Value.Float64()
}

// processRAMBytes returns resident memory, falling back to Go runtime memory.
func processRAMBytes() uint64 {
	data, err := os.ReadFile("/proc/self/statm")
	if err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 2 {
			if pages, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
				return pages * uint64(os.Getpagesize())
			}
		}
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	return memory.Sys
}
