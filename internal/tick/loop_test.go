package tick

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type testWorld struct {
	ticks atomic.Int64
	delay time.Duration
}

func (w *testWorld) Tick() {
	if w.delay > 0 {
		time.Sleep(w.delay)
	}
	w.ticks.Add(1)
}

type testWorldManager struct {
	ticks  atomic.Int64
	worlds []WorldTicker
}

func (m *testWorldManager) Tick() {
	m.ticks.Add(1)
}

func (m *testWorldManager) Worlds() []WorldTicker {
	return m.worlds
}

func TestLoopTicksWorldsAndScheduler(t *testing.T) {
	worldA := &testWorld{}
	worldB := &testWorld{}
	manager := &testWorldManager{worlds: []WorldTicker{worldA, worldB}}

	loop, err := NewLoop(LoopConfig{
		TargetTPS:          100,
		MaxCatchUpTicks:    5,
		OverloadMode:       OverloadCatchUp,
		CancelPendingTasks: true,
		SchedulerConfig: SchedulerConfig{
			IngressCapacity: 256,
			WheelSize:       128,
		},
	}, manager)
	if err != nil {
		t.Fatalf("new loop failed: %v", err)
	}

	if err := loop.Start(); err != nil {
		t.Fatalf("start loop failed: %v", err)
	}

	runLater := make(chan struct{}, 1)
	if _, err := loop.Scheduler().RunTaskLater(func() {
		runLater <- struct{}{}
	}, 2); err != nil {
		t.Fatalf("schedule task failed: %v", err)
	}

	select {
	case <-runLater:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for scheduled task")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := loop.Stop(ctx); err != nil {
		t.Fatalf("stop loop failed: %v", err)
	}

	if manager.ticks.Load() == 0 {
		t.Fatal("expected world manager ticks to be > 0")
	}
	if worldA.ticks.Load() == 0 || worldB.ticks.Load() == 0 {
		t.Fatalf("expected world ticks to be > 0, got worldA=%d worldB=%d", worldA.ticks.Load(), worldB.ticks.Load())
	}

	snapshot := loop.Timing()
	if snapshot.Tick == 0 {
		t.Fatal("expected timing tick to be > 0")
	}
	if snapshot.SmoothedTPS <= 0 {
		t.Fatalf("expected smoothed tps > 0, got %.2f", snapshot.SmoothedTPS)
	}
}

func TestLoopTimingStability(t *testing.T) {
	loop, err := NewLoop(LoopConfig{
		TargetTPS:          100,
		MaxCatchUpTicks:    3,
		OverloadMode:       OverloadCatchUp,
		CancelPendingTasks: true,
	}, nil)
	if err != nil {
		t.Fatalf("new loop failed: %v", err)
	}

	if err := loop.Start(); err != nil {
		t.Fatalf("start loop failed: %v", err)
	}

	deadline := time.Now().Add(700 * time.Millisecond)
	for time.Now().Before(deadline) {
		if loop.CurrentTick() >= 10 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := loop.Stop(ctx); err != nil {
		t.Fatalf("stop loop failed: %v", err)
	}

	snapshot := loop.Timing()
	if snapshot.Tick < 10 {
		t.Fatalf("expected at least 10 ticks, got %d", snapshot.Tick)
	}

	period := snapshot.TickPeriod()
	if period <= 0 || period > 120*time.Millisecond {
		t.Fatalf("unexpected tick period: %s", period)
	}
}

func TestLoopOverloadTracking(t *testing.T) {
	world := &testWorld{delay: 8 * time.Millisecond}
	manager := &testWorldManager{worlds: []WorldTicker{world}}

	loop, err := NewLoop(LoopConfig{
		TargetTPS:          500,
		MaxCatchUpTicks:    2,
		OverloadMode:       OverloadCatchUp,
		CancelPendingTasks: true,
	}, manager)
	if err != nil {
		t.Fatalf("new loop failed: %v", err)
	}

	if err := loop.Start(); err != nil {
		t.Fatalf("start loop failed: %v", err)
	}

	time.Sleep(80 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := loop.Stop(ctx); err != nil {
		t.Fatalf("stop loop failed: %v", err)
	}

	snapshot := loop.Timing()
	if snapshot.OverloadCount == 0 {
		t.Fatal("expected overload count to be > 0")
	}
}
