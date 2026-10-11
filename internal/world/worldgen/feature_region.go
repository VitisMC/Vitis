package worldgen

import (
	"container/list"
	"fmt"
	"sync"

	blockentity "github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	"github.com/vitismc/vitis/internal/world/worldgen/climate"
)

// terrainBaseCacheLimit covers overlapping column32 dependencies without repeated terrain work.
const terrainBaseCacheLimit = 12288

// terrainBaseSeamReserve retains both sides of a band transition at the tested 1251-column width.
const terrainBaseSeamReserve = 10240

// baseBiomeColumn retains climate values without sharing mutable search cursors.
type baseBiomeColumn struct {
	target climate.Target
	set    bool
}

// baseChunkEntry publishes immutable terrain and structures before decoration begins.
type baseChunkEntry struct {
	ready      chan struct{}
	chunk      *section.Chunk
	biomeGrid  []int32
	columns    [36]baseBiomeColumn
	err        error
	lru        *list.Element
	seam       bool
	heightOnce [4]sync.Once
	heights    [4][256]int16
	structures []*structureStartPlan
}

// height memoizes immutable terrain heightmaps without mutating the shared chunk.
func (e *baseChunkEntry) height(x, z int, kind section.HeightmapKind) (int, bool) {
	if kind >= section.HeightmapKind(len(e.heights)) {
		return e.chunk.MinY(), false
	}
	e.heightOnce[kind].Do(func() {
		for columnZ := 0; columnZ < 16; columnZ++ {
			for columnX := 0; columnX < 16; columnX++ {
				height, _ := e.chunk.Heightmap(columnX, columnZ, kind)
				e.heights[kind][columnZ*16+columnX] = int16(height)
			}
		}
	})
	height := int(e.heights[kind][(z&15)*16+(x&15)])
	return height, height > e.chunk.MinY()
}

// featureRegion resolves adjacent terrain without waiting for any decoration task.
type featureRegion struct {
	generator        *TerrainGenerator
	snapshot         *terrainSnapshot
	sourceX, sourceZ int32
	neighbors        [9]*baseChunkEntry
	farNeighbors     map[[2]int32]*baseChunkEntry
	radius           int
	err              error
}

// prepare completes unclaimed neighboring terrain before waiting on another worker's dependencies.
func (r *featureRegion) prepare() {
	for dx := int32(-1); dx <= 1; dx++ {
		for dz := int32(-1); dz <= 1; dz++ {
			if dx == 0 && dz == 0 {
				continue
			}
			entry := r.generator.terrainBaseFor(r.snapshot, r.sourceX+dx, r.sourceZ+dz, nil, false)
			r.neighbors[(dx+1)*3+dz+1] = entry
			if entry != nil && entry.err != nil && r.err == nil {
				r.err = entry.err
			}
		}
	}
}

// neighbor memoizes the nine immutable dependencies used by one decoration worker.
func (r *featureRegion) neighbor(x, z int) *baseChunkEntry {
	dx, dz := (x>>4)-int(r.sourceX), (z>>4)-int(r.sourceZ)
	radius := max(1, r.radius)
	if dx < -radius || dx > radius || dz < -radius || dz > radius {
		return nil
	}
	if dx < -1 || dx > 1 || dz < -1 || dz > 1 {
		key := [2]int32{int32(x >> 4), int32(z >> 4)}
		if entry := r.farNeighbors[key]; entry != nil {
			return entry
		}
		entry := r.generator.terrainBase(r.snapshot, key[0], key[1], nil)
		if r.farNeighbors == nil {
			r.farNeighbors = make(map[[2]int32]*baseChunkEntry)
		}
		r.farNeighbors[key] = entry
		if entry.err != nil && r.err == nil {
			r.err = entry.err
		}
		return entry
	}
	index := (dx+1)*3 + dz + 1
	entry := r.neighbors[index]
	if entry == nil {
		entry = r.generator.terrainBase(r.snapshot, int32(x>>4), int32(z>>4), nil)
		r.neighbors[index] = entry
		if entry.err != nil && r.err == nil {
			r.err = entry.err
		}
	}
	return entry
}

// block reads immutable terrain after the source's own border writes have been checked.
func (r *featureRegion) block(x, y, z int) int32 {
	if y < r.snapshot.minY || y >= r.snapshot.minY+r.snapshot.height {
		return 0
	}
	entry := r.neighbor(x, z)
	if entry == nil || entry.err != nil {
		return 0
	}
	return entry.chunk.GetBlock(x, y, z)
}

