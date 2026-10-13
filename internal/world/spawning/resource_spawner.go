package spawning

import (
	"math"
	"math/rand"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/block/fluid"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/physics"
)

// ChunkPos identifies a resident chunk eligible for natural spawning.
type ChunkPos struct{ X, Z int32 }

// Settings exposes authoritative dimension and gamerule state.
type Settings struct {
	Dimension     string
	Enabled       bool
	Peaceful      bool
	Spawn         entity.Vec3
	SeaLevel      int
	SeaLevelKnown bool
	Seed          int64
	DayTime       int64
	WorldAge      int64
	Difficulty    int32
}

// Environment supplies resident terrain, biome and lighting queries without loading chunks.
type Environment interface {
	physics.BlockAccess
	SpawnSettings() Settings
	SpawnChunks(players []PlayerPos) []ChunkPos
	SpawnBiome(x, y, z int) string
	SpawnLight(x, y, z int) (block, sky, brightness uint8, available bool)
}

// tickResources evaluates one spawn cycle against a consistent compiled snapshot.
func (s *Spawner) tickResources(mobs *entity.MobManager, players []PlayerPos, blocks BlockAccess) []SpawnRequest {
	environment, ok := blocks.(Environment)
	definitions := s.runtime.current()
	if !ok || definitions == nil || mobs == nil {
		return nil
	}
	settings := environment.SpawnSettings()
	s.ticks++
	s.despawn(mobs, players, blocks, definitions, settings.Peaceful)
	if !settings.Enabled || len(players) == 0 || s.ticks%uint64(s.interval) != 0 {
		return nil
	}
	rules, ok := definitions.dimensions[settings.Dimension]
	if !ok {
		return nil
	}
	chunks := environment.SpawnChunks(players)
	if len(chunks) == 0 {
		return nil
	}
	global := make(map[Category]int)
	var potential spawnPotential
	local := make([]map[Category]int, len(players))
	for i := range local {
		local[i] = make(map[Category]int)
	}
	for _, mob := range mobs.Mobs() {
		if mob.Removed() || mob.IsDead() || mob.PersistenceRequired() {
			continue
		}
		category := definitions.categories[mob.TypeName()]
		global[category]++
		addLocal(local, players, mob.Position(), category)
		potential.add(mob.Position(), definitions.costAt(environment, mob.TypeName(), mob.Position()).charge)
	}
	var requests []SpawnRequest
	for _, limit := range categoryLimits {
		category := limit.category
		cap := limit.cap * len(chunks) / 289
		if cap == 0 || global[category] >= cap || settings.Peaceful && category == Monster || category == Creature && s.ticks%400 != 0 {
			continue
		}
		for range 3 {
			if global[category] >= cap {
				break
			}
			candidate := chunks[s.rng.Intn(len(chunks))]
			x, z := int(candidate.X)*16+s.rng.Intn(16), int(candidate.Z)*16+s.rng.Intn(16)
			ceiling := rules.MinY + rules.Height - 1
			for ceiling > rules.MinY && blocks.GetBlock(x, ceiling, z) == 0 {
				ceiling--
			}
			y := rules.MinY + s.rng.Intn(max(1, min(rules.Height, ceiling-rules.MinY+2)))
			entries := definitions.biomes[environment.SpawnBiome(x, y, z)][category]
			entry := weightedRandom(s.rng, entries)
			if entry == nil || entity.GetMobTypeDef(entry.TypeName) == nil {
				continue
			}
			size := entry.MinGroup + s.rng.Intn(entry.MaxGroup-entry.MinGroup+1)
			for range size {
				if global[category] >= cap || len(requests) >= 64 {
					break
				}
				gx, gz := x+s.rng.Intn(5)-2, z+s.rng.Intn(5)-2
				pos := entity.Vec3{X: float64(gx) + .5, Y: float64(y), Z: float64(gz) + .5}
				member := *entry
				if member.TypeName == "slime" {
					inhabited := int64(0)
					if local, ok := environment.(interface{ SpawnInhabitedTime(x, z int) int64 }); ok {
						inhabited = local.SpawnInhabitedTime(gx, gz)
					}
					member.slimeSize = naturalSlimeSize(s.rng, settings, inhabited)
				}
				if !entity.IsTickingAt(blocks, pos) || nearestPlayerDist(pos, players) <= 24*24 || nearestPlayerDist(pos, players) > 128*128 || distanceSquared(pos, settings.Spawn) <= 24*24 || !localAllows(local, players, pos, category, limit.cap) {
					continue
				}
				if !hasEntry(definitions.biomes[environment.SpawnBiome(gx, y, gz)][category], entry.TypeName) || !canPlace(environment, member, pos, rules, s.rng, definitions) {
					continue
				}
				cost := definitions.costAt(environment, entry.TypeName, pos)
				if !potential.allows(pos, cost) {
					continue
				}
				requests = append(requests, SpawnRequest{TypeName: entry.TypeName, X: pos.X, Y: pos.Y, Z: pos.Z, SlimeSize: member.slimeSize})
				potential.add(pos, cost.charge)
				global[category]++
				addLocal(local, players, pos, category)
			}
		}
	}
	return requests
}

