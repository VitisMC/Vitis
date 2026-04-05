package tick

import (
	"testing"
)

func TestSchedulerBasic(t *testing.T) {
	s := NewScheduler()
	s.Schedule(0, 64, 0, 100, 5, PriorityNormal, TickTypeBlock, 0)

	if s.Pending() != 1 {
		t.Errorf("expected 1 pending, got %d", s.Pending())
	}

	ticks := s.Drain(104, 10)
	if len(ticks) != 0 {
		t.Errorf("expected 0 ticks before target, got %d", len(ticks))
	}

	ticks = s.Drain(105, 10)
	if len(ticks) != 1 {
		t.Errorf("expected 1 tick at target, got %d", len(ticks))
	}
	if ticks[0].X != 0 || ticks[0].Y != 64 || ticks[0].Z != 0 {
		t.Errorf("wrong position")
	}
}

func TestSchedulerPriorityOrder(t *testing.T) {
	s := NewScheduler()
	s.Schedule(0, 64, 0, 100, 5, PriorityNormal, TickTypeBlock, 0)
	s.Schedule(1, 64, 0, 100, 5, PriorityHigh, TickTypeBlock, 0)
	s.Schedule(2, 64, 0, 100, 5, PriorityVeryHigh, TickTypeBlock, 0)

	ticks := s.Drain(105, 10)
	if len(ticks) != 3 {
		t.Fatalf("expected 3 ticks, got %d", len(ticks))
	}

	if ticks[0].X != 2 {
		t.Errorf("expected VeryHigh priority first (x=2), got x=%d", ticks[0].X)
	}
	if ticks[1].X != 1 {
		t.Errorf("expected High priority second (x=1), got x=%d", ticks[1].X)
	}
	if ticks[2].X != 0 {
		t.Errorf("expected Normal priority third (x=0), got x=%d", ticks[2].X)
	}
}

func TestSchedulerTickOrder(t *testing.T) {
	s := NewScheduler()
	s.Schedule(0, 64, 0, 100, 10, PriorityNormal, TickTypeBlock, 0)
	s.Schedule(1, 64, 0, 100, 5, PriorityNormal, TickTypeBlock, 0)
	s.Schedule(2, 64, 0, 100, 15, PriorityNormal, TickTypeBlock, 0)

	ticks := s.Drain(105, 10)
	if len(ticks) != 1 {
		t.Fatalf("expected 1 tick at 105, got %d", len(ticks))
	}
	if ticks[0].X != 1 {
		t.Errorf("expected x=1 first, got x=%d", ticks[0].X)
	}

	ticks = s.Drain(110, 10)
	if len(ticks) != 1 {
		t.Fatalf("expected 1 tick at 110, got %d", len(ticks))
	}
	if ticks[0].X != 0 {
		t.Errorf("expected x=0 second, got x=%d", ticks[0].X)
	}
}

func TestSchedulerNoDuplicates(t *testing.T) {
	s := NewScheduler()
	s.Schedule(0, 64, 0, 100, 5, PriorityNormal, TickTypeBlock, 0)
	s.Schedule(0, 64, 0, 100, 10, PriorityHigh, TickTypeBlock, 0)

	if s.Pending() != 1 {
		t.Errorf("expected 1 pending (no duplicates), got %d", s.Pending())
	}
}

func TestSchedulerIsScheduled(t *testing.T) {
	s := NewScheduler()

	if s.IsScheduled(0, 64, 0, TickTypeBlock, 0) {
		t.Error("should not be scheduled initially")
	}

	s.Schedule(0, 64, 0, 100, 5, PriorityNormal, TickTypeBlock, 0)

	if !s.IsScheduled(0, 64, 0, TickTypeBlock, 0) {
		t.Error("should be scheduled after Schedule")
	}

	s.Drain(105, 10)

	if s.IsScheduled(0, 64, 0, TickTypeBlock, 0) {
		t.Error("should not be scheduled after Drain")
	}
}

func TestSchedulerFluidVsBlock(t *testing.T) {
	s := NewScheduler()
	s.Schedule(0, 64, 0, 100, 5, PriorityNormal, TickTypeBlock, 0)
	s.Schedule(0, 64, 0, 100, 5, PriorityNormal, TickTypeFluid, 1)

	if s.Pending() != 2 {
		t.Errorf("expected 2 pending (block + fluid), got %d", s.Pending())
	}

	if !s.IsScheduled(0, 64, 0, TickTypeBlock, 0) {
		t.Error("block tick should be scheduled")
	}
	if !s.IsScheduled(0, 64, 0, TickTypeFluid, 1) {
		t.Error("fluid tick should be scheduled")
	}
}

func TestSchedulerNoKeyCollision(t *testing.T) {
	s := NewScheduler()
	s.Schedule(10, 64, 20, 100, 5, PriorityNormal, TickTypeFluid, 2)
	s.Schedule(10, 64, 21, 100, 5, PriorityNormal, TickTypeFluid, 2)
	s.Schedule(11, 64, 20, 100, 5, PriorityNormal, TickTypeFluid, 2)
	s.Schedule(10, 65, 20, 100, 5, PriorityNormal, TickTypeFluid, 2)
	s.Schedule(10, 64, 20, 100, 5, PriorityNormal, TickTypeBlock, 2)
	s.Schedule(10, 64, 20, 100, 5, PriorityNormal, TickTypeFluid, 3)

	if s.Pending() != 6 {
		t.Errorf("expected 6 unique ticks, got %d", s.Pending())
	}

	s2 := NewScheduler()
	s2.Schedule(0, 0, 1, 0, 0, PriorityNormal, TickTypeFluid, 0)
	s2.Schedule(1, 0, 0, 0, 0, PriorityNormal, TickTypeFluid, 0)
	if s2.Pending() != 2 {
		t.Errorf("x=0,z=1 and x=1,z=0 must be distinct, got pending=%d", s2.Pending())
	}

	s3 := NewScheduler()
	s3.Schedule(-1, 64, 0, 0, 0, PriorityNormal, TickTypeFluid, 1)
	s3.Schedule(0, 64, -1, 0, 0, PriorityNormal, TickTypeFluid, 1)
	if s3.Pending() != 2 {
		t.Errorf("negative coords must be distinct, got pending=%d", s3.Pending())
	}
}

func TestSchedulerLargeCoordinates(t *testing.T) {
	s := NewScheduler()
	s.Schedule(1000000, 200, -500000, 0, 5, PriorityNormal, TickTypeFluid, 2)
	s.Schedule(1000001, 200, -500000, 0, 5, PriorityNormal, TickTypeFluid, 2)
	s.Schedule(1000000, 200, -499999, 0, 5, PriorityNormal, TickTypeFluid, 2)

	if s.Pending() != 3 {
		t.Errorf("expected 3 unique ticks with large coords, got %d", s.Pending())
	}

	ticks := s.Drain(5, 10)
	if len(ticks) != 3 {
		t.Errorf("expected 3 drained, got %d", len(ticks))
	}
	for _, tk := range ticks {
		if tk.Y != 200 {
			t.Errorf("expected Y=200, got %d", tk.Y)
		}
	}
}

func TestSchedulerMaxCount(t *testing.T) {
	s := NewScheduler()
	for i := 0; i < 10; i++ {
		s.Schedule(i, 64, 0, 100, 5, PriorityNormal, TickTypeBlock, 0)
	}

	ticks := s.Drain(105, 3)
	if len(ticks) != 3 {
		t.Errorf("expected 3 ticks (max), got %d", len(ticks))
	}
	if s.Pending() != 7 {
		t.Errorf("expected 7 remaining, got %d", s.Pending())
	}
}