// height reads a neighboring heightmap without modifying its cached chunk.
func (r *featureRegion) height(x, z int, kind section.HeightmapKind) (int, bool) {
	entry := r.neighbor(x, z)
	if entry == nil || entry.err != nil {
		return r.snapshot.minY, false
	}
	return entry.height(x, z, kind)
}

// terrainBase coalesces concurrent terrain requests and evicts completed entries only.
func (g *TerrainGenerator) terrainBase(snapshot *terrainSnapshot, x, z int32, biomes *cachedBiomeSource) *baseChunkEntry {
	return g.terrainBaseFor(snapshot, x, z, biomes, true)
}

// terrainBaseFor optionally defers active dependencies so workers can prepare other terrain first.
func (g *TerrainGenerator) terrainBaseFor(snapshot *terrainSnapshot, x, z int32, biomes *cachedBiomeSource, wait bool) *baseChunkEntry {
	key := rawChunkKey{snapshot: snapshot, x: x, z: z}
	g.baseMu.Lock()
	if g.baseCache == nil {
		g.baseCache = make(map[rawChunkKey]*baseChunkEntry)
	}
	if entry := g.baseCache[key]; entry != nil {
		if entry.lru != nil {
			g.baseEntryQueue(entry).MoveToBack(entry.lru)
		}
		g.baseMu.Unlock()
		if wait {
			<-entry.ready
		} else {
			select {
			case <-entry.ready:
			default:
				return nil
			}
		}
		if d := g.diagnostics.Load(); d != nil {
			d.recordBaseHit()
		}
		return entry
	}
	entry := &baseChunkEntry{ready: make(chan struct{})}
	g.baseCache[key] = entry
	g.baseMu.Unlock()
	if biomes == nil {
		if cached := snapshot.biomePool.Get(); cached != nil {
			biomes = cached.(*cachedBiomeSource)
			biomes.reset(snapshot.biomes)
		} else {
			biomes = newCachedBiomeSource(snapshot.biomes)
		}
		defer snapshot.biomePool.Put(biomes)
		if g.diagnostics.Load() != nil && biomes.climateColumns == nil {
			biomes.climateColumns = make(map[[2]int]struct{})
		}
	}
	biomes.beginChunk(int(x)*16, int(z)*16, snapshot.minY, snapshot.height)
	entry.structures, entry.err = snapshot.structures.references(x, z)
	if entry.err == nil {
		entry.chunk, entry.err = g.generateTerrainBaseWithStructures(snapshot, x, z, biomes, entry.structures)
	}
	if entry.err == nil {
		entry.biomeGrid = append([]int32(nil), biomes.grid...)
		for index, column := range biomes.columns {
			entry.columns[index] = baseBiomeColumn{column.target, column.set}
		}
	}
	if d := g.diagnostics.Load(); d != nil {
		d.recordBase(key, biomes)
	}
	g.baseMu.Lock()
	close(entry.ready)
	if g.baseActive != nil && g.baseActive != snapshot {
		delete(g.baseCache, key)
	} else {
		entry.seam = g.isBaseSeam(z)
		entry.lru = g.baseEntryQueue(entry).PushBack(key)
		g.evictTerrainBases()
	}
	g.baseMu.Unlock()
	return entry
}

// evictTerrainBases retains band-edge dependencies within the overall cache limit.
// The caller holds baseMu and only completed entries are linked in either queue.
func (g *TerrainGenerator) evictTerrainBases() {
	for g.baseLRU.Len()+g.baseSeamLRU.Len() > terrainBaseCacheLimit {
		queue := &g.baseLRU
		if g.baseSeamLRU.Len() > terrainBaseSeamReserve || queue.Len() == 0 {
			queue = &g.baseSeamLRU
		}
		front := queue.Front()
		oldKey := front.Value.(rawChunkKey)
		g.baseCache[oldKey].lru = nil
		delete(g.baseCache, oldKey)
		queue.Remove(front)
	}
}

// isBaseSeam protects neighboring terrain at the edges of a 32-row generation band.
func (g *TerrainGenerator) isBaseSeam(z int32) bool {
	row := (int(z) - int(g.baseBandZ.Load())) & 31
	return row <= 1 || row >= 30
}

// baseEntryQueue selects the eviction queue assigned when an entry was published.
func (g *TerrainGenerator) baseEntryQueue(entry *baseChunkEntry) *list.List {
	if entry.seam {
		return &g.baseSeamLRU
	}
	return &g.baseLRU
}

// generateTerrainBase fills terrain, surface, carvers, and existing structures without feature recursion.
func (g *TerrainGenerator) generateTerrainBase(snapshot *terrainSnapshot, x, z int32, biomes *cachedBiomeSource) (*section.Chunk, error) {
	return g.generateTerrainBaseWithStructures(snapshot, x, z, biomes, nil)
}

