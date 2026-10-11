package world

import (
	"math"

	biomedata "github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/spawning"
)

// SpawnSettings reads the world's authoritative natural-spawn policy.
func (w *World) SpawnSettings() spawning.Settings {
	x, y, z, _ := w.WorldSpawn()
	settings := spawning.Settings{Dimension: w.Dimension(), Enabled: w.GameRule("doMobSpawning") == "true", Peaceful: w.Difficulty() == 0, Spawn: entity.Vec3{X: float64(x), Y: float64(y), Z: float64(z)}}
	settings.Seed, settings.DayTime, settings.WorldAge, settings.Difficulty = w.Seed(), w.TimeOfDay(), w.WorldAge(), w.Difficulty()
	if w.readSeaLevel != nil {
		settings.SeaLevel, settings.SeaLevelKnown = w.readSeaLevel()
	}
	return settings
}

// SpawnInhabitedTime reads local difficulty state without loading terrain.
func (w *World) SpawnInhabitedTime(x, z int) int64 {
	if c, ok := w.chunks.PeekLoaded(int32(x>>4), int32(z>>4)); ok {
		return c.InhabitedTime()
	}
	return 0
}

// tickChunkInhabitation advances saved counters once for resident spawn-candidate chunks.
func (w *World) tickChunkInhabitation() {
	players := w.buildSpawnPlayerList()
	if len(players) == 0 {
		return
	}
	w.chunks.VisitLoaded(func(c *chunk.Chunk) {
		if !w.IsBlockTicking(c.X(), c.Z()) {
			return
		}
		for _, player := range players {
			dx, dz := player.X-(float64(c.X())*16+8), player.Z-(float64(c.Z())*16+8)
			if dx*dx+dz*dz < 128*128 {
				c.AddInhabitedTime(1)
				return
			}
		}
	})
}

// SpawnChunks lists active resident chunks within the local-cap range of players.
func (w *World) SpawnChunks(players []spawning.PlayerPos) []spawning.ChunkPos {
	var positions []spawning.ChunkPos
	w.chunks.VisitLoaded(func(c *chunk.Chunk) {
		if !w.IsEntityTicking(c.X(), c.Z()) {
			return
		}
		for _, player := range players {
			px, pz := int32(math.Floor(player.X/16)), int32(math.Floor(player.Z/16))
			if math.Abs(float64(c.X()-px)) <= 8 && math.Abs(float64(c.Z()-pz)) <= 8 {
				positions = append(positions, spawning.ChunkPos{X: c.X(), Z: c.Z()})
				break
			}
		}
	})
	return positions
}

// SpawnBiome reads the stored quart biome at a resident block position.
func (w *World) SpawnBiome(x, y, z int) string {
	c, ok := w.chunks.PeekLoaded(int32(x>>4), int32(z>>4))
	if !ok || y < c.Data().MinY() || y > c.Data().MaxY() {
		return ""
	}
	s := c.Data().Sections[(y-c.Data().MinY())>>4]
	if biome := biomedata.BiomeByID(s.GetBiome((x&15)>>2, (y&15)>>2, (z&15)>>2)); biome != nil {
		return biome.Name
	}
	return ""
}

// SpawnLight returns local light with time- and weather-dependent sky darkening.
func (w *World) SpawnLight(x, y, z int) (uint8, uint8, uint8, bool) {
	block, sky, ok := w.LightLevels(x, y, z)
	if !ok {
		return 0, 0, 0, false
	}
	darken := w.skyDarkening()
	if w.weather.IsThundering() {
		darken = 10
	}
	return block, sky, max(block, uint8(max(0, int(sky)-darken))), true
}

// skyDarkening calculates the daylight curve and current weather attenuation.
func (w *World) skyDarkening() int {
	fraction := float64((w.TimeOfDay()%24000+24000)%24000)/24000 - .25
	fraction -= math.Floor(fraction)
	angle := (fraction*2 + .5 - math.Cos(fraction*math.Pi)/2) / 3
	daylight := .5 + 2*math.Max(-.25, math.Min(.25, math.Cos(angle*2*math.Pi)))
	rain := 1 - float64(w.weather.RainLevel())*5/16
	thunder := 1 - float64(w.weather.ThunderLevel())*5/16
	return int((1 - daylight*rain*thunder) * 11)
}
