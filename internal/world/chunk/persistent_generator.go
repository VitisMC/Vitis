package chunk

import (
	"errors"
	"fmt"
	"os"

	"github.com/vitismc/vitis/internal/world/chunk/section"
	"github.com/vitismc/vitis/internal/world/persistence"
	"github.com/vitismc/vitis/internal/world/region"
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
	c, err := g.generateChunk(x, z)
	if err != nil || g.store == nil {
		return c, err
	}
	if c == nil || c.Data() == nil {
		return nil, fmt.Errorf("load chunk (%d,%d): missing section data", x, z)
	}
	data, err := g.store.ReadEntityNBT(int(x), int(z))
	if errors.Is(err, region.ErrNotFound) || os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read entities (%d,%d): %w", x, z, err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("load entities (%d,%d): %w", x, z, region.ErrBadData)
	}
	c.LoadEntityNBT(data)
	return c, nil
}

// generateChunk loads or generates section data on a bounded background worker.
func (g *PersistentGenerator) generateChunk(x, z int32) (*Chunk, error) {
	if g.store != nil {
		nbtData, err := g.store.ReadChunkNBT(int(x), int(z))
		if err == nil {
			if len(nbtData) == 0 {
				return nil, fmt.Errorf("load chunk (%d,%d): %w", x, z, region.ErrBadData)
			}
			var loaded *Chunk
			var decodeErr error
			if bounded, ok := g.fallback.(interface{ Bounds() (int, int, error) }); ok {
				if minY, height, err := bounded.Bounds(); err == nil {
					loaded, decodeErr = DecodeChunkNBTWithBounds(nbtData, x, z, g.biomeID, minY, height)
				} else {
					loaded, decodeErr = DecodeChunkNBT(nbtData, x, z, g.biomeID)
				}
			} else {
				loaded, decodeErr = DecodeChunkNBT(nbtData, x, z, g.biomeID)
			}
			if decodeErr != nil {
				return nil, fmt.Errorf("decode chunk (%d,%d): %w", x, z, decodeErr)
			}
			if loaded == nil || loaded.Data() == nil {
				return nil, fmt.Errorf("decode chunk (%d,%d): empty chunk", x, z)
			}
			loaded.Data().ComputeLight()
			loaded.SetState(StateLoaded)
			return loaded, nil
		}
		if !errors.Is(err, region.ErrNotFound) && !os.IsNotExist(err) {
			return nil, fmt.Errorf("read chunk (%d,%d): %w", x, z, err)
		}
	}

	if g.fallback != nil {
		return g.fallback.Generate(x, z)
	}

	c := New(x, z)
	gd := section.NewChunk(x, z, section.OverworldSections, g.biomeID)
	c.SetData(gd)
	gd.ComputeLight()
	c.MarkDirty()
	c.SetState(StateLoaded)
	return c, nil
}
