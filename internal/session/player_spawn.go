package session

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/vitismc/vitis/internal/entity"
)

// PreparePlayerSpawn waits for asynchronous chunk preparation and resolves the
// login position on the world tick, retaining valid saved positions.
func PreparePlayerSpawn(ctx context.Context, world WorldAccessor, position entity.Vec3, saved bool) (entity.Vec3, error) {
	if world == nil {
		return position, nil
	}
	if !validPlayerPosition(position.X, position.Y, position.Z) {
		return entity.Vec3{}, fmt.Errorf("invalid saved player position")
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		result := make(chan spawnPreparation, 1)
		accepted := world.Submit(&spawnPlayerCommand{ctx: ctx, world: world, position: position, saved: saved, result: result})
		if accepted {
			select {
			case resolved := <-result:
				if resolved.err == nil {
					return resolved.position, nil
				}
				if resolved.err != errRespawnChunksPending {
					return entity.Vec3{}, resolved.err
				}
			case <-ctx.Done():
				return entity.Vec3{}, ctx.Err()
			}
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return entity.Vec3{}, ctx.Err()
		}
	}
}

// spawnPreparation carries an independently resolved position to the login task.
type spawnPreparation struct {
	position entity.Vec3
	err      error
}

// spawnPlayerCommand validates login collisions only after its chunks are loaded.
type spawnPlayerCommand struct {
	ctx      context.Context
	world    WorldAccessor
	position entity.Vec3
	saved    bool
	result   chan<- spawnPreparation
}

// Execute resolves the login transition without performing blocking chunk IO.
func (c *spawnPlayerCommand) Execute() {
	if err := c.ctx.Err(); err != nil {
		c.result <- spawnPreparation{err: err}
		return
	}
	if err := prepareRespawnChunks(c.world, c.position.X, c.position.Z); err != nil {
		c.result <- spawnPreparation{err: err}
		return
	}
	p := c.position
	if c.saved && spawnPositionClear(c.world, p.X, p.Y, p.Z) {
		c.result <- spawnPreparation{position: p}
		return
	}
	resolve := safeWorldRespawn
	if c.saved {
		resolve = safeLocalRespawn
	}
	x, y, z, ok := resolve(c.world, int(math.Floor(p.X)), int(math.Floor(p.Y)), int(math.Floor(p.Z)))
	if !ok {
		c.result <- spawnPreparation{err: errRespawnObstructed}
		return
	}
	c.result <- spawnPreparation{position: entity.Vec3{X: x, Y: y, Z: z}}
}

// spawnBlockAccess adapts the existing world reader to collision detection.
type spawnBlockAccess struct{ WorldAccessor }

// GetBlockStateAt returns one loaded block for collision queries.
func (a spawnBlockAccess) GetBlockStateAt(x, y, z int) int32 { return a.GetBlock(x, y, z) }
