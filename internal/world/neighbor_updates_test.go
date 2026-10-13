package world

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/block/behavior"
	"github.com/vitismc/vitis/internal/world/chunk"
)

// neighborTestWorld builds an isolated world without asynchronous IO.
func neighborTestWorld(t *testing.T) (*World, *chunk.Storage) {
	t.Helper()
	storage := chunk.NewStorage(25)
	storage.Set(chunk.New(0, 0))
	w, err := New(Config{Name: "neighbors", ChunkManager: chunk.NewManager(chunk.ManagerConfig{Storage: storage})})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := w.Close(nil); err != nil {
			t.Error(err)
		}
	})
	if err := w.SetGameRule("randomTickSpeed", "0"); err != nil {
		t.Fatal(err)
	}
	return w, storage
}

// TestNeighborCropRemovalIsDeferredAndIdempotent covers mutation, drops, and repeated work.
func TestNeighborCropRemovalIsDeferredAndIdempotent(t *testing.T) {
	for _, drops := range []string{"true", "false"} {
		t.Run(drops, func(t *testing.T) {
			w, storage := neighborTestWorld(t)
			c, _ := storage.Get(0, 0)
			crop := block.DefaultStateID("minecraft:wheat")
			c.SetBlock(8, 63, 8, block.DefaultStateID("minecraft:farmland"))
			c.SetBlock(8, 64, 8, crop)
			if err := w.SetGameRule("doTileDrops", drops); err != nil {
				t.Fatal(err)
			}
			w.setBlockAt(8, 63, 8, 0)
			w.NotifyNeighbors(8, 63, 8)
			if c.GetBlock(8, 64, 8) != crop || w.ItemEntities().Count() != 0 {
				t.Fatal("notification mutated the crop recursively")
			}
			w.Tick()
			want := 0
			if drops == "true" {
				want = 1
			}
			if c.GetBlock(8, 64, 8) != 0 || w.ItemEntities().Count() != want || w.neighborCount != 0 {
				t.Fatalf("crop=%d drops=%d queued=%d", c.GetBlock(8, 64, 8), w.ItemEntities().Count(), w.neighborCount)
			}
			w.Tick()
			if w.ItemEntities().Count() != want {
				t.Fatal("repeated notification duplicated loot")
			}
		})
	}
}

// neighborOrderBehavior records callbacks for ordered notification tests.
type neighborOrderBehavior struct {
	behavior.DefaultBehavior
	seen []neighborUpdate
	w    *World
}

// OnNeighborUpdate records each source and optionally enqueues chained work.
func (b *neighborOrderBehavior) OnNeighborUpdate(ctx *behavior.Context) {
	b.seen = append(b.seen, neighborUpdate{ctx.X, ctx.Y, ctx.Z, ctx.SourceX, ctx.SourceY, ctx.SourceZ})
	if b.w != nil && len(b.seen) == 1 {
		b.w.enqueueNeighbor(neighborUpdate{x: ctx.X, y: ctx.Y, z: ctx.Z, sourceX: 99})
	}
}

