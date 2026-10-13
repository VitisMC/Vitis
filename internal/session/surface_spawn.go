package session

import (
	"github.com/vitismc/vitis/internal/block"
	gencollision "github.com/vitismc/vitis/internal/data/generated/collision"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/world/chunk/section"
)

// safeWorldRespawn prefers nearby dry surfaces, falling back to the water surface.
// World spawn searches never descend through the surface into enclosed caves.
func safeWorldRespawn(world WorldAccessor, x, _ int, z int) (float64, float64, float64, bool) {
	if world == nil {
		return 0, 0, 0, false
	}
	var water entity.Vec3
	hasWater := false
	for radius := 0; radius <= 4; radius++ {
		for dx := -radius; dx <= radius; dx++ {
			for dz := -radius; dz <= radius; dz++ {
				if radius > 0 && absInt(dx) != radius && absInt(dz) != radius {
					continue
				}
				position, wet, valid := surfaceSpawnAt(world, x+dx, z+dz)
				if !valid {
					continue
				}
				if !wet {
					return position.X, position.Y, position.Z, true
				}
				if !hasWater {
					water, hasWater = position, true
				}
			}
		}
	}
	return water.X, water.Y, water.Z, hasWater
}

// surfaceSpawnAt resolves the first supporting or fluid surface in a loaded column.
func surfaceSpawnAt(world WorldAccessor, x, z int) (entity.Vec3, bool, bool) {
	owner, loaded := world.GetChunk(int32(x>>4), int32(z>>4))
	if !loaded || owner == nil || owner.Data() == nil {
		return entity.Vec3{}, false, false
	}
	data := owner.Data()
	top, exists := data.Heightmap(x, z, section.HeightmapWorldSurface)
	if !exists {
		return entity.Vec3{}, false, false
	}
	for y := top; y > data.MinY(); y-- {
		state := world.GetBlock(x, y-1, z)
		if spawnContainsWater(state) {
			position := entity.Vec3{X: float64(x) + 0.5, Y: float64(y), Z: float64(z) + 0.5}
			box := physics.PlayerDimensions().MakeBoundingBox(position.X, position.Y, position.Z)
			valid := position.Y+physics.PlayerDimensions().Height <= float64(data.MaxY()+1) &&
				spawnPositionClear(world, position.X, position.Y, position.Z) && !spawnBoxTouchesHazard(world, box)
			return position, true, valid
		}
		if block.IsLava(state) {
			return entity.Vec3{}, false, false
		}
		if !gencollision.IsEmpty(state) {
			px, py, pz, valid := standingSpawnAt(world, x, y, z)
			return entity.Vec3{X: px, Y: py, Z: pz}, false, valid
		}
	}
	return entity.Vec3{}, false, false
}

// spawnContainsWater includes aquatic plants and waterlogged blocks in surface checks.
func spawnContainsWater(state int32) bool {
	if block.IsAir(state) {
		return false
	}
	if block.IsWater(state) {
		return true
	}
	switch block.NameFromState(state) {
	case "minecraft:kelp", "minecraft:kelp_plant", "minecraft:seagrass", "minecraft:tall_seagrass", "minecraft:bubble_column":
		return true
	}
	return block.PropertiesFromState(state)["waterlogged"] == "true"
}
