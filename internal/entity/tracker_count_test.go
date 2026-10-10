package entity

import (
	"sync"
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

// TestTrackerCountConcurrentLifecycle avoids sharing the tick-owned player slice with workers.
func TestTrackerCountConcurrentLifecycle(t *testing.T) {
	tracker := NewTracker()
	player := NewPlayer(1, protocol.UUID{1}, "tracked", Vec3{}, Vec2{}, nil, 2)
	var worker sync.WaitGroup
	worker.Add(1)
	go func() {
		defer worker.Done()
		for n := 0; n < 10000; n++ {
			count := tracker.Count()
			if count < 0 || count > 1 {
				t.Error("invalid membership count")
				return
			}
		}
	}()
	for n := 0; n < 1000; n++ {
		tracker.AddPlayer(player)
		if tracker.Count() != 1 {
			t.Fatal("join count missing")
		}
		tracker.RemovePlayer(player.ID())
		if tracker.Count() != 0 {
			t.Fatal("leave count missing")
		}
	}
	worker.Wait()
}
