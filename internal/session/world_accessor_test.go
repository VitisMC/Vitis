package session

import (
	"testing"

	"github.com/vitismc/vitis/internal/world/chunk"
)

// TestWorldAccessorDistinguishesUnloadedBlocks avoids accepting unavailable cells as air.
func TestWorldAccessorDistinguishesUnloadedBlocks(t *testing.T) {
	storage := chunk.NewStorage(1)
	storage.Set(chunk.New(-1, 0))
	manager := chunk.NewManager(chunk.ManagerConfig{Storage: storage})
	t.Cleanup(func() { _ = manager.Close(nil) })
	accessor := &DefaultWorldAccessor{Chunks: manager}
	if state, ready := accessor.LookupBlockState(-1, 64, 0); !ready || state != 0 {
		t.Fatalf("resident air = %d, available=%v", state, ready)
	}
	if accessor.GetBlock(0, 64, 0) >= 0 {
		t.Fatal("unloaded block returned a valid state")
	}
}

type testWorldCommand struct {
	executed *bool
}

func (c *testWorldCommand) Execute() {
	*c.executed = true
}

// TestWorldAccessorSubmitDefersMutation verifies that commands are owned by the world queue.
func TestWorldAccessorSubmitDefersMutation(t *testing.T) {
	var queued func()
	accessor := &DefaultWorldAccessor{SubmitTask: func(task func()) bool {
		queued = task
		return true
	}}
	executed := false
	if !accessor.Submit(&testWorldCommand{executed: &executed}) {
		t.Fatal("Submit rejected command")
	}
	if executed {
		t.Fatal("command executed in the network caller")
	}
	if queued == nil {
		t.Fatal("command was not queued")
	}
	queued()
	if !executed {
		t.Fatal("queued command was not executed")
	}
}

// TestWorldAccessorSubmitPropagatesBackpressure verifies bounded queue rejection.
func TestWorldAccessorSubmitPropagatesBackpressure(t *testing.T) {
	accessor := &DefaultWorldAccessor{SubmitTask: func(func()) bool { return false }}
	executed := false
	if accessor.Submit(&testWorldCommand{executed: &executed}) {
		t.Fatal("Submit hid queue saturation")
	}
	if executed {
		t.Fatal("rejected command executed")
	}
}

// TestWorldAccessorSubmitRequiresScheduler prevents network handlers from mutating the world inline.
func TestWorldAccessorSubmitRequiresScheduler(t *testing.T) {
	executed := false
	accessor := &DefaultWorldAccessor{}
	if accessor.Submit(&testWorldCommand{executed: &executed}) {
		t.Fatal("Submit accepted a command without a world scheduler")
	}
	if executed {
		t.Fatal("command executed outside the world tick")
	}
}
