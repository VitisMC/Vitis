package tick

import "testing"

// TestInactiveDueTicksDoNotBlockActiveChunks preserves paused ticks and global execution order.
func TestInactiveDueTicksDoNotBlockActiveChunks(t *testing.T) {
	s := NewScheduler()
	s.Schedule(-1, 64, 0, 0, 0, PriorityHigh, TickTypeBlock, 1)
	s.Schedule(0, 64, 0, 0, 1, PriorityNormal, TickTypeBlock, 1)
	s.Schedule(16, 64, 0, 0, 1, PriorityHigh, TickTypeBlock, 1)
	active := func(x, z int32) bool { return x >= 0 }
	got := s.DrainEligible(5, 1, active)
	if len(got) != 1 || got[0].X != 16 || s.Pending() != 2 {
		t.Fatalf("unexpected first drain: %+v pending=%d", got, s.Pending())
	}
	got = s.DrainEligible(5, 10, active)
	if len(got) != 1 || got[0].X != 0 || s.Pending() != 1 {
		t.Fatalf("unexpected second drain: %+v pending=%d", got, s.Pending())
	}
	if snapshot := s.SnapshotChunk(-1, 0); len(snapshot) != 1 || snapshot[0].X != -1 {
		t.Fatal("paused tick lost or wrong negative chunk")
	}
	got = s.Drain(6, 10)
	if len(got) != 1 || got[0].X != -1 {
		t.Fatal("reactivated tick did not execute")
	}
}

// TestSchedulerChunkRemovalKeepsOtherChunks preserves unrelated pending work.
func TestSchedulerChunkRemovalKeepsOtherChunks(t *testing.T) {
	s := NewScheduler()
	s.Schedule(-1, 64, -1, 0, 8, PriorityNormal, TickTypeFluid, 2)
	s.Schedule(0, 64, 0, 0, 8, PriorityNormal, TickTypeFluid, 2)
	s.RemoveChunk(-1, -1)
	if s.Pending() != 1 || len(s.SnapshotChunk(0, 0)) != 1 {
		t.Fatal("wrong ticks removed")
	}
}
