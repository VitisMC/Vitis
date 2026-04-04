package tick

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestSchedulerImmediateAndDelayedTask(t *testing.T) {
	s, err := NewScheduler(SchedulerConfig{IngressCapacity: 128, WheelSize: 64})
	if err != nil {
		t.Fatalf("new scheduler failed: %v", err)
	}
	defer func() {
		_ = s.Stop(context.Background(), true)
	}()

	var immediate atomic.Int64
	var delayed atomic.Int64

	if _, err := s.RunTask(func() {
		immediate.Add(1)
	}); err != nil {
		t.Fatalf("run task failed: %v", err)
	}
	if _, err := s.RunTaskLater(func() {
		delayed.Add(1)
	}, 3); err != nil {
		t.Fatalf("run task later failed: %v", err)
	}

	s.Tick(1)
	if immediate.Load() != 1 {
		t.Fatalf("expected immediate task to run once, got %d", immediate.Load())
	}
	if delayed.Load() != 0 {
		t.Fatalf("expected delayed task to not run on tick 1, got %d", delayed.Load())
	}

	s.Tick(2)
	if delayed.Load() != 0 {
		t.Fatalf("expected delayed task to not run on tick 2, got %d", delayed.Load())
	}

	s.Tick(3)
	if delayed.Load() != 1 {
		t.Fatalf("expected delayed task to run on tick 3, got %d", delayed.Load())
	}
}

func TestSchedulerRepeatingTaskAndCancel(t *testing.T) {
	s, err := NewScheduler(SchedulerConfig{IngressCapacity: 128, WheelSize: 64})
	if err != nil {
		t.Fatalf("new scheduler failed: %v", err)
	}
	defer func() {
		_ = s.Stop(context.Background(), true)
	}()

	var runs atomic.Int64
	task, err := s.RunRepeatingTask(func() {
		runs.Add(1)
	}, 2)
	if err != nil {
		t.Fatalf("run repeating task failed: %v", err)
	}

	for i := uint64(1); i <= 5; i++ {
		s.Tick(i)
	}
	if runs.Load() != 2 {
		t.Fatalf("expected repeating task to run twice by tick 5, got %d", runs.Load())
	}

	if !task.Cancel() {
		t.Fatal("expected task cancel to succeed")
	}

	for i := uint64(6); i <= 10; i++ {
		s.Tick(i)
	}
	if runs.Load() != 2 {
		t.Fatalf("expected repeating task to stop after cancel, got %d", runs.Load())
	}
}

func TestSchedulerAsyncTask(t *testing.T) {
	s, err := NewScheduler(SchedulerConfig{IngressCapacity: 128, WheelSize: 64})
	if err != nil {
		t.Fatalf("new scheduler failed: %v", err)
	}
	defer func() {
		_ = s.Stop(context.Background(), true)
	}()

	done := make(chan struct{}, 1)
	if _, err := s.RunAsyncTask(func() {
		done <- struct{}{}
	}); err != nil {
		t.Fatalf("run async task failed: %v", err)
	}

	s.Tick(1)

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("timed out waiting for async task execution")
	}
}
