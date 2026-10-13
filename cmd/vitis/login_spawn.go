package main

import (
	"context"
	"fmt"
	"math"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/logger"
	"github.com/vitismc/vitis/internal/world"
	"github.com/vitismc/vitis/internal/world/chunk"
)

// retainLoginSpawn pins collision dependencies until admission publishes player tickets.
func retainLoginSpawn(ctx context.Context, w *world.World, sessionID uint64, position entity.Vec3) error {
	if ctx == nil || w == nil || sessionID == 0 || sessionID>>63 != 0 {
		return fmt.Errorf("login spawn: invalid owner")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	x, z := math.Floor(position.X/16), math.Floor(position.Z/16)
	if math.IsNaN(x) || math.IsNaN(z) || x < -1875000 || x >= 1875000 || z < -1875000 || z >= 1875000 {
		return fmt.Errorf("login spawn: invalid coordinates")
	}
	result := make(chan error, 1)
	if !w.Submit(func() {
		if err := ctx.Err(); err != nil {
			result <- err
			return
		}
		result <- w.SetChunkTicket(chunk.Ticket{
			Kind: chunk.TicketView, Owner: sessionID | (uint64(1) << 63),
			X: int32(x), Z: int32(z), Radius: 1, ExpiresAt: w.CurrentTick() + 1200,
		})
	}) {
		return fmt.Errorf("login spawn: world command queue is full")
	}
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// releaseLoginSpawn follows queued admission; expiry bounds retention if ingress is full.
func releaseLoginSpawn(w *world.World, sessionID uint64) {
	if w == nil {
		return
	}
	if !w.Submit(func() { w.RemoveChunkTicket(chunk.TicketView, sessionID|(uint64(1)<<63)) }) {
		logger.Warn("login spawn ticket release queue is full", "world", w.Name(), "session", sessionID)
	}
}
