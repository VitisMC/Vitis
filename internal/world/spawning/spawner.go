package spawning

import (
	"math"
	"math/rand"

	"github.com/vitismc/vitis/internal/entity"
)

// MobCap defines the maximum number of mobs per category.
var MobCap = map[entity.MobCategory]int{
	entity.MobCategoryHostile:       70,
	entity.MobCategoryPassive:       10,
	entity.MobCategoryAmbient:       15,
	entity.MobCategoryWaterCreature: 5,
}

// SpawnEntry defines a mob type eligible for spawning.
type SpawnEntry struct {
	TypeName string
	Weight   int
	MinGroup int
	MaxGroup int
	Category entity.MobCategory
}

// DefaultOverworldSpawns provides the default overworld spawn table.
var DefaultOverworldSpawns = []SpawnEntry{
	// Hostile
	{TypeName: "zombie", Weight: 100, MinGroup: 1, MaxGroup: 4, Category: entity.MobCategoryHostile},
	{TypeName: "skeleton", Weight: 100, MinGroup: 1, MaxGroup: 4, Category: entity.MobCategoryHostile},
	{TypeName: "creeper", Weight: 100, MinGroup: 1, MaxGroup: 4, Category: entity.MobCategoryHostile},
	{TypeName: "spider", Weight: 100, MinGroup: 1, MaxGroup: 4, Category: entity.MobCategoryHostile},
	{TypeName: "enderman", Weight: 10, MinGroup: 1, MaxGroup: 4, Category: entity.MobCategoryHostile},
	{TypeName: "witch", Weight: 5, MinGroup: 1, MaxGroup: 1, Category: entity.MobCategoryHostile},
	{TypeName: "slime", Weight: 10, MinGroup: 1, MaxGroup: 4, Category: entity.MobCategoryHostile},
	// Passive
	{TypeName: "cow", Weight: 8, MinGroup: 2, MaxGroup: 4, Category: entity.MobCategoryPassive},
	{TypeName: "pig", Weight: 10, MinGroup: 2, MaxGroup: 4, Category: entity.MobCategoryPassive},
	{TypeName: "sheep", Weight: 12, MinGroup: 2, MaxGroup: 4, Category: entity.MobCategoryPassive},
	{TypeName: "chicken", Weight: 10, MinGroup: 2, MaxGroup: 4, Category: entity.MobCategoryPassive},
	// Ambient
	{TypeName: "bat", Weight: 10, MinGroup: 1, MaxGroup: 8, Category: entity.MobCategoryAmbient},
}

// PlayerPos represents a player position for spawning calculations.
type PlayerPos struct {
	X, Y, Z float64
}

// BlockAccess provides block state queries needed for spawn checks.
type BlockAccess interface {
	GetBlock(x, y, z int) int32
}

// Spawner handles natural mob spawning each tick.
type Spawner struct {
	spawnTable   []SpawnEntry
	spawnCounter int
	interval     int // ticks between spawn attempts
}

// NewSpawner creates a mob spawner with the given spawn table.
func NewSpawner(table []SpawnEntry, interval int) *Spawner {
	if interval <= 0 {
		interval = 20 // attempt every second by default
	}
	return &Spawner{
		spawnTable: table,
		interval:   interval,
	}
}

