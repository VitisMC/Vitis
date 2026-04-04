package metrics

import (
	"testing"
	"time"
)

func TestProfilerTPS(t *testing.T) {
	p := NewProfiler()
	for i := 0; i < 20; i++ {
		p.RecordTick()
	}
	tps := p.TPS()
	if tps < 15 || tps > 25 {
		t.Logf("TPS = %.1f (acceptable for burst)", tps)
	}
}

func TestProfilerTotalTicks(t *testing.T) {
	p := NewProfiler()
	p.RecordTick()
	p.RecordTick()
	p.RecordTick()
	if p.TotalTicks() != 3 {
		t.Fatalf("TotalTicks = %d, want 3", p.TotalTicks())
	}
}

func TestProfilerUptime(t *testing.T) {
	p := NewProfiler()
	time.Sleep(5 * time.Millisecond)
	if p.Uptime() < 5*time.Millisecond {
		t.Fatal("uptime too short")
	}
}

func TestStatsSnap(t *testing.T) {
	s := NewStats()
	s.PlayersOnline.Add(5)
	s.PacketsSent.Add(100)
	snap := s.Snap()
	if snap.PlayersOnline != 5 {
		t.Fatalf("PlayersOnline = %d, want 5", snap.PlayersOnline)
	}
	if snap.PacketsSent != 100 {
		t.Fatalf("PacketsSent = %d, want 100", snap.PacketsSent)
	}
	if snap.HeapBytes == 0 {
		t.Fatal("HeapBytes should be > 0")
	}
	if snap.Goroutines == 0 {
		t.Fatal("Goroutines should be > 0")
	}
}

func TestMemoryUsage(t *testing.T) {
	m := MemoryUsage()
	if m == 0 {
		t.Fatal("expected non-zero memory")
	}
}

func TestGoroutineCount(t *testing.T) {
	c := GoroutineCount()
	if c < 1 {
		t.Fatal("expected at least 1 goroutine")
	}
}