// generateTerrainBaseWithStructures keeps planned starts separate from later stage-specific piece writes.
func (g *TerrainGenerator) generateTerrainBaseWithStructures(snapshot *terrainSnapshot, x, z int32, biomes *cachedBiomeSource, starts []*structureStartPlan) (*section.Chunk, error) {
	result := section.NewChunkWithBounds(x, z, snapshot.minY, snapshot.height, g.biomeID)
	var blocks []int32
	if cached := snapshot.blockPool.Get(); cached != nil {
		blocks = cached
		clear(blocks)
	} else {
		blocks = make([]int32, len(result.Sections)*section.BlocksPerSection)
	}
	if err := result.UseDenseBlocks(blocks); err != nil {
		snapshot.blockPool.Put(blocks)
		return nil, err
	}
	defer snapshot.blockPool.Put(blocks)
	shape := snapshot.settings.Noise
	heights := g.generateTerrainRangeWithStructures(snapshot, result, int(x)*16, int(z)*16, shape.MinY, shape.MinY+shape.Height, densityForStructures(starts))
	populateBiomes(result, biomes, x, z)
	treasureHeight, _ := result.Heightmap(8, 8, section.HeightmapOceanFloor)
	snapshot.surface.applyWithHeights(result, x, z, biomes, heights)
	snapshot.carvers.apply(result, x, z)
	result.PackDenseBlocks()
	placeBuriedTreasure(snapshot.resources, result, biomes, treasureHeight, g.seed, x, z)
	recordStructurePlans(result, starts)
	return result, nil
}

// cloneTerrainBase copies section state, structures, and block entities into owned decoration storage.
func cloneTerrainBase(source *section.Chunk) (*section.Chunk, error) {
	result := source.CloneRaw()
	if root := source.PersistentNBT(); root != nil {
		result.SetPersistentNBT(root)
	}
	for _, entity := range source.BlockEntities {
		var root *nbt.Compound
		if persistent, ok := entity.(interface{ PersistentNBT() *nbt.Compound }); ok {
			root = persistent.PersistentNBT()
		} else {
			var err error
			root, err = nbt.NewDecoder(entity.ChunkDataNBT()).ReadRootCompound()
			if err != nil {
				return nil, fmt.Errorf("clone generated entity %s: %w", entity.TypeName(), err)
			}
		}
		clone := blockentity.NewRawBlockEntity(root)
		if clone == nil {
			return nil, fmt.Errorf("clone generated entity %s: invalid persistent identity", entity.TypeName())
		}
		result.AddBlockEntity(clone)
	}
	return result, nil
}

// pruneTerrainBases releases completed dependencies from superseded resource snapshots.
func (g *TerrainGenerator) pruneTerrainBases(snapshot *terrainSnapshot) {
	g.baseMu.Lock()
	defer g.baseMu.Unlock()
	g.baseActive = snapshot
	for key, entry := range g.baseCache {
		if key.snapshot != snapshot && entry.lru != nil {
			g.baseEntryQueue(entry).Remove(entry.lru)
			entry.lru = nil
			delete(g.baseCache, key)
		}
	}
}

// mergeRegionBorders applies canonical writes rather than limiting replacement to air and leaves.
func mergeRegionBorders(target *section.Chunk, sourceX, sourceZ int32, entry *rawChunkEntry) {
	if entry.writes != nil {
		entry.writes.ForChunk(target.X, target.Z, func(position section.BorderPosition, write section.FeatureBorderWrite) {
			if target.ApplyFeatureBorder(sourceX, sourceZ, position.X, position.Y, position.Z, write.State, write.Priority) {
				target.RemoveBlockEntity(int32(position.X), int32(position.Y), int32(position.Z))
				if root := entry.entities[position]; root != nil {
					target.AddBlockEntity(blockentity.NewRawBlockEntity(root))
				}
			}
		})
		return
	}
	if !target.FeatureRegionEnabled() {
		mergeFeatureBorders(target, entry.border, entry.entities)
		return
	}
	for position, state := range entry.border {
		if target.ApplyFeatureBorder(sourceX, sourceZ, position.X, position.Y, position.Z, state, entry.priorities[position]) {
			target.RemoveBlockEntity(int32(position.X), int32(position.Y), int32(position.Z))
			if root := entry.entities[position]; root != nil {
				target.AddBlockEntity(blockentity.NewRawBlockEntity(root))
			}
		}
	}
}