// TestNeighborOrderAndRingWrap preserves FIFO order during chained ring-buffer writes.
func TestNeighborOrderAndRingWrap(t *testing.T) {
	w, storage := neighborTestWorld(t)
	c, _ := storage.Get(0, 0)
	stone := block.DefaultStateID("minecraft:stone")
	id := block.BlockIDFromState(stone)
	old := behavior.Get(id)
	b := &neighborOrderBehavior{}
	behavior.Register(id, b)
	t.Cleanup(func() { behavior.Register(id, old) })
	offsets := [6][3]int{{-1, 0, 0}, {1, 0, 0}, {0, -1, 0}, {0, 1, 0}, {0, 0, -1}, {0, 0, 1}}
	for _, offset := range offsets {
		c.SetBlock(8+offset[0], 64+offset[1], 8+offset[2], stone)
	}
	w.NotifyNeighbors(8, 64, 8)
	w.processNeighborUpdates()
	if len(b.seen) != 6 {
		t.Fatalf("callbacks=%d", len(b.seen))
	}
	for i, offset := range offsets {
		if got := b.seen[i]; got.x != 8+offset[0] || got.y != 64+offset[1] || got.z != 8+offset[2] || got.sourceX != 8 || got.sourceY != 64 || got.sourceZ != 8 {
			t.Fatalf("notification %d = %+v", i, got)
		}
	}
	b.seen, b.w = nil, w
	w.neighborCursor = neighborQueueCapacity - 1
	w.enqueueNeighbor(neighborUpdate{x: 7, y: 64, z: 8, sourceX: 1})
	w.enqueueNeighbor(neighborUpdate{x: 7, y: 64, z: 8, sourceX: 2})
	w.processNeighborUpdates()
	if len(b.seen) != 3 || b.seen[0].sourceX != 1 || b.seen[1].sourceX != 2 || b.seen[2].sourceX != 99 || len(w.neighborUpdates) != neighborQueueCapacity {
		t.Fatalf("wrapped notifications = %+v, capacity=%d", b.seen, len(w.neighborUpdates))
	}
}

