package world

import (
	"errors"
	"github.com/vitismc/vitis/internal/logger"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/level"
	"github.com/vitismc/vitis/internal/world/persistence"
)

const (
	defaultAutosaveInterval = 1200
	defaultAutosaveBatch    = 2
	defaultAutosaveQueue    = 64
)

type chunkSaveSnapshot struct {
	metadata   *level.Level
	worldDir   string
	chunk      *chunk.Chunk
	revision   uint64
	accessTick uint64
	frozen     *chunk.Chunk
	entities   []byte
}

type chunkSaveResult struct {
	snapshot chunkSaveSnapshot
	err      error
}

// chunkAutosave owns the bounded disk writer; only the world tick touches chunks.
type chunkAutosave struct {
	interval        uint64
	batch           int
	queue           chan chunkSaveSnapshot
	results         chan chunkSaveResult
	keys            []int64
	cursor          int
	closed          bool
	metadataPending bool
	metadataDirty   bool
}

// newChunkAutosave starts one disk writer for immutable chunk NBT snapshots.
func newChunkAutosave(store *persistence.ChunkStore, interval, batch, capacity int) *chunkAutosave {
	if interval <= 0 {
		interval = defaultAutosaveInterval
	}
	if batch <= 0 {
		batch = defaultAutosaveBatch
	}
	if capacity <= 0 {
		capacity = defaultAutosaveQueue
	}
	a := &chunkAutosave{
		interval: uint64(interval),
		batch:    batch,
		queue:    make(chan chunkSaveSnapshot, capacity),
		results:  make(chan chunkSaveResult, capacity+1),
	}
	go a.writeSnapshots(store)

	return a
}

// writeSnapshots drains available work in bounded batches without waiting to fill a batch.
func (a *chunkAutosave) writeSnapshots(store *persistence.ChunkStore) {
	defer close(a.results)
	var snapshots [16]chunkSaveSnapshot
	var results [16]chunkSaveResult
	var writes [16]persistence.ChunkWrite
	var writeIndices [16]int
	for first := range a.queue {
		snapshots[0] = first
		count := 1
	collect:
		for count < len(snapshots) {
			select {
			case snapshot, ok := <-a.queue:
				if !ok {
					break collect
				}
				snapshots[count] = snapshot
				count++
			default:
				break collect
			}
		}
		writeCount := 0
		for index, snapshot := range snapshots[:count] {
			if snapshot.metadata != nil {
				results[index] = chunkSaveResult{snapshot: snapshot, err: snapshot.metadata.Save(snapshot.worldDir)}
				continue
			}
			data, err := chunk.EncodeChunkNBT(snapshot.frozen)
			snapshot.frozen = nil
			results[index] = chunkSaveResult{snapshot: snapshot, err: err}
			if err == nil {
				writes[writeCount] = persistence.ChunkWrite{X: int(snapshot.chunk.X()), Z: int(snapshot.chunk.Z()), NBT: data}
				writeIndices[writeCount] = index
				writeCount++
			}
		}
		for index, err := range store.WriteChunkBatch(writes[:writeCount]) {
			results[writeIndices[index]].err = err
		}
		entityCount := 0
		for index, result := range results[:count] {
			if result.snapshot.chunk != nil && result.err == nil && result.snapshot.entities != nil {
				writes[entityCount] = persistence.ChunkWrite{X: int(result.snapshot.chunk.X()), Z: int(result.snapshot.chunk.Z()), NBT: result.snapshot.entities}
				writeIndices[entityCount] = index
				entityCount++
			}
		}
		for index, err := range store.WriteEntityBatch(writes[:entityCount]) {
			results[writeIndices[index]].err = errors.Join(results[writeIndices[index]].err, err)
		}
		for _, result := range results[:count] {
			a.results <- result
		}
		clear(snapshots[:count])
		clear(results[:count])
		clear(writes[:max(writeCount, entityCount)])
	}
}

// tick acknowledges completed writes and queues bounded snapshots from world state.
func (a *chunkAutosave) tick(w *World) {
	if a == nil || a.closed || w == nil || w.chunks == nil {
		return
	}
	for i := 0; i < cap(a.results); i++ {
		select {
		case result := <-a.results:
			a.acknowledge(result)
			if w.pregen != nil && result.snapshot.chunk != nil {
				w.pregen.onSaved(result)
			}
		default:
			i = cap(a.results)
		}
	}
	if w.worldDir != "" && w.CurrentTick()%a.interval == 0 {
		a.metadataDirty = true
	}
	if a.metadataDirty && !a.metadataPending && len(a.queue) < cap(a.queue) {
		snapshot := w.MetadataSnapshot()
		if snapshot != nil {
			a.queue <- chunkSaveSnapshot{metadata: snapshot, worldDir: w.worldDir}
			a.metadataPending = true
			a.metadataDirty = false
		}
	}
	if len(a.keys) == 0 && w.CurrentTick()%a.interval == 0 {
		w.markScheduledTickSnapshots()
		a.keys = w.chunks.DirtyKeys(a.keys)
		a.cursor = 0
	}
	processed := 0
	for a.cursor < len(a.keys) && processed < a.batch {
		if len(a.queue) == cap(a.queue) {
			return
		}
		key := a.keys[a.cursor]
		c := w.chunks.DirtyByKey(key)
		if c == nil {
			a.cursor++
			continue
		}
		queued, err := a.enqueue(c)
		if err != nil {
			logger.Error("autosave chunk encoding failed", "x", c.X(), "z", c.Z(), "error", err)
			a.cursor++
			continue
		}
		a.cursor++
		if queued {
			processed++
		}
	}
	if a.cursor == len(a.keys) {
		a.keys = nil
		a.cursor = 0
	}
}

// markScheduledTickSnapshots refreshes remaining delays even when blocks have not changed.
func (w *World) markScheduledTickSnapshots() {
	w.chunks.VisitLoaded(func(c *chunk.Chunk) {
		if c.HasScheduledTicks() && !c.Dirty() && !c.SavePending() {
			c.MarkDirty()
		}
	})
}

// enqueue queues one immutable snapshot without blocking the world tick.
func (a *chunkAutosave) enqueue(c *chunk.Chunk) (bool, error) {
	if a == nil || a.closed || c == nil || c.SavePending() || len(a.queue) == cap(a.queue) {
		return false, nil
	}
	if !c.BeginSave() {
		return false, nil
	}
	frozen, err := chunk.SnapshotForSave(c)
	if err != nil {
		c.FinishSave()
		return false, err
	}
	entities, _ := frozen.EntityNBT()
	a.queue <- chunkSaveSnapshot{chunk: c, revision: c.Revision(), accessTick: c.LastAccessTick(), frozen: frozen, entities: entities}
	return true, nil
}

// acknowledge applies a disk result without accepting stale revisions.
func (a *chunkAutosave) acknowledge(result chunkSaveResult) {
	if result.snapshot.metadata != nil {
		a.metadataPending = false
		if result.err != nil {
			a.metadataDirty = true
			logger.Error("autosave world metadata failed", "error", result.err)
		}
		return
	}
	c := result.snapshot.chunk
	c.FinishSave()
	if result.err != nil {
		logger.Error("autosave chunk write failed", "x", c.X(), "z", c.Z(), "error", result.err)
		return
	}
	c.ClearDirtyIfRevision(result.snapshot.revision)
}

// close drains every queued write before final synchronous world flushing.
func (a *chunkAutosave) close() {
	if a == nil || a.closed {
		return
	}
	a.closed = true
	close(a.queue)
	for result := range a.results {
		a.acknowledge(result)
	}
}