// despawn advances idle timers only in active chunks and honors persistence.
func (s *Spawner) despawn(mobs *entity.MobManager, players []PlayerPos, blocks BlockAccess, definitions *spawnDefinitions, peaceful bool) {
	for _, mob := range mobs.Mobs() {
		if mob.Removed() || mob.IsDead() || !entity.IsTickingAt(blocks, mob.Position()) {
			continue
		}
		category := definitions.categories[mob.TypeName()]
		if peaceful && category == Monster {
			mob.Remove()
			continue
		}
		if mob.PersistenceRequired() || category == Creature || category == "" || len(players) == 0 {
			mob.SetNoActionTime(0)
			continue
		}
		distance := nearestPlayerDist(mob.Position(), players)
		if distance <= 32*32 {
			mob.SetNoActionTime(0)
			continue
		}
		mob.SetNoActionTime(min(math.MaxInt32-1, mob.NoActionTime()) + 1)
		if distance > 128*128 || mob.NoActionTime() > 600 && s.rng.Intn(800) == 0 {
			mob.Remove()
		}
	}
}

// canPlace validates the supported species' collision, substrate and light rules.
func canPlace(environment Environment, entry SpawnEntry, pos entity.Vec3, rules DimensionRules, rng *rand.Rand, definitions ...*spawnDefinitions) bool {
	def := entity.GetMobTypeDef(entry.TypeName)
	if def == nil {
		return false
	}
	width, height := def.Width, def.Height
	if entry.TypeName == "slime" {
		width *= float64(max(1, entry.slimeSize))
		height *= float64(max(1, entry.slimeSize))
	}
	if pos.Y < float64(rules.MinY+1) || pos.Y+height >= float64(rules.MinY+rules.Height) {
		return false
	}
	x, y, z := int(math.Floor(pos.X)), int(math.Floor(pos.Y)), int(math.Floor(pos.Z))
	if entry.Group == Monster && environment.SpawnSettings().Peaceful {
		return false
	}
	blockLight, sky, brightness, available := environment.SpawnLight(x, y, z)
	if !available {
		return false
	}
	box := (physics.EntityDimensions{Width: width, Height: height}).MakeBoundingBox(pos.X, pos.Y, pos.Z)
	for bx := int(math.Floor(box.MinX)); bx <= int(math.Floor(box.MaxX-1e-7)); bx++ {
		for by := int(math.Floor(box.MinY)); by <= int(math.Floor(box.MaxY-1e-7)); by++ {
			for bz := int(math.Floor(box.MinZ)); bz <= int(math.Floor(box.MaxZ-1e-7)); bz++ {
				state := environment.GetBlockStateAt(bx, by, bz)
				if state < 0 || entry.TypeName != "drowned" && fluid.ContainedFluidType(state) != fluid.FluidTypeNone {
					return false
				}
			}
		}
	}
	if len(physics.CollectBlockCollisions(environment, box)) != 0 {
		return false
	}
	if entry.TypeName == "drowned" {
		var definition *spawnDefinitions
		if len(definitions) != 0 {
			definition = definitions[0]
		}
		return canPlaceDrowned(environment, pos, rules, rng, definition, blockLight, sky, brightness)
	}
	substrate := environment.GetBlockStateAt(x, y-1, z)
	if entry.Group == Ambient {
		return entry.TypeName == "bat" && y < 63 && brightness <= uint8(rng.Intn(4)) && block.IsSolid(environment.GetBlockStateAt(x, y+1, z))
	}
	if !block.IsSolid(substrate) {
		return false
	}
	below := block.NameFromState(substrate)
	if below == "minecraft:bedrock" || below == "minecraft:barrier" || below == "minecraft:magma_block" {
		return false
	}
	if entry.Group == Creature {
		return below == "minecraft:grass_block" && brightness >= 9
	}
	if entry.Group != Monster {
		return false
	}
	// Phantom spawning belongs to the separate insomnia-driven spawn cycle.
	if entry.TypeName == "phantom" {
		return false
	}
	if entry.TypeName == "slime" {
		var definition *spawnDefinitions
		if len(definitions) != 0 {
			definition = definitions[0]
		}
		return canPlaceSlime(environment, pos, rng, definition, brightness)
	}
	if entry.TypeName == "husk" || entry.TypeName == "stray" {
		if sky != 15 {
			return false
		}
	}
	if entry.TypeName == "blaze" {
		return true
	}
	return darkEnough(blockLight, sky, brightness, rules, rng)
}

// hasEntry checks that a group member's biome permits the selected species.
func hasEntry(entries []SpawnEntry, name string) bool {
	for _, entry := range entries {
		if entry.TypeName == name {
			return true
		}
	}
	return false
}

// nearChunk checks the vanilla local-cap neighborhood around one player.
func nearChunk(player PlayerPos, pos entity.Vec3) bool {
	return math.Abs(math.Floor(player.X/16)-math.Floor(pos.X/16)) <= 8 && math.Abs(math.Floor(player.Z/16)-math.Floor(pos.Z/16)) <= 8
}

// addLocal reserves one mob against every nearby player's category count.
func addLocal(local []map[Category]int, players []PlayerPos, pos entity.Vec3, category Category) {
	for i, player := range players {
		if nearChunk(player, pos) {
			local[i][category]++
		}
	}
}

// localAllows admits a spawn when at least one nearby player has room.
func localAllows(local []map[Category]int, players []PlayerPos, pos entity.Vec3, category Category, cap int) bool {
	for i, player := range players {
		if nearChunk(player, pos) && local[i][category] < cap {
			return true
		}
	}
	return false
}

// distanceSquared compares positions without allocating or taking a square root.
func distanceSquared(a, b entity.Vec3) float64 {
	dx, dy, dz := a.X-b.X, a.Y-b.Y, a.Z-b.Z
	return dx*dx + dy*dy + dz*dz
}
