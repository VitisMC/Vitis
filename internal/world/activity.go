package world

import (
	"fmt"
	"math"
	"strconv"

	"github.com/vitismc/vitis/internal/logger"
	"github.com/vitismc/vitis/internal/world/chunk"
)

// initActivity installs ticket-based simulation and scheduled-tick persistence.
func (w *World) initActivity(distance int32) {
	if distance <= 0 {
		distance = 6
	}
	w.simulationDistance = distance
	w.activity = chunk.NewActivityTracker()
	w.playerTicketOwners = make(map[uint64]struct{})
	w.chunks.SetRetentionPredicate(func(x, z int32) bool { return w.activity.At(x, z) != chunk.ActivityUnloaded })
	w.tickScheduler.SetChangeHandler(func(x, z int32) {
		if c, ok := w.chunks.PeekLoaded(x, z); ok {
			c.MarkDirty()
		}
	})
	w.chunks.SetPublishValidator(func(c *chunk.Chunk) error {
		if w.chunkStore != nil && !c.IsPregenerationOnly() {
			if _, err := w.prepareEntityChunk(c); err != nil {
				return err
			}
		}
		return c.BindScheduledTicks(w.tickScheduler, w.CurrentTick)
	})
	w.chunks.SetPublishHandler(func(c *chunk.Chunk) {
		w.bindChunkLight(c)
		w.logEntitySnapshotError(w.bindChunkEntities(c))
		w.queueChunkNeighborChecks(c)
	})
	w.chunks.SetEvictionHandler(func(c *chunk.Chunk) {
		delete(w.neighborBorders, c.Key())
		w.evictChunkEntities(c)
		w.tickScheduler.RemoveChunk(c.X(), c.Z())
		delete(w.lightChanges, c.Key())
		for _, offset := range [4][2]int32{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			if neighbor, ok := w.chunks.PeekLoaded(c.X()+offset[0], c.Z()+offset[1]); ok && !neighbor.IsPregenerationOnly() {
				if w.lightChanges == nil {
					w.lightChanges = make(map[int64]struct{})
				}
				w.lightChanges[neighbor.Key()] = struct{}{}
			}
		}
	})
}

// SetChunkTicket acquires a bounded runtime residency requirement on the world tick.
func (w *World) SetChunkTicket(ticket chunk.Ticket) error {
	if w == nil {
		return fmt.Errorf("set chunk ticket: unavailable world")
	}
	return w.activity.Set(ticket)
}

// setRuntimeTicket drops stale ownership when internal residency input is invalid.
func (w *World) setRuntimeTicket(ticket chunk.Ticket) {
	if err := w.activity.Set(ticket); err != nil {
		w.activity.Remove(ticket.Kind, ticket.Owner)
		logger.Error("invalid runtime chunk ticket", "world", w.name, "owner", ticket.Owner, "error", err)
	}
}

// RemoveChunkTicket releases one residency reason on the world tick.
func (w *World) RemoveChunkTicket(kind chunk.TicketKind, owner uint64) {
	if w != nil {
		w.activity.Remove(kind, owner)
	}
}

// ChunkActivity returns residency activity only after loading has completed.
func (w *World) ChunkActivity(x, z int32) chunk.Activity {
	if w == nil {
		return chunk.ActivityUnloaded
	}
	if _, ok := w.chunks.PeekLoaded(x, z); !ok {
		return chunk.ActivityUnloaded
	}
	activity := w.activity.At(x, z)
	if activity >= chunk.ActivityBlockTicking && !w.loadedNeighborhood(x, z, 1) {
		return chunk.ActivityLoaded
	}
	if activity >= chunk.ActivityEntityTicking && !w.loadedNeighborhood(x, z, 2) {
		return chunk.ActivityBlockTicking
	}
	return activity
}

// loadedNeighborhood verifies FULL dependencies without touching their access timestamps.
func (w *World) loadedNeighborhood(x, z, radius int32) bool {
	for dx := -radius; dx <= radius; dx++ {
		for dz := -radius; dz <= radius; dz++ {
			if _, ok := w.chunks.PeekLoaded(x+dx, z+dz); !ok {
				return false
			}
		}
	}
	return true
}

// IsEntityTicking reports whether entity simulation may advance in a loaded chunk.
func (w *World) IsEntityTicking(x, z int32) bool {
	return w.ChunkActivity(x, z) >= chunk.ActivityEntityTicking
}

// IsBlockTicking reports whether scheduled block and fluid ticks may run.
func (w *World) IsBlockTicking(x, z int32) bool {
	return w.ChunkActivity(x, z) >= chunk.ActivityBlockTicking
}

// updateActivity refreshes player tickets without activating pregenerated chunks.
func (w *World) updateActivity() {
	seen := make(map[uint64]struct{}, len(w.playerTicketOwners))
	for _, p := range w.tracker.Players() {
		if p.Removed() {
			continue
		}
		position := p.Position()
		x, z := int32(math.Floor(position.X/16)), int32(math.Floor(position.Z/16))
		owner := uint64(uint32(p.ID()))
		seen[owner] = struct{}{}
		w.setRuntimeTicket(chunk.Ticket{Kind: chunk.TicketPlayer, Owner: owner, X: x, Z: z, Radius: w.simulationDistance})
		w.setRuntimeTicket(chunk.Ticket{Kind: chunk.TicketView, Owner: owner, X: x, Z: z, Radius: w.clientViewDistanceFor(p)})
	}
	for owner := range w.playerTicketOwners {
		if _, ok := seen[owner]; !ok {
			w.activity.Remove(chunk.TicketPlayer, owner)
			w.activity.Remove(chunk.TicketView, owner)
		}
	}
	w.playerTicketOwners = seen
	if w.dimension == "minecraft:overworld" {
		x, _, z, _ := w.WorldSpawn()
		radius, _ := strconv.Atoi(w.GameRule("spawnChunkRadius"))
		if radius > 0 {
			w.setRuntimeTicket(chunk.Ticket{Kind: chunk.TicketSpawn, X: x >> 4, Z: z >> 4, Radius: int32(radius - 1)})
		} else {
			w.activity.Remove(chunk.TicketSpawn, 0)
		}
	}
	w.activity.Update(w.CurrentTick())
	w.activity.VisitRequired(w.chunkCompletionsPerTick, func(x, z int32) {
		if !w.chunks.IsTracked(x, z) {
			w.chunks.RequestLoad(x, z)
		} else if c, ok := w.chunks.PeekLoaded(x, z); ok && c.IsPregenerationOnly() {
			w.chunks.RequestLoad(x, z)
		}
	})
	w.chunks.VisitLoaded(func(c *chunk.Chunk) {
		w.bindChunkLight(c)
		if err := c.BindScheduledTicks(w.tickScheduler, w.CurrentTick); err != nil {
			logger.Error("bind chunk ticks failed", "x", c.X(), "z", c.Z(), "error", err)
		}
	})
}
