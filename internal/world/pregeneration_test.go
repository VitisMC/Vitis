package world

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/persistence"
	"github.com/vitismc/vitis/internal/world/tick"
)

// signalledGenerator observes dispatch while retaining the normal test terrain.
type signalledGenerator struct {
	started chan struct{}
	chunk.Generator
}

// Generate reports the start of work without blocking a generation worker.
func (g signalledGenerator) Generate(x, z int32) (*chunk.Chunk, error) {
	select {
	case g.started <- struct{}{}:
	default:
	}
	return g.Generator.Generate(x, z)
}

// TestPregenerationDispatchesWithinCurrentTick prevents a full idle tick before each worker refill.
func TestPregenerationDispatchesWithinCurrentTick(t *testing.T) {
	started := make(chan struct{}, 1)
	w, err := New(Config{
		Name: "dispatch", ChunkStore: persistence.NewChunkStore(t.TempDir()),
		Generator:        signalledGenerator{started: started, Generator: chunk.NewStubGenerator()},
		ChunkLoadWorkers: 2, ChunkWorkerQueueSize: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := w.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	if err := w.StartPregeneration("square", 0, 0, 16); err != nil {
		t.Fatal(err)
	}
	w.Tick()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("pregeneration workers were left waiting until the next world tick")
	}
	if status := w.PregenerationStatus(); status.Completed != 0 || status.Generated != 0 {
		t.Fatal("dispatch counted a chunk before its write acknowledgement")
	}
}

// TestPregenerationPersistsAndReports verifies that completion means durable chunks.
func TestPregenerationPersistsAndReports(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pregeneration.json")
	store := persistence.NewChunkStore(dir)
	w, err := New(Config{Name: "test", ChunkStore: store, Generator: chunk.NewStubGenerator(), PregenerationStatePath: path, ChunkLoadWorkers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.StartPregeneration("square", 0, 0, 16); err != nil {
		t.Fatal(err)
	}
	if err := w.StartPregeneration("square", 0, 0, 16); err == nil {
		t.Fatal("accepted overlapping job")
	}
	deadline := time.Now().Add(5 * time.Second)
	for w.PregenerationStatus().State != "completed" && time.Now().Before(deadline) {
		w.Tick()
		time.Sleep(time.Millisecond)
	}
	status := w.PregenerationStatus()
	if status.State != "completed" || status.Completed != status.Total || status.Generated != status.Total || status.Average <= 0 || status.Maximum <= 0 {
		t.Fatalf("unexpected completion: %+v", status)
	}
	for z := int32(-1); z <= 1; z++ {
		for x := int32(-1); x <= 1; x++ {
			if !store.HasChunk(int(x), int(z)) {
				t.Fatalf("missing persisted chunk %d,%d", x, z)
			}
		}
	}
	if loaded := w.chunks.Len(); loaded != 0 {
		t.Fatalf("retained %d unused chunks after durable pregeneration", loaded)
	}
	if err := w.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(Config{Name: "test", ChunkStore: persistence.NewChunkStore(dir), PregenerationStatePath: path})
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.PregenerationStatus(); got.State != "completed" || got.Completed != status.Completed {
		t.Fatalf("reopened status: %+v", got)
	}
	if err := reopened.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// TestPregenerationReleaseRequiresExclusiveUse retains chunks requested by runtime users.
func TestPregenerationReleaseRequiresExclusiveUse(t *testing.T) {
	for _, scenario := range []string{"exclusive", "shared", "shared_before_save", "entity", "player", "scheduled", "access", "mutation", "error"} {
		t.Run(scenario, func(t *testing.T) {
			storage := chunk.NewStorage(1)
			manager := chunk.NewManager(chunk.ManagerConfig{Storage: storage, Generator: chunk.NewStubGenerator()})
			defer func() {
				if err := manager.Close(context.Background()); err != nil {
					t.Error(err)
				}
			}()
			manager.SetTick(10)
			if scenario == "shared_before_save" {
				manager.RequestLoad(-1, 2)
			}
			if !manager.RequestPregeneration(-1, 2) {
				t.Fatal("pregeneration request rejected")
			}
			manager.PumpLoadRequests()
			var c *chunk.Chunk
			deadline := time.Now().Add(time.Second)
			for c == nil && time.Now().Before(deadline) {
				manager.ApplyLoadCompletions(1)
				c, _ = manager.GetPregenerationChunk(-1, 2)
				if c == nil {
					time.Sleep(time.Millisecond)
				}
			}
			if c == nil {
				t.Fatal("chunk did not load")
			}
			c.MarkDirty()
			w := &World{chunks: manager, entities: entity.NewManager(), tracker: entity.NewTracker()}
			p := newPregeneration(w, "")
			p.state, p.activeSince = "running", time.Now()
			p.pending[c.Key()] = &pregenerationPosition{X: c.X(), Z: c.Z(), New: true, Requested: true, Saving: true}
			result := chunkSaveResult{snapshot: chunkSaveSnapshot{chunk: c, revision: c.Revision(), accessTick: c.LastAccessTick()}}
			c.BeginSave()
			switch scenario {
			case "shared":
				manager.GetChunk(c.X(), c.Z())
			case "entity":
				w.entities.Add(entity.NewEntity(1, protocol.UUID{}, 0, entity.Vec3{}, entity.Vec2{}))
			case "player":
				w.tracker.AddPlayer(entity.NewPlayer(1, protocol.UUID{}, "test", entity.Vec3{}, entity.Vec2{}, nil, 2))
			case "scheduled":
				w.tickScheduler = tick.NewScheduler()
				w.tickScheduler.Schedule(-16, 64, 32, 0, 100, 0, 0, 0)
			case "access":
				c.Touch(11)
			case "mutation":
				c.MarkDirty()
			case "error":
				result.err = errors.New("disk full")
			}
			(&chunkAutosave{}).acknowledge(result)
			p.onSaved(result)
			if got := manager.IsTracked(c.X(), c.Z()); got != (scenario != "exclusive") {
				t.Fatalf("tracked = %v", got)
			}
			if scenario == "error" {
				if p.state != "paused" || p.task.Completed != 0 || !c.Dirty() || p.pending[c.Key()].Saving {
					t.Fatal("failed write discarded or counted a chunk")
				}
				if err := p.changeState("paused", "running"); err != nil {
					t.Fatal(err)
				}
				result.err = nil
				c.BeginSave()
				(&chunkAutosave{}).acknowledge(result)
				p.onSaved(result)
				if manager.Len() != 0 || p.task.Completed != 1 {
					t.Fatal("resumed save did not release and count the chunk")
				}
			} else if p.task.Completed != 1 || p.task.Generated != 1 {
				t.Fatal("successful write was not counted")
			}
		})
	}
}

// TestPregenerationSaveBudgetTracksQueueCapacity removes the fixed sixteen-snapshot ceiling.
func TestPregenerationSaveBudgetTracksQueueCapacity(t *testing.T) {
	for _, capacity := range []int{7, 64} {
		storage := chunk.NewStorage(25)
		manager := chunk.NewManager(chunk.ManagerConfig{Storage: storage})
		store := persistence.NewChunkStore(t.TempDir())
		w := &World{name: "budget", chunks: manager, chunkStore: store, autosave: &chunkAutosave{queue: make(chan chunkSaveSnapshot, capacity)}}
		p := &pregeneration{w: w, state: "running", pending: make(map[int64]*pregenerationPosition), activeSince: time.Now(), windowSince: time.Now(), lastReport: time.Now(), lastCheckpoint: time.Now(), task: pregenerationCheckpoint{Width: 5, Cursor: 25, Total: 25}}
		for x := int32(0); x < 25; x++ {
			c := chunk.New(x, 0)
			c.MarkDirty()
			storage.Set(c)
			p.pending[c.Key()] = &pregenerationPosition{X: x, Z: 0, New: true, Requested: true}
		}
		p.tick()
		if got, want := len(w.autosave.queue), min(25, capacity); got != want {
			t.Fatalf("capacity %d queued %d, want %d", capacity, got, want)
		}
		if p.task.Completed != 0 || p.task.Generated != 0 {
			t.Fatal("unwritten snapshots counted as completed chunks")
		}
		p.onSaved(chunkSaveResult{snapshot: chunkSaveSnapshot{chunk: chunk.New(0, 0)}, err: errors.New("disk full")})
		if p.state != "paused" || p.task.Completed != 0 || p.task.Generated != 0 || p.err == "" {
			t.Fatal("write failure did not pause without counting the chunk")
		}
		if err := manager.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// TestPregenerationPauseResumeAndCancel verifies durable control operations.
func TestPregenerationPauseResumeAndCancel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pregeneration.json")
	w, err := New(Config{Name: "test", ChunkStore: persistence.NewChunkStore(dir), Generator: chunk.NewStubGenerator(), PregenerationStatePath: path})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.StartPregeneration("circle", -7, -7, 24); err != nil {
		t.Fatal(err)
	}
	if err := w.PausePregeneration(); err != nil {
		t.Fatal(err)
	}
	if w.PregenerationStatus().State != "paused" {
		t.Fatal("job did not pause")
	}
	if err := w.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(Config{Name: "test", ChunkStore: persistence.NewChunkStore(dir), Generator: chunk.NewStubGenerator(), PregenerationStatePath: path})
	if err != nil {
		t.Fatal(err)
	}
	if reopened.PregenerationStatus().State != "paused" {
		t.Fatal("job was not restored paused")
	}
	if err := reopened.ContinuePregeneration(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for reopened.PregenerationStatus().State != "completed" && time.Now().Before(deadline) {
		reopened.Tick()
		time.Sleep(time.Millisecond)
	}
	if status := reopened.PregenerationStatus(); status.State != "completed" || status.Completed != status.Total {
		t.Fatalf("restored job did not finish: %+v", status)
	}
	if err := reopened.StartPregeneration("square", 128, 128, 8); err != nil {
		t.Fatal(err)
	}
	if err := reopened.CancelPregeneration(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("checkpoint still exists: %v", err)
	}
	if err := reopened.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// TestPregenerationSelectionRejectsInvalidBounds verifies coordinate validation.
func TestPregenerationSelectionRejectsInvalidBounds(t *testing.T) {
	w, err := New(Config{Name: "test", ChunkStore: persistence.NewChunkStore(t.TempDir())})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close(context.Background())
	for _, args := range []struct {
		shape        string
		x, z, radius int
	}{{"triangle", 0, 0, 16}, {"square", 0, 0, 0}, {"circle", 29999984, 0, 16}} {
		if err := w.StartPregeneration(args.shape, args.x, args.z, args.radius); err == nil {
			t.Fatalf("accepted invalid selection %+v", args)
		}
	}
	if !insideSelection("circle", -7, -7, 1, -1, -1) {
		t.Fatal("center chunk excluded")
	}
}

// TestPregenerationColumnOrder visits each coordinate once across partial bands.
func TestPregenerationColumnOrder(t *testing.T) {
	for _, order := range []string{"column16", "column32"} {
		for _, width := range []int64{1, 15, 16, 17, 31, 32, 33, 1251} {
			task := pregenerationCheckpoint{Order: order, MinX: -625, MinZ: -625, Width: width}
			seen := make(map[[2]int32]bool)
			for index := int64(0); index < width*width; index++ {
				x, z := candidateForTask(&task, index)
				if x < task.MinX || x >= task.MinX+int32(width) || z < task.MinZ || z >= task.MinZ+int32(width) || seen[[2]int32{x, z}] {
					t.Fatalf("width=%d index=%d duplicate or out-of-bounds (%d,%d)", width, index, x, z)
				}
				seen[[2]int32{x, z}] = true
			}
			if len(seen) != int(width*width) {
				t.Fatalf("width=%d visited=%d", width, len(seen))
			}
		}
	}
}