// Tick runs the spawn cycle. Returns spawn requests for the world to execute.
func (s *Spawner) Tick(mobs *entity.MobManager, players []PlayerPos, blocks BlockAccess) []SpawnRequest {
	s.spawnCounter++
	if s.spawnCounter < s.interval {
		return nil
	}
	s.spawnCounter = 0

	if len(players) == 0 || mobs == nil {
		return nil
	}

	var requests []SpawnRequest

	// Process despawns first
	for _, mob := range mobs.Mobs() {
		if mob.IsDead() || mob.Entity.Removed() {
			continue
		}
		dist := nearestPlayerDist(mob.Position(), players)
		if dist > 128*128 {
			// Instant despawn beyond 128 blocks
			mob.Entity.Remove()
		} else if dist > 32*32 && rand.Intn(800) == 0 {
			// Random despawn between 32-128 blocks
			mob.Entity.Remove()
		}
	}

	// Check mob caps per category and attempt spawns
	for cat, cap := range MobCap {
		current := mobs.CountByCategory(cat)
		if current >= cap {
			continue
		}

		entries := s.entriesForCategory(cat)
		if len(entries) == 0 {
			continue
		}

		// Pick a random player as spawn center
		player := players[rand.Intn(len(players))]

		// Attempt spawn in a 15x15 chunk area around the player (24-128 block radius)
		for attempts := 0; attempts < 3; attempts++ {
			entry := weightedRandom(entries)
			if entry == nil {
				continue
			}

			// Random position 24-128 blocks from player
			angle := rand.Float64() * 2 * math.Pi
			dist := 24.0 + rand.Float64()*104.0
			spawnX := player.X + math.Cos(angle)*dist
			spawnZ := player.Z + math.Sin(angle)*dist

			// Find ground level (simplified: scan down from y=256)
			spawnY := findSpawnY(blocks, int(math.Floor(spawnX)), int(math.Floor(spawnZ)), cat)
			if spawnY < -64 {
				continue
			}

			// Light level check for hostile mobs
			if cat == entity.MobCategoryHostile {
				// Simplified: hostile mobs spawn at y < surface or in dark areas
				// In a real implementation, we'd check sky and block light
				// For now, only allow hostile spawns at night-like conditions (below y=60 or surface)
			}

			groupSize := entry.MinGroup
			if entry.MaxGroup > entry.MinGroup {
				groupSize += rand.Intn(entry.MaxGroup - entry.MinGroup + 1)
			}

			for i := 0; i < groupSize; i++ {
				ox := float64(rand.Intn(5) - 2)
				oz := float64(rand.Intn(5) - 2)
				sx := spawnX + ox
				sz := spawnZ + oz
				sy := findSpawnY(blocks, int(math.Floor(sx)), int(math.Floor(sz)), cat)
				if sy < -64 {
					continue
				}
				requests = append(requests, SpawnRequest{
					TypeName: entry.TypeName,
					X:        sx + 0.5,
					Y:        float64(sy),
					Z:        sz + 0.5,
				})
				current++
				if current >= cap {
					break
				}
			}
			if current >= cap {
				break
			}
		}
	}

	return requests
}

// SpawnRequest describes a mob spawn the world should execute.
type SpawnRequest struct {
	TypeName string
	X, Y, Z  float64
}

func (s *Spawner) entriesForCategory(cat entity.MobCategory) []SpawnEntry {
	var result []SpawnEntry
	for _, e := range s.spawnTable {
		if e.Category == cat {
			result = append(result, e)
		}
	}
	return result
}

func weightedRandom(entries []SpawnEntry) *SpawnEntry {
	if len(entries) == 0 {
		return nil
	}
	total := 0
	for _, e := range entries {
		total += e.Weight
	}
	if total <= 0 {
		return &entries[0]
	}
	r := rand.Intn(total)
	for i := range entries {
		r -= entries[i].Weight
		if r < 0 {
			return &entries[i]
		}
	}
	return &entries[len(entries)-1]
}

func nearestPlayerDist(pos entity.Vec3, players []PlayerPos) float64 {
	minDist := math.MaxFloat64
	for _, p := range players {
		dx := pos.X - p.X
		dy := pos.Y - p.Y
		dz := pos.Z - p.Z
		d := dx*dx + dy*dy + dz*dz
		if d < minDist {
			minDist = d
		}
	}
	return minDist
}

// findSpawnY finds a valid Y coordinate for mob spawning by scanning down.
func findSpawnY(blocks BlockAccess, x, z int, cat entity.MobCategory) int {
	if blocks == nil {
		return 64 // fallback
	}
	// Scan down from y=255 to find the first solid block with 2 air blocks above
	for y := 255; y >= -64; y-- {
		below := blocks.GetBlock(x, y-1, z)
		at := blocks.GetBlock(x, y, z)
		above := blocks.GetBlock(x, y+1, z)
		if below > 0 && at == 0 && above == 0 {
			return y
		}
	}
	return -65 // no valid position found
}
