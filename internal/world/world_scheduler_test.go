package world

import (
	"testing"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol"
)

type chunkRateTestSession struct {
	rate         float32
	ok           bool
	viewDistance int32
	viewOK       bool
}

func (s chunkRateTestSession) Send(protocol.Packet) error { return nil }

func (s chunkRateTestSession) ChunkBatchRate() (float32, bool) { return s.rate, s.ok }

func (s chunkRateTestSession) ChunkBatchAcknowledgements() uint64 { return 0 }

func (s chunkRateTestSession) ClientViewDistance() (int32, bool) {
	return s.viewDistance, s.viewOK
}

func TestSchedulerRejectsSaturatedIngress(t *testing.T) {
	s := NewScheduler(1)
	if !s.Schedule(func() {}) {
		t.Fatal("first task was not accepted")
	}
	if s.Schedule(func() {}) {
		t.Fatal("saturated ingress accepted a task")
	}
	if pending := s.Pending(); pending != 1 {
		t.Fatalf("expected one queued task, got %d", pending)
	}
}

func TestWorldSubmitRunsOnTick(t *testing.T) {
	w, err := New(Config{Name: "test"})
	if err != nil {
		t.Fatalf("new world: %v", err)
	}
	defer w.Close(nil)

	runs := 0
	if !w.Submit(func() { runs++ }) {
		t.Fatal("world did not accept queued command")
	}
	if runs != 0 {
		t.Fatal("world command ran outside the tick")
	}
	w.Tick()
	if runs != 1 {
		t.Fatalf("world command runs = %d, want 1", runs)
	}
}

func TestChunkBatchFeedbackRespectsWorldBudget(t *testing.T) {
	w, err := New(Config{Name: "test", ChunkStreamChunksPerTick: 4})
	if err != nil {
		t.Fatalf("new world: %v", err)
	}
	defer w.Close(nil)

	player := entity.NewPlayer(1, protocol.UUID{}, "test", entity.Vec3{}, entity.Vec2{}, chunkRateTestSession{rate: 1.1, ok: true}, 2)
	if got := w.chunkStreamRateFor(player); got != 2 {
		t.Fatalf("rate = %d, want 2", got)
	}

	player = entity.NewPlayer(2, protocol.UUID{}, "test", entity.Vec3{}, entity.Vec2{}, chunkRateTestSession{rate: 100, ok: true}, 2)
	if got := w.chunkStreamRateFor(player); got != 4 {
		t.Fatalf("rate = %d, want configured cap 4", got)
	}

	player = entity.NewPlayer(3, protocol.UUID{}, "test", entity.Vec3{}, entity.Vec2{}, chunkRateTestSession{rate: 0, ok: true}, 2)
	if got := w.chunkStreamRateFor(player); got != 0 {
		t.Fatalf("rate = %d, want paused delivery", got)
	}
}

func TestClientViewDistanceRespectsWorldLimit(t *testing.T) {
	w, err := New(Config{Name: "test", ChunkStreamViewDistance: 10})
	if err != nil {
		t.Fatalf("new world: %v", err)
	}
	defer w.Close(nil)

	player := entity.NewPlayer(1, protocol.UUID{}, "test", entity.Vec3{}, entity.Vec2{}, chunkRateTestSession{viewDistance: 4, viewOK: true}, 10)
	if got := w.clientViewDistanceFor(player); got != 4 {
		t.Fatalf("view distance = %d, want 4", got)
	}

	player = entity.NewPlayer(2, protocol.UUID{}, "test", entity.Vec3{}, entity.Vec2{}, chunkRateTestSession{viewDistance: 32, viewOK: true}, 10)
	if got := w.clientViewDistanceFor(player); got != 10 {
		t.Fatalf("view distance = %d, want configured cap 10", got)
	}
}
