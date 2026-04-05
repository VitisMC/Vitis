package chunk

import (
	"github.com/vitismc/vitis/internal/world/chunk/section"
	"github.com/vitismc/vitis/internal/world/persistence"
)

// PersistentGenerator tries to load chunks from disk (Anvil region files),
// falling back to terrain generation when the chunk doesn't exist on disk.
type PersistentGenerator struct {
	store    *persistence.ChunkStore
	fallback Generator
	biomeID  int32
}

// NewPersistentGenerator creates a generator that reads from disk first.
func NewPersistentGenerator(store *persistence.ChunkStore, fallback Generator, biomeID int32) *PersistentGenerator {
	return &PersistentGenerator{
		store:    store,
		fallback: fallback,
		biomeID:  biomeID,
	}
}

// Generate attempts to load a chunk from the Anvil region store.
// If not found on disk, delegates to the fallback generator.
func (g *PersistentGenerator) Generate(x, z int32) (*Chunk, error) {
	if g.store != nil {
		nbtData, err := g.store.ReadChunkNBT(int(x), int(z))
		if err == nil && len(nbtData) > 0 {
			gameData, decodeErr := DecodeChunkNBT(nbtData, x, z, g.biomeID)
			if decodeErr == nil && gameData != nil {
				c := New(x, z)
				c.SetGameData(gameData)
				gameData.ComputeLight()
				c.SetState(StateLoaded)
				return c, nil
			}
		}
	}

	if g.fallback != nil {
		return g.fallback.Generate(x, z)
	}

	c := New(x, z)
	gd := section.NewChunk(x, z, section.OverworldSections, g.biomeID)
	c.SetGameData(gd)
	gd.ComputeLight()
	c.SetState(StateLoaded)
	return c, nil
}