// TestNeighborOverflowSurvivesRestart persists deferred support checks through chunk NBT.
func TestNeighborOverflowSurvivesRestart(t *testing.T) {
	w, storage := neighborTestWorld(t)
	c, _ := storage.Get(0, 0)
	c.SetBlock(8, 64, 8, block.DefaultStateID("minecraft:wheat"))
	w.updateActivity()
	for range neighborQueueCapacity + 1 {
		w.enqueueNeighbor(neighborUpdate{x: 8, y: 64, z: 8})
	}
	if w.neighborCount != neighborQueueCapacity || w.tickScheduler.Pending() != 1 {
		t.Fatalf("queued=%d deferred=%d", w.neighborCount, w.tickScheduler.Pending())
	}
	frozen, err := chunk.SnapshotForSave(c)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := chunk.EncodeChunkNBT(frozen)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := chunk.DecodeChunkNBT(encoded, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	restarted, newStorage := neighborTestWorld(t)
	newStorage.Set(reloaded)
	restarted.Tick()
	if reloaded.GetBlock(8, 64, 8) == 0 || restarted.tickScheduler.Pending() != 1 {
		t.Fatal("inactive deferred check was lost or executed")
	}
	activateTestChunk(t, restarted, newStorage, 0, 0)
	restarted.Tick()
	if reloaded.GetBlock(8, 64, 8) != 0 || restarted.ItemEntities().Count() != 1 {
		t.Fatal("persisted overflow did not remove the unsupported crop exactly once")
	}
}

// TestFarmlandCheckRevalidatesBeforeConversion covers delayed updates and removed obstructions.
func TestFarmlandCheckRevalidatesBeforeConversion(t *testing.T) {
	w, storage := neighborTestWorld(t)
	activateTestChunk(t, w, storage, 0, 0)
	farmland := block.DefaultStateID("minecraft:farmland")
	w.setBlockAt(8, 63, 8, farmland)
	w.setBlockAt(8, 64, 8, block.DefaultStateID("minecraft:stone"))
	w.Tick()
	if w.getBlockAt(8, 63, 8) != farmland || w.tickScheduler.Pending() != 1 {
		t.Fatal("farmland converted before its delayed tick")
	}
	w.setBlockAt(8, 64, 8, 0)
	w.Tick()
	if w.getBlockAt(8, 63, 8) != farmland {
		t.Fatal("removed obstruction still converted farmland")
	}
	w.setBlockAt(8, 64, 8, block.DefaultStateID("minecraft:stone"))
	w.Tick()
	w.Tick()
	if w.getBlockAt(8, 63, 8) != block.DefaultStateID("minecraft:dirt") || w.ItemEntities().Count() != 0 {
		t.Fatal("scheduled conversion failed or produced loot")
	}
}

// placeTestPortal builds a six-cell portal across the negative chunk seam.
func placeTestPortal(t *testing.T, w *World, storage *chunk.Storage) int32 {
	t.Helper()
	if _, present := storage.Get(-1, 0); !present {
		storage.Set(chunk.New(-1, 0))
	}
	obsidian := block.DefaultStateID("minecraft:obsidian")
	portalState := block.StateID("minecraft:nether_portal", map[string]string{"axis": "x"})
	for x := -1; x <= 0; x++ {
		w.setBlockAt(x, 63, 8, obsidian)
		w.setBlockAt(x, 67, 8, obsidian)
		for y := 64; y < 67; y++ {
			w.setBlockAt(x, y, 8, portalState)
		}
	}
	for y := 64; y < 67; y++ {
		w.setBlockAt(-2, y, 8, obsidian)
		w.setBlockAt(1, y, 8, obsidian)
	}
	return portalState
}

// TestPortalNeighborCollapseAcrossChunkSeam checks batched construction and missing corners.
func TestPortalNeighborCollapseAcrossChunkSeam(t *testing.T) {
	w, storage := neighborTestWorld(t)
	state := placeTestPortal(t, w, storage)
	w.Tick()
	for x := -1; x <= 0; x++ {
		for y := 64; y < 67; y++ {
			if w.getBlockAt(x, y, 8) != state {
				t.Fatal("batched portal construction collapsed before completion")
			}
		}
	}
	w.setBlockAt(-2, 65, 8, 0)
	if w.getBlockAt(-1, 65, 8) != state {
		t.Fatal("portal collapsed recursively during frame removal")
	}
	w.Tick()
	for x := -1; x <= 0; x++ {
		for y := 64; y < 67; y++ {
			if w.getBlockAt(x, y, 8) != 0 {
				t.Fatalf("invalid portal survived at %d,%d,8", x, y)
			}
		}
	}
	if w.ItemEntities().Count() != 0 {
		t.Fatal("portal collapse produced loot")
	}
}

// TestPortalRetainsUnknownFrameAndReconcilesLoadedBorder bounds deferred seam checks.
func TestPortalRetainsUnknownFrameAndReconcilesLoadedBorder(t *testing.T) {
	w, storage := neighborTestWorld(t)
	state := placeTestPortal(t, w, storage)
	w.Tick()
	right, _ := storage.Get(0, 0)
	storage.Delete(0, 0)
	w.setBlockAt(-2, 65, 8, 0)
	w.Tick()
	if w.getBlockAt(-1, 65, 8) != state {
		t.Fatal("unknown frame was treated as a broken frame")
	}
	storage.Set(right)
	w.queueChunkNeighborChecks(right)
	w.queueChunkNeighborChecks(right)
	if len(w.neighborBorders) != 1 {
		t.Fatal("duplicate publication duplicated border scan")
	}
	w.processChunkNeighborChecks()
	if cursor := w.neighborBorders[right.Key()]; cursor != neighborBorderWorkPerTick {
		t.Fatalf("border work=%d, want %d", cursor, neighborBorderWorkPerTick)
	}
	for len(w.neighborBorders) > 0 {
		w.processChunkNeighborChecks()
	}
	if w.getBlockAt(-1, 65, 8) != state || w.tickScheduler.Pending() == 0 {
		t.Fatal("border reconciliation mutated immediately or discarded checks")
	}
	activateTestChunk(t, w, storage, -1, 0)
	activateTestChunk(t, w, storage, 0, 0)
	w.Tick()
	if w.getBlockAt(-1, 65, 8) != 0 || w.getBlockAt(0, 65, 8) != 0 {
		t.Fatal("reloaded frame did not collapse across the seam")
	}
}

// TestNeighborWorkBudgetRetainsDeferredChecks bounds chained work without losing support changes.
func TestNeighborWorkBudgetRetainsDeferredChecks(t *testing.T) {
	w, storage := neighborTestWorld(t)
	c, _ := storage.Get(0, 0)
	crop := block.DefaultStateID("minecraft:wheat")
	c.SetBlock(8, 64, 8, crop)
	w.NotifyNeighbors(8, 63, 8)
	w.neighborWork = neighborWorkPerTick
	w.processNeighborUpdates()
	w.Tick()
	if c.GetBlock(8, 64, 8) != crop || w.tickScheduler.Pending() != 1 || w.neighborCount != 0 {
		t.Fatal("work limit dropped or executed a deferred check")
	}
	activateTestChunk(t, w, storage, 0, 0)
	w.Tick()
	if c.GetBlock(8, 64, 8) != 0 || w.ItemEntities().Count() != 1 {
		t.Fatal("deferred support check did not resume exactly once")
	}
}

// TestNeighborBorderBudgetRotatesBetweenChunks prevents ongoing loads from starving checks.
func TestNeighborBorderBudgetRotatesBetweenChunks(t *testing.T) {
	w, storage := neighborTestWorld(t)
	left := chunk.New(-1, 0)
	storage.Set(left)
	right, _ := storage.Get(0, 0)
	w.queueChunkNeighborChecks(left)
	w.queueChunkNeighborChecks(right)
	w.processChunkNeighborChecks()
	w.processChunkNeighborChecks()
	if w.neighborBorders[left.Key()] != neighborBorderWorkPerTick || w.neighborBorders[right.Key()] != neighborBorderWorkPerTick {
		t.Fatalf("unfair border cursors: %v", w.neighborBorders)
	}
	storage.Delete(-1, 0)
	w.processChunkNeighborChecks()
	if _, pending := w.neighborBorders[left.Key()]; pending {
		t.Fatal("evicted chunk retained a border scan")
	}
}

// TestNeighborChecksRunBeforeShutdownSnapshot persists supported mutations and drops at shutdown.
func TestNeighborChecksRunBeforeShutdownSnapshot(t *testing.T) {
	path := t.TempDir()
	c := chunk.New(0, 0)
	farmland := block.DefaultStateID("minecraft:farmland")
	crop := block.DefaultStateID("minecraft:wheat")
	c.SetBlock(8, 63, 8, farmland)
	c.SetBlock(8, 64, 8, crop)
	w, _ := persistentTestWorld(t, path, c)
	w.updateActivity()
	w.setBlockAt(8, 63, 8, 0)
	if err := w.Close(nil); err != nil {
		t.Fatal(err)
	}
	restarted, _ := persistentTestWorld(t, path, nil)
	t.Cleanup(func() {
		if err := restarted.Close(nil); err != nil {
			t.Error(err)
		}
	})
	reloaded := awaitEntityLoad(t, restarted, 0, 0)
	if reloaded.GetBlock(8, 64, 8) != 0 || restarted.ItemEntities().Count() != 1 {
		t.Fatal("shutdown lost a queued support check or its item drop")
	}
}

// replacingCropBehavior models a callback that replaces its target before the shape check.
type replacingCropBehavior struct {
	behavior.CropBehavior
	w *World
}

// OnNeighborUpdate replaces the crop with a block that does not require farmland.
func (b *replacingCropBehavior) OnNeighborUpdate(ctx *behavior.Context) {
	b.w.setBlockAt(ctx.X, ctx.Y, ctx.Z, block.DefaultStateID("minecraft:stone"))
}

// TestNeighborShapeReadsPostCallbackState prevents stale crop checks from deleting replacements.
func TestNeighborShapeReadsPostCallbackState(t *testing.T) {
	w, storage := neighborTestWorld(t)
	c, _ := storage.Get(0, 0)
	crop := block.DefaultStateID("minecraft:wheat")
	id := block.BlockIDFromState(crop)
	old := behavior.Get(id)
	behavior.Register(id, &replacingCropBehavior{w: w})
	t.Cleanup(func() { behavior.Register(id, old) })
	c.SetBlock(8, 64, 8, crop)
	w.NotifyNeighbors(8, 63, 8)
	w.Tick()
	if c.GetBlock(8, 64, 8) != block.DefaultStateID("minecraft:stone") || w.ItemEntities().Count() != 0 {
		t.Fatal("stale shape check removed a callback's replacement or produced loot")
	}
}
