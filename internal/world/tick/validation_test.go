package tick

import (
	"math"
	"testing"

	"github.com/vitismc/vitis/internal/block"
)

// TestScheduleRejectsInvalidWork prevents coordinate truncation and deadline overflow.
func TestScheduleRejectsInvalidWork(t *testing.T) {
	s := NewScheduler()
	s.Schedule(0, 64, 0, 0, -1, PriorityNormal, TickTypeBlock, 0)
	s.Schedule(0, 64, 0, math.MaxUint64, 1, PriorityNormal, TickTypeBlock, 0)
	s.Schedule(0, 64, 0, 0, 1, Priority(4), TickTypeBlock, 0)
	s.Schedule(0, 64, 0, 0, 1, PriorityNormal, TickType(2), 0)
	if s.Pending() != 0 {
		t.Fatal("accepted malformed scheduled work")
	}
}

// TestBlockTickDeduplicatesStateChanges preserves one tick across mutable block properties.
func TestBlockTickDeduplicatesStateChanges(t *testing.T) {
	s := NewScheduler()
	first := block.StateID("minecraft:farmland", map[string]string{"moisture": "0"})
	second := block.StateID("minecraft:farmland", map[string]string{"moisture": "7"})
	s.Schedule(1, 64, 1, 0, 5, PriorityNormal, TickTypeBlock, first)
	s.Schedule(1, 64, 1, 0, 1, PriorityHigh, TickTypeBlock, second)
	if s.Pending() != 1 || !s.IsScheduled(1, 64, 1, TickTypeBlock, second) {
		t.Fatal("a property change duplicated pending work")
	}
	got := s.Drain(5, 10)
	if len(got) != 1 || got[0].TargetTick != 5 {
		t.Fatal("duplicate changed the original deadline")
	}
}
