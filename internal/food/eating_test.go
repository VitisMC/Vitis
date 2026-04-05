package food

import "testing"

func TestEatingTrackerStartAndGet(t *testing.T) {
	tr := NewEatingTracker()
	props := &Properties{Nutrition: 8, Saturation: 0.8, EatDuration: 32}
	tr.Start(1, "minecraft:cooked_beef", 100, 0, props)

	es := tr.Get(1)
	if es == nil {
		t.Fatal("expected eating state for entity 1")
	}
	if es.Nutrition != 8 {
		t.Errorf("nutrition = %d, want 8", es.Nutrition)
	}
	if es.TotalTicks != 32 {
		t.Errorf("total ticks = %d, want 32", es.TotalTicks)
	}
}

func TestEatingTrackerCancel(t *testing.T) {
	tr := NewEatingTracker()
	props := &Properties{Nutrition: 4, Saturation: 0.3, EatDuration: 32}
	tr.Start(1, "minecraft:apple", 50, 0, props)

	es := tr.Cancel(1)
	if es == nil {
		t.Fatal("expected eating state on cancel")
	}
	if es.ItemName != "minecraft:apple" {
		t.Errorf("item name = %s, want minecraft:apple", es.ItemName)
	}

	if tr.Get(1) != nil {
		t.Error("state should be nil after cancel")
	}
}

func TestEatingTrackerCancelNonExistent(t *testing.T) {
	tr := NewEatingTracker()
	if tr.Cancel(99) != nil {
		t.Error("cancel of non-existent entity should return nil")
	}
}

func TestEatingStateDone(t *testing.T) {
	es := &EatingState{TotalTicks: 32, ElapsedTicks: 31}
	if es.Done() {
		t.Error("should not be done at 31/32")
	}
	es.ElapsedTicks = 32
	if !es.Done() {
		t.Error("should be done at 32/32")
	}
}

func TestEatingTrackerTick(t *testing.T) {
	tr := NewEatingTracker()
	props := &Properties{Nutrition: 4, Saturation: 0.3, EatDuration: 3}
	tr.Start(1, "minecraft:apple", 50, 0, props)

	finished := tr.Tick()
	if len(finished) != 0 {
		t.Error("should not be finished after 1 tick")
	}

	tr.Tick()
	finished = tr.Tick()
	if len(finished) != 1 || finished[0] != 1 {
		t.Errorf("expected entity 1 finished, got %v", finished)
	}
}

func TestEatingTrackerMultiplePlayers(t *testing.T) {
	tr := NewEatingTracker()
	props1 := &Properties{Nutrition: 4, Saturation: 0.3, EatDuration: 2}
	props2 := &Properties{Nutrition: 8, Saturation: 0.8, EatDuration: 4}
	tr.Start(1, "minecraft:apple", 50, 0, props1)
	tr.Start(2, "minecraft:cooked_beef", 100, 0, props2)

	tr.Tick()
	finished := tr.Tick()
	if len(finished) != 1 || finished[0] != 1 {
		t.Errorf("only entity 1 should finish at tick 2, got %v", finished)
	}

	tr.Tick()
	finished = tr.Tick()
	if len(finished) != 1 || finished[0] != 2 {
		t.Errorf("only entity 2 should finish at tick 4, got %v", finished)
	}
}
