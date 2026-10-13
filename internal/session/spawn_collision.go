package session

import (
	"math"

	"github.com/vitismc/vitis/internal/block"
	gencollision "github.com/vitismc/vitis/internal/data/generated/collision"
	"github.com/vitismc/vitis/internal/entity/physics"
)

// spawnCollisionChunksReady requires every chunk read by a collision query.
func spawnCollisionChunksReady(world WorldAccessor, box physics.AABB) bool {
	if world == nil {
		return false
	}
	minX, _, minZ, maxX, _, maxZ := physics.BlockCollisionBounds(box)
	for cx := minX >> 4; cx <= maxX>>4; cx++ {
		for cz := minZ >> 4; cz <= maxZ>>4; cz++ {
			if owner, loaded := world.GetChunk(int32(cx), int32(cz)); !loaded || owner == nil {
				return false
			}
		}
	}
	return true
}

// spawnPositionClear checks the complete standing player volume in loaded chunks.
func spawnPositionClear(world WorldAccessor, x, y, z float64) bool {
	if !validPlayerPosition(x, y, z) {
		return false
	}
	box := physics.PlayerDimensions().MakeBoundingBox(x, y, z)
	return spawnCollisionChunksReady(world, box) && !physics.IsCollidingAt(spawnBlockAccess{world}, box)
}

// standingSpawnAt resolves a shape's top surface and checks standing clearance.
func standingSpawnAt(world WorldAccessor, x, y, z int) (float64, float64, float64, bool) {
	if world == nil {
		return 0, 0, 0, false
	}
	px, pz := float64(x)+0.5, float64(z)+0.5
	box := physics.PlayerDimensions().MakeBoundingBox(px, float64(y), pz)
	floor := math.Inf(-1)
	for by := y - 1; by <= y; by++ {
		for _, shape := range gencollision.ShapesForStateID(world.GetBlock(x, by, z)) {
			top := float64(by) + shape.MaxY
			if top > float64(y)+1 || top <= floor ||
				float64(x)+shape.MaxX <= box.MinX || float64(x)+shape.MinX >= box.MaxX ||
				float64(z)+shape.MaxZ <= box.MinZ || float64(z)+shape.MinZ >= box.MaxZ {
				continue
			}
			floor = top
		}
	}
	if math.IsInf(floor, -1) || floor < -64 || floor+physics.PlayerDimensions().Height > 320 ||
		!spawnPositionClear(world, px, floor, pz) {
		return 0, 0, 0, false
	}
	box = physics.PlayerDimensions().MakeBoundingBox(px, floor, pz)
	if !physics.CheckOnGround(spawnBlockAccess{world}, box) || spawnTouchesHazard(world, box) {
		return 0, 0, 0, false
	}
	return px, floor, pz, true
}

// spawnTouchesHazard excludes damaging blocks and fluids at the body or feet.
func spawnTouchesHazard(world WorldAccessor, box physics.AABB) bool {
	box.MinY -= 0.001
	return spawnBoxTouchesHazard(world, box)
}

// spawnBoxTouchesHazard checks hazards strictly inside the supplied volume.
func spawnBoxTouchesHazard(world WorldAccessor, box physics.AABB) bool {
	for x := int(math.Floor(box.MinX)); x < int(math.Ceil(box.MaxX)); x++ {
		for y := int(math.Floor(box.MinY)); y < int(math.Ceil(box.MaxY)); y++ {
			for z := int(math.Floor(box.MinZ)); z < int(math.Ceil(box.MaxZ)); z++ {
				state := world.GetBlock(x, y, z)
				if spawnContainsWater(state) || block.IsLava(state) || block.IsFire(state) ||
					block.IsCactus(state) || block.IsSweetBerryBush(state) {
					return true
				}
				switch block.NameFromState(state) {
				case "minecraft:magma_block", "minecraft:powder_snow", "minecraft:wither_rose":
					return true
				case "minecraft:campfire", "minecraft:soul_campfire":
					if block.PropertiesFromState(state)["lit"] == "true" {
						return true
					}
				}
			}
		}
	}
	return false
}
