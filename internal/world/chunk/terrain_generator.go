package chunk

import (
	"github.com/vitismc/vitis/internal/world/terrain"
)

// TerrainGenerator wraps a terrain.NoiseGenerator and implements the chunk.Generator interface.
// It produces lifecycle Chunks with proper section.Chunk game data attached.
type TerrainGenerator struct {
	noise   *terrain.NoiseGenerator
	biomeID int32
}

// NewTerrainGenerator creates a generator backed by noise terrain.
func NewTerrainGenerator(seed int64, biomeID int32) *TerrainGenerator {
	return &TerrainGenerator{
		noise:   terrain.NewNoiseGenerator(seed, biomeID),
		biomeID: biomeID,
	}
}

// Generate produces a lifecycle Chunk with full section.Chunk game data.
func (g *TerrainGenerator) Generate(x, z int32) (*Chunk, error) {
	gameData := g.noise.Generate(x, z)

	c := New(x, z)
	c.SetGameData(gameData)
	gameData.ComputeLight()
	c.SetState(StateLoaded)
	return c, nil
}
