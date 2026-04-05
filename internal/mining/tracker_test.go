package mining

import "testing"

func TestTrackerStartAndGet(t *testing.T) {
	tr := NewTracker()
	tr.Start(1, 10, 20, 30, 100, 60, true)

	state := tr.Get(1)
	if state == nil {
		t.Fatal("expected dig state, got nil")
	}
	if state.X != 10 || state.Y != 20 || state.Z != 30 {
		t.Errorf("position = (%d,%d,%d), want (10,20,30)", state.X, state.Y, state.Z)
	}
	if state.TotalTicks != 60 {
		t.Errorf("TotalTicks = %d, want 60", state.TotalTicks)
	}
}

func TestTrackerCancel(t *testing.T) {
	tr := NewTracker()
	tr.Start(1, 0, 0, 0, 1, 20, true)

	state := tr.Cancel(1)
	if state == nil {
		t.Fatal("Cancel should return the state")
	}
	if tr.Get(1) != nil {
		t.Error("Get after Cancel should return nil")
	}
}

func TestTrackerTick(t *testing.T) {
	tr := NewTracker()
	tr.Start(1, 0, 0, 0, 1, 10, true)

	changed := tr.Tick()
	if len(changed) == 0 {
		t.Error("first tick should produce a stage change")
	}

	state := tr.Get(1)
	if state.ElapsedTicks != 1 {
		t.Errorf("ElapsedTicks = %d, want 1", state.ElapsedTicks)
	}
}

func TestDigStateDone(t *testing.T) {
	state := &DigState{TotalTicks: 5, ElapsedTicks: 5}
	if !state.Done() {
		t.Error("should be done at elapsed == total")
	}
}

func TestDigStateStage(t *testing.T) {
	state := &DigState{TotalTicks: 10, ElapsedTicks: 5}
	stage := state.Stage()
	if stage != 5 {
		t.Errorf("stage = %d, want 5", stage)
	}

	state.ElapsedTicks = 10
	stage = state.Stage()
	if stage != 9 {
		t.Errorf("stage at 100%% = %d, want 9", stage)
	}
}

func TestTrackerMultiplePlayers(t *testing.T) {
	tr := NewTracker()
	tr.Start(1, 0, 0, 0, 1, 20, true)
	tr.Start(2, 5, 5, 5, 2, 40, true)

	tr.Tick()

	s1 := tr.Get(1)
	s2 := tr.Get(2)
	if s1 == nil || s2 == nil {
		t.Fatal("both states should exist")
	}
	if s1.ElapsedTicks != 1 || s2.ElapsedTicks != 1 {
		t.Error("both should have 1 elapsed tick")
	}
}
