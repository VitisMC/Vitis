package worldgen

import (
	"container/list"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/vitismc/vitis/internal/block"
	blockentity "github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	"github.com/vitismc/vitis/internal/world/worldgen/density"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// TerrainGenerator fills base terrain from the active noise router snapshot.
type TerrainGenerator struct {
	runtime         *Runtime
	settingsID      string
	dimensionTypeID string
	seed            int64
	biomeID         int32
	biomeSource     json.RawMessage
	bonusChest      bool
	bonusChunkX     int32
	bonusChunkZ     int32

	mu            sync.Mutex
	generationMu  sync.RWMutex
	resource      *ResourceSet
	compiled      *terrainSnapshot
	rawMu         sync.Mutex
	rawCache      map[rawChunkKey]*rawChunkEntry
	rawClock      uint64
	rawTerrains   int
	rawBorders    int
	diagnostics   atomic.Pointer[terrainDiagnostics]
	terrainLRU    list.List
	borderLRU     list.List
	borderSeamLRU list.List
	baseMu        sync.Mutex
	baseCache     map[rawChunkKey]*baseChunkEntry
	baseLRU       list.List
	baseSeamLRU   list.List
	baseBandZ     atomic.Int32
	baseActive    *terrainSnapshot
}

const rawChunkCacheLimit = 1024

// EnableBonusChest selects the spawn chunk for optional initial chest placement.
// It must be called before any concurrent generation starts.
func (g *TerrainGenerator) EnableBonusChest(spawnX, spawnZ int32) {
	g.bonusChest = true
	g.bonusChunkX, g.bonusChunkZ = spawnX>>4, spawnZ>>4
}

// rawBorderCacheLimit retains small neighboring-write results separately from full terrain.
const rawBorderCacheLimit = 18432

// rawBorderSeamReserve protects far-feature transitions within the bounded memory budget.
const rawBorderSeamReserve = 15360

type rawChunkKey struct {
	snapshot *terrainSnapshot
	x, z     int32
}

type rawChunkEntry struct {
	ready      chan struct{}
	chunk      *section.Chunk
	border     map[section.BorderPosition]int32
	entities   map[section.BorderPosition]*nbt.Compound
	priorities map[section.BorderPosition]uint32
	writes     *section.FeatureBorderBuffer
	err        error
	lastUse    uint64
	complete   bool
	taken      bool
	lru        *list.Element
	seam       bool
}

type terrainSnapshot struct {
	resources        *ResourceSet
	settings         NoiseSettings
	minY             int
	height           int
	finalDensity     density.Function
	interpolation    *density.ChunkInterpolation
	blockState       int32
	fluidState       int32
	aquifer          aquiferConfig
	biomes           biomeSource
	surface          *surfaceSystem
	oreVeins         *oreVeinifier
	carvers          *carverSystem
	features         *featureSystem
	featureRadius    int
	farFeatureBiomes map[string]int
	farUnfiltered    int
	evaluationPool   sync.Pool
	biomePool        sync.Pool
	aquiferPool      sync.Pool
	blockPool        terrainBufferPool[int32]
	priorityPool     terrainBufferPool[uint32]
	structures       *structurePlanner
}

// sourceMayWriteFar conservatively tests all quart biomes of a distant source.
func (s *terrainSnapshot) sourceMayWriteFar(x, z int32, distance int) bool {
	if s == nil || s.features == nil {
		return false
	}
	if s.farUnfiltered >= distance {
		return true
	}
	if len(s.farFeatureBiomes) == 0 {
		return false
	}
	for _, name := range s.features.biomeNamesInChunk(x, z) {
		if s.farFeatureBiomes[name] >= distance {
			return true
		}
	}
	return false
}

// NewTerrainGenerator creates a reload-aware seeded terrain generator.
func (r *Runtime) NewTerrainGenerator(settingsID string, seed int64, biomeID int32) (*TerrainGenerator, error) {
	dimensionTypeID := map[string]string{"minecraft:overworld": "minecraft:overworld", "minecraft:nether": "minecraft:the_nether", "minecraft:end": "minecraft:the_end"}[settingsID]
	biomeSources := map[string]json.RawMessage{
		"minecraft:overworld": json.RawMessage(`{"type":"minecraft:multi_noise","preset":"minecraft:overworld"}`),
		"minecraft:nether":    json.RawMessage(`{"type":"minecraft:multi_noise","preset":"minecraft:nether"}`),
		"minecraft:end":       json.RawMessage(`{"type":"minecraft:the_end"}`),
	}
	return r.newTerrainGenerator(settingsID, dimensionTypeID, biomeSources[settingsID], seed, biomeID)
}

// NewDimensionTerrainGenerator creates a generator from a data-driven dimension definition.
func (r *Runtime) NewDimensionTerrainGenerator(dimensionID string, seed int64, biomeID int32) (*TerrainGenerator, error) {
	dimension, ok := r.Dimension(dimensionID)
	if !ok {
		return nil, fmt.Errorf("create terrain generator: unknown dimension %s", dimensionID)
	}
	if dimension.Generator.Type != "minecraft:noise" {
		return nil, fmt.Errorf("create terrain generator: unsupported generator %s", dimension.Generator.Type)
	}
	return r.newTerrainGenerator(dimension.Generator.Settings, dimension.Type, dimension.Generator.BiomeSource, seed, biomeID)
}

func (r *Runtime) newTerrainGenerator(settingsID, dimensionTypeID string, biomeSource json.RawMessage, seed int64, biomeID int32) (*TerrainGenerator, error) {
	generator := &TerrainGenerator{
		runtime: r, settingsID: settingsID, dimensionTypeID: dimensionTypeID,
		seed: seed, biomeID: biomeID, biomeSource: append(json.RawMessage(nil), biomeSource...),
	}
	generator.baseBandZ.Store(1<<31 - 1)
	if _, err := generator.snapshot(); err != nil {
		return nil, err
	}
	return generator, nil
}

// Generate fills one chunk column from base density and default fluid settings.
func (g *TerrainGenerator) Generate(chunkX, chunkZ int32) (*section.Chunk, error) {
	if g == nil {
		return nil, fmt.Errorf("generate terrain: nil generator")
	}
	g.generationMu.RLock()
	defer g.generationMu.RUnlock()
	for {
		origin := g.baseBandZ.Load()
		if chunkZ >= origin || g.baseBandZ.CompareAndSwap(origin, chunkZ) {
			break
		}
	}
	snapshot, err := g.snapshot()
	if err != nil {
		return nil, err
	}
	// A target already running on another worker must not prevent this worker
	// from preparing its missing neighboring sources before waiting.
	entry := g.rawChunkFor(snapshot, chunkX, chunkZ, true, false)
	var result *section.Chunk
	defer func() {
		if result != nil {
			if writes := result.ReleaseFeatureRegion(); writes != nil {
				snapshot.priorityPool.Put(writes)
			}
		}
	}()
	if entry != nil {
		result, err = g.takeRawChunk(snapshot, chunkX, chunkZ)
		if err != nil {
			return nil, err
		}
	}
	radius := max(1, snapshot.featureRadius)
	width := radius*2 + 1
	neighbors := make([]*rawChunkEntry, 0, width*width-1)
	coordinates := make([][2]int32, 0, width*width-1)
	for dx := -radius; dx <= radius; dx++ {
		for dz := -radius; dz <= radius; dz++ {
			if dx == 0 && dz == 0 {
				continue
			}
			coordinate := [2]int32{chunkX + int32(dx), chunkZ + int32(dz)}
			if distance := max(absInt(dx), absInt(dz)); distance > 1 && !snapshot.sourceMayWriteFar(coordinate[0], coordinate[1], distance) {
				continue
			}
			coordinates = append(coordinates, coordinate)
			neighbors = append(neighbors, g.rawChunkFor(snapshot, coordinate[0], coordinate[1], false, false))
		}
	}
	if result == nil {
		result, err = g.takeRawChunk(snapshot, chunkX, chunkZ)
		if err != nil {
			return nil, err
		}
	}
	// Prepare missing neighbors before waiting on work already running elsewhere.
	// Border merging retains the original order regardless of completion order.
	for index, entry := range neighbors {
		if entry == nil {
			entry = g.rawChunkFor(snapshot, coordinates[index][0], coordinates[index][1], false, false)
		}
		if entry == nil {
			entry = g.rawChunkFor(snapshot, coordinates[index][0], coordinates[index][1], false, true)
		}
		if entry.err != nil {
			return nil, entry.err
		}
		mergeRegionBorders(result, coordinates[index][0], coordinates[index][1], entry)
	}
	if writes := result.ReleaseFeatureRegion(); writes != nil {
		snapshot.priorityPool.Put(writes)
	}
	result.DropBorderBlocks()
	return result, nil
}

// generateRaw produces one chunk and records tree blocks outside its boundary.
func (g *TerrainGenerator) generateRaw(snapshot *terrainSnapshot, chunkX, chunkZ int32) (*section.Chunk, error) {
	baseX, baseZ := int(chunkX)*16, int(chunkZ)*16
	var biomes *cachedBiomeSource
	if cached := snapshot.biomePool.Get(); cached != nil {
		biomes = cached.(*cachedBiomeSource)
		biomes.reset(snapshot.biomes)
	} else {
		biomes = newCachedBiomeSource(snapshot.biomes)
	}
	defer snapshot.biomePool.Put(biomes)
	if diagnostics := g.diagnostics.Load(); diagnostics != nil {
		if biomes.climateColumns == nil {
			biomes.climateColumns = make(map[[2]int]struct{})
		}
		defer diagnostics.record(rawChunkKey{snapshot: snapshot, x: chunkX, z: chunkZ}, biomes)
	}
	biomes.beginChunk(baseX, baseZ, snapshot.minY, snapshot.height)
	base := g.terrainBaseFor(snapshot, chunkX, chunkZ, biomes, false)
	region := &featureRegion{generator: g, snapshot: snapshot, sourceX: chunkX, sourceZ: chunkZ, radius: max(1, snapshot.featureRadius)}
	region.prepare()
	if region.err != nil {
		return nil, region.err
	}
	if base == nil {
		base = g.terrainBase(snapshot, chunkX, chunkZ, biomes)
	}
	region.neighbors[4] = base
	if base.err != nil {
		return nil, base.err
	}
	result, err := cloneTerrainBase(base.chunk)
	if err != nil {
		return nil, err
	}
	biomes.reset(snapshot.biomes)
	biomes.beginChunk(baseX, baseZ, snapshot.minY, snapshot.height)
	copy(biomes.grid, base.biomeGrid)
	for index, column := range base.columns {
		biomes.columns[index].target, biomes.columns[index].set = column.target, column.set
	}
	result.EnableHeightmapCache()
	var writes []uint32
	if cached := snapshot.priorityPool.Get(); cached != nil {
		writes = cached
	} else {
		writes = make([]uint32, len(result.Sections)*section.BlocksPerSection)
	}
	if err := result.UseFeatureRegionRadius(region.block, region.height, writes, region.radius); err != nil {
		snapshot.priorityPool.Put(writes)
		return nil, err
	}
	snapshot.features.applyWithStructures(result, chunkX, chunkZ, biomes, func(step int) { region.placeStructureStage(result, step) })
	if region.err != nil {
		snapshot.priorityPool.Put(result.ReleaseFeatureRegion())
		return nil, region.err
	}
	result.SetFeatureWriteRadius(1)
	if g.bonusChest && chunkX == g.bonusChunkX && chunkZ == g.bonusChunkZ {
		feature, ok := snapshot.resources.features.configured["minecraft:bonus_chest"].(*bonusChestFeature)
		if !ok {
			snapshot.priorityPool.Put(result.ReleaseFeatureRegion())
			return nil, fmt.Errorf("bonus chest feature is unavailable")
		}
		result.SetFeatureStep(15, 0)
		seed := g.seed ^ int64(chunkX)*341873128712 ^ int64(chunkZ)*132897987541
		feature.placeFeature(snapshot.features, result, worldrandom.NewLegacy(seed), blockPosition{int(chunkX) * 16, 0, int(chunkZ) * 16})
	}
	result.FinishFeatureReads()
	result.FeatureBorderWrites().Compact()
	return result, nil
}

// takeRawChunk transfers terrain ownership while retaining immutable neighboring writes.
func (g *TerrainGenerator) takeRawChunk(snapshot *terrainSnapshot, x, z int32) (*section.Chunk, error) {
	key := rawChunkKey{snapshot: snapshot, x: x, z: z}
	for {
		entry := g.rawChunk(snapshot, x, z)
		if entry.err != nil {
			return nil, entry.err
		}
		g.rawMu.Lock()
		if entry.taken {
			g.rawMu.Unlock()
			continue
		}
		entry.taken = true
		if g.rawCache[key] == entry {
			g.removeRawEntry(entry)
			borderEntry := &rawChunkEntry{border: entry.border, entities: entry.entities, priorities: entry.priorities, writes: entry.writes, lastUse: entry.lastUse, complete: true}
			g.rawCache[key] = borderEntry
			g.queueRawEntry(key, borderEntry)
			g.rawTerrains--
			g.rawBorders++
			g.evictRawChunks()
		}
		g.rawMu.Unlock()
		return entry.chunk, nil
	}
}

// rawChunk generates or waits for one source chunk in the bounded cache.
func (g *TerrainGenerator) rawChunk(snapshot *terrainSnapshot, x, z int32) *rawChunkEntry {
	return g.rawChunkFor(snapshot, x, z, true, true)
}

// rawChunkFor reuses border-only entries without rebuilding already consumed terrain.
func (g *TerrainGenerator) rawChunkFor(snapshot *terrainSnapshot, x, z int32, needTerrain, wait bool) *rawChunkEntry {
	key := rawChunkKey{snapshot: snapshot, x: x, z: z}
	for {
		g.rawMu.Lock()
		if g.rawCache == nil {
			g.rawCache = make(map[rawChunkKey]*rawChunkEntry)
		}
		entry := g.rawCache[key]
		if entry != nil {
			g.rawClock++
			entry.lastUse = g.rawClock
			if entry.lru != nil {
				g.rawEntryQueue(entry).MoveToBack(entry.lru)
			}
		}
		if entry != nil && entry.ready != nil {
			ready := entry.ready
			g.rawMu.Unlock()
			if !wait {
				return nil
			}
			<-ready
			continue
		}
		if entry != nil && (entry.err != nil || entry.chunk != nil || !needTerrain && entry.complete) {
			g.rawMu.Unlock()
			return entry
		}
		if entry == nil || entry.complete {
			if entry != nil {
				g.rawBorders--
				g.removeRawEntry(entry)
			}
			entry = &rawChunkEntry{}
			g.rawCache[key] = entry
		}
		g.rawClock++
		entry.lastUse = g.rawClock
		entry.ready = make(chan struct{})
		g.rawMu.Unlock()

		chunk, err := g.generateRaw(snapshot, x, z)
		g.rawMu.Lock()
		entry.chunk, entry.err, entry.complete = chunk, err, true
		if chunk != nil {
			entry.writes = chunk.FeatureBorderWrites()
			for position, be := range chunk.BorderBlockEntities() {
				if persistent, ok := be.(interface{ PersistentNBT() *nbt.Compound }); ok {
					if root := persistent.PersistentNBT(); root != nil {
						if entry.entities == nil {
							entry.entities = make(map[section.BorderPosition]*nbt.Compound)
						}
						entry.entities[position] = root
					}
				}
			}
		}
		close(entry.ready)
		entry.ready = nil
		g.queueRawEntry(key, entry)
		if chunk != nil {
			g.rawTerrains++
		} else {
			g.rawBorders++
		}
		g.evictRawChunks()
		g.rawMu.Unlock()
		return entry
	}
}

// evictRawChunks bounds cached terrain and border writes after generation.
func (g *TerrainGenerator) evictRawChunks() {
	for g.rawTerrains > rawChunkCacheLimit || g.rawBorders > rawBorderCacheLimit {
		evictTerrain := g.rawTerrains > rawChunkCacheLimit
		queue := &g.borderLRU
		if evictTerrain {
			queue = &g.terrainLRU
		} else if g.borderSeamLRU.Len() > rawBorderSeamReserve || queue.Len() == 0 {
			queue = &g.borderSeamLRU
		}
		front := queue.Front()
		if front == nil {
			break
		}
		oldestKey := front.Value.(rawChunkKey)
		oldest := g.rawCache[oldestKey]
		g.removeRawEntry(oldest)
		delete(g.rawCache, oldestKey)
		if evictTerrain {
			g.rawTerrains--
		} else {
			g.rawBorders--
		}
	}
}

// rawEntryQueue separates reusable future terrain from small consumed-border entries.
func (g *TerrainGenerator) rawEntryQueue(entry *rawChunkEntry) *list.List {
	if entry.chunk != nil {
		return &g.terrainLRU
	}
	if entry.seam {
		return &g.borderSeamLRU
	}
	return &g.borderLRU
}

// queueRawEntry publishes only completed entries into bounded eviction order.
func (g *TerrainGenerator) queueRawEntry(key rawChunkKey, entry *rawChunkEntry) {
	radius := 2
	if key.snapshot != nil {
		radius = min(16, max(radius, key.snapshot.featureRadius))
	}
	row := (int(key.z) - int(g.baseBandZ.Load())) & 31
	entry.seam = entry.chunk == nil && (row < radius || row >= 32-radius)
	entry.lru = g.rawEntryQueue(entry).PushBack(key)
}

// removeRawEntry detaches an entry before consumption, replacement or resource reload.
func (g *TerrainGenerator) removeRawEntry(entry *rawChunkEntry) {
	if entry.lru != nil {
		g.rawEntryQueue(entry).Remove(entry.lru)
		entry.lru = nil
	}
}

// mergeTreeBorders adds neighboring trunks and leaves only where terrain permits.
func mergeTreeBorders(target *section.Chunk, border map[section.BorderPosition]int32) {
	mergeFeatureBorders(target, border, nil)
}

// mergeFeatureBorders transfers accepted neighboring blocks with independent persistent entities.
func mergeFeatureBorders(target *section.Chunk, border map[section.BorderPosition]int32, entities map[section.BorderPosition]*nbt.Compound) {
	baseX, baseZ := int(target.X)*16, int(target.Z)*16
	for position, state := range border {
		if position.X < baseX || position.X >= baseX+16 || position.Z < baseZ || position.Z >= baseZ+16 {
			continue
		}
		current := target.GetBlock(position.X, position.Y, position.Z)
		if block.IsAir(current) || strings.HasSuffix(block.NameFromState(current), "_leaves") && strings.HasSuffix(block.NameFromState(state), "_leaves") {
			target.SetBlock(position.X, position.Y, position.Z, state)
			target.RemoveBlockEntity(int32(position.X), int32(position.Y), int32(position.Z))
			if root := entities[position]; root != nil {
				target.AddBlockEntity(blockentity.NewRawBlockEntity(root))
			}
		}
	}
}

// Bounds returns the current dimension's vertical chunk range.
func (g *TerrainGenerator) Bounds() (int, int, error) {
	snapshot, err := g.snapshot()
	if err != nil {
		return 0, 0, err
	}
	return snapshot.minY, snapshot.height, nil
}

// generateTerrainRange fills terrain and records the surface while each state is already available.
func (g *TerrainGenerator) generateTerrainRange(snapshot *terrainSnapshot, result *section.Chunk, baseX, baseZ, startY, endY int) [16][16]int {
	return g.generateTerrainRangeWithStructures(snapshot, result, baseX, baseZ, startY, endY, structureDensity{})
}

// generateTerrainRangeWithStructures adds terrain adaptation after graph interpolation and before aquifers.
func (g *TerrainGenerator) generateTerrainRangeWithStructures(snapshot *terrainSnapshot, result *section.Chunk, baseX, baseZ, startY, endY int, structures structureDensity) [16][16]int {
	var heights [16][16]int
	for x := range heights {
		for z := range heights[x] {
			heights[x][z] = result.MinY() - 1
		}
	}
	startY, endY = max(startY, result.MinY()), min(endY, result.MaxY()+1)
	// The validated standard Overworld graph is strictly negative from Y=256.
	// Its aquifer is dry there, so upper sections remain air until decoration.
	adapted := len(structures.pieces) != 0 || len(structures.junctions) != 0
	if snapshot.interpolation != nil && g.settingsID == "minecraft:overworld" && !adapted {
		endY = min(endY, 256)
	}
	if startY >= endY {
		return heights
	}
	var evaluation *density.Evaluation
	if cached := snapshot.evaluationPool.Get(); cached != nil {
		evaluation = cached.(*density.Evaluation)
		evaluation.ResetChunk(baseX, startY, baseZ, endY-startY)
	} else {
		evaluation = density.NewChunkEvaluation(baseX, startY, baseZ, endY-startY)
	}
	defer snapshot.evaluationPool.Put(evaluation)
	var aquifer *chunkAquifer
	if cached := snapshot.aquiferPool.Get(); cached != nil {
		aquifer = cached.(*chunkAquifer)
		aquifer.reset()
	} else {
		aquifer = newChunkAquifer(snapshot.aquifer)
	}
	defer snapshot.aquiferPool.Put(aquifer)
	blocks := result.DenseBlocks()
	if blocks == nil {
		if cached := snapshot.blockPool.Get(); cached != nil {
			blocks = cached
			clear(blocks)
		} else {
			blocks = make([]int32, len(result.Sections)*section.BlocksPerSection)
		}
		defer snapshot.blockPool.Put(blocks)
	}
	var oreRandom worldrandom.PositionalScratch
	place := func(position density.Position) {
		densityValue := snapshot.finalDensity.Sample(position)
		if adapted {
			densityValue += structures.sample(position.X, position.Y, position.Z)
		}
		state := aquifer.substance(position, densityValue)
		if state < 0 {
			if ore, ok := snapshot.oreVeins.stateWithScratch(position, &oreRandom); ok {
				state = ore
			} else {
				state = snapshot.blockState
			}
		}
		index := (position.Y-result.MinY())*256 + (position.Z-baseZ)*16 + position.X - baseX
		blocks[index] = state
		if !isAirState(state) {
			x, z := position.X-baseX, position.Z-baseZ
			heights[x][z] = max(heights[x][z], position.Y)
		}
	}
	stone := func(position density.Position, width, height int) {
		for x := position.X - baseX; x < position.X-baseX+width; x++ {
			for z := position.Z - baseZ; z < position.Z-baseZ+width; z++ {
				heights[x][z] = max(heights[x][z], position.Y+height-1)
			}
		}
		for y := position.Y; y < position.Y+height; y++ {
			for z := position.Z; z < position.Z+width; z++ {
				index := (y-result.MinY())*256 + (z-baseZ)*16 + position.X - baseX
				for x := 0; x < width; x++ {
					blocks[index+x] = snapshot.blockState
				}
			}
		}
	}
	interpolation := snapshot.interpolation
	if adapted {
		interpolation = interpolation.WithoutCellElision()
	}
	if !interpolation.FillCells(evaluation, place, stone) {
		for localX := 0; localX < 16; localX++ {
			for localZ := 0; localZ < 16; localZ++ {
				position := density.Position{X: baseX + localX, Z: baseZ + localZ, Evaluation: evaluation}
				for y := startY; y < endY; y++ {
					position.Y = y
					place(position)
				}
			}
		}
	}
	if result.DenseBlocks() == nil {
		for index, current := range result.Sections {
			current.FillBlocks((*[section.BlocksPerSection]int32)(blocks[index*section.BlocksPerSection:]))
		}
	}
	return heights
}

func populateBiomes(result *section.Chunk, source biomeSource, chunkX, chunkZ int32) {
	if result == nil || source == nil {
		return
	}
	baseQuartX, baseQuartZ := int(chunkX)*4, int(chunkZ)*4
	for localX := 0; localX < 4; localX++ {
		for localZ := 0; localZ < 4; localZ++ {
			for _, current := range result.Sections {
				baseQuartY := int(current.Y) * 4
				for localY := 0; localY < 4; localY++ {
					current.SetBiome(localX, localY, localZ, source.sample(baseQuartX+localX, baseQuartY+localY, baseQuartZ+localZ))
				}
			}
		}
	}
}

func (g *TerrainGenerator) snapshot() (*terrainSnapshot, error) {
	if g == nil || g.runtime == nil {
		return nil, fmt.Errorf("generate terrain: nil worldgen runtime")
	}
	resources := g.runtime.current()
	if resources == nil {
		return nil, fmt.Errorf("generate terrain: worldgen resources are not loaded")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.resource == resources && g.compiled != nil {
		return g.compiled, nil
	}
	compiled, err := compileTerrainSnapshot(resources, g.settingsID, g.dimensionTypeID, g.biomeSource, g.seed, g.biomeID)
	if err != nil {
		return nil, err
	}
	g.resource = resources
	compiled.structures = newStructurePlanner(g, compiled)
	g.compiled = compiled
	// Completed entries from an older resource set cannot serve new generation.
	// Running entries retain their publication channels until their callers finish.
	g.rawMu.Lock()
	for key, entry := range g.rawCache {
		if key.snapshot != compiled && entry.ready == nil {
			g.removeRawEntry(entry)
			delete(g.rawCache, key)
			if entry.chunk != nil {
				g.rawTerrains--
			} else {
				g.rawBorders--
			}
		}
	}
	g.rawMu.Unlock()
	g.pruneTerrainBases(compiled)
	return compiled, nil
}

func compileTerrainSnapshot(resources *ResourceSet, settingsID, dimensionTypeID string, biomeDefinition json.RawMessage, seed int64, fallbackBiome int32) (*terrainSnapshot, error) {
	settings, ok := resources.noiseSettings[settingsID]
	if !ok {
		return nil, fmt.Errorf("generate terrain: unknown noise settings %s", settingsID)
	}
	compiler, err := newDensityCompiler(resources, settingsID, seed)
	if err != nil {
		return nil, err
	}
	var router map[string]json.RawMessage
	if err := json.Unmarshal(settings.NoiseRouter, &router); err != nil {
		return nil, fmt.Errorf("generate terrain: decode noise router: %w", err)
	}
	finalDensity, err := compiler.CompileInline(router["final_density"])
	if err != nil {
		return nil, fmt.Errorf("generate terrain: compile final density: %w", err)
	}
	if settingsID == "minecraft:overworld" && resources.standardTerrainSettings(settingsID) {
		finalDensity = compiler.OverworldFillDensity(finalDensity)
	}
	blockState, err := block.BlockStateFromProperties(settings.DefaultBlock.Name, settings.DefaultBlock.Properties)
	if err != nil {
		return nil, fmt.Errorf("generate terrain: default block: %w", err)
	}
	fluidState, err := block.BlockStateFromProperties(settings.DefaultFluid.Name, settings.DefaultFluid.Properties)
	if err != nil {
		return nil, fmt.Errorf("generate terrain: default fluid: %w", err)
	}
	lavaState, err := block.DefaultBlockState("minecraft:lava")
	if err != nil {
		return nil, fmt.Errorf("generate terrain: lava state: %w", err)
	}
	compileRouter := func(name string) (density.Function, error) {
		function, err := compiler.CompileInline(router[name])
		if err != nil {
			return nil, fmt.Errorf("generate terrain: compile router %s: %w", name, err)
		}
		return function, nil
	}
	barrier, err := compileRouter("barrier")
	if err != nil {
		return nil, err
	}
	floodedness, err := compileRouter("fluid_level_floodedness")
	if err != nil {
		return nil, err
	}
	spread, err := compileRouter("fluid_level_spread")
	if err != nil {
		return nil, err
	}
	lavaNoise, err := compileRouter("lava")
	if err != nil {
		return nil, err
	}
	erosion, err := compileRouter("erosion")
	if err != nil {
		return nil, err
	}
	depth, err := compileRouter("depth")
	if err != nil {
		return nil, err
	}
	initialDensity, err := compileRouter("initial_density_without_jaggedness")
	if err != nil {
		return nil, err
	}
	veinToggle, err := compileRouter("vein_toggle")
	if err != nil {
		return nil, err
	}
	veinRidged, err := compileRouter("vein_ridged")
	if err != nil {
		return nil, err
	}
	veinGap, err := compileRouter("vein_gap")
	if err != nil {
		return nil, err
	}
	biomes, err := compileBiomeSource(resources, biomeDefinition, router, compiler, fallbackBiome)
	if err != nil {
		return nil, fmt.Errorf("generate terrain: %w", err)
	}
	featureResources, err := resources.features.forBiomeOrder(possibleBiomeNames(biomes))
	if err != nil {
		return nil, fmt.Errorf("generate terrain: %w", err)
	}
	surface, err := compileSurfaceSystem(resources, settings, compiler, seed, blockState.ID(), biomes, initialDensity)
	if err != nil {
		return nil, fmt.Errorf("generate terrain: %w", err)
	}
	bounds := dimensionBounds(resources, dimensionTypeID, settings)
	aquiferConfiguration := aquiferConfig{
		enabled: settings.AquifersEnabled, seed: seed, legacy: settings.LegacyRandomSource,
		minY: settings.Noise.MinY, height: settings.Noise.Height, seaLevel: settings.SeaLevel,
		defaultFluid: fluidState.ID(), lava: lavaState.ID(),
		barrier: barrier, floodedness: floodedness, spread: spread, lavaNoise: lavaNoise,
		erosion: erosion, depth: depth, initialDensity: initialDensity,
		preliminarySurface: surface.preliminarySurfaceWithEvaluation,
	}
	var interpolation *density.ChunkInterpolation
	if resources.standardTerrainSettings(settingsID) {
		interpolation = compiler.ChunkInterpolation()
		if settingsID == "minecraft:overworld" {
			interpolation.SetDryCeiling(finalDensity, max(-54, settings.SeaLevel)+5)
		}
	}
	featureRadius := 1
	farFeatureBiomes := make(map[string]int)
	farUnfiltered := 0
	for _, step := range featureResources.steps {
		for _, id := range step.ids {
			if placed := featureResources.placed[id]; placed != nil {
				if radius := placed.writeRadius(); radius > 1 {
					featureRadius = max(featureRadius, radius)
					if !placed.hasPlacement("minecraft:biome") {
						farUnfiltered = max(farUnfiltered, radius)
					}
					for biome, biomeSteps := range featureResources.biomeFeatures {
						for _, featureIDs := range biomeSteps {
							for _, featureID := range featureIDs {
								if featureID == id {
									farFeatureBiomes[biome] = max(farFeatureBiomes[biome], radius)
								}
							}
						}
					}
				}
			}
		}
	}
	return &terrainSnapshot{
		resources: resources, settings: settings, finalDensity: finalDensity,
		interpolation: interpolation,
		blockPool:     newTerrainBufferPool[int32](16), priorityPool: newTerrainBufferPool[uint32](16),
		minY: bounds.minY, height: bounds.height,
		blockState: blockState.ID(), fluidState: fluidState.ID(), biomes: biomes, surface: surface,
		oreVeins: newOreVeinifier(settings.OreVeinsEnabled, seed, settings.LegacyRandomSource, veinToggle, veinRidged, veinGap),
		aquifer:  aquiferConfiguration,
		carvers:  newCarverSystem(resources, settings, seed, biomes, aquiferConfiguration, surface),
		features: newFeatureSystem(featureResources, seed, settings.Noise.MinY, settings.Noise.Height, settings.SeaLevel, biomes), featureRadius: featureRadius, farFeatureBiomes: farFeatureBiomes, farUnfiltered: farUnfiltered,
	}, nil
}

type verticalBounds struct{ minY, height int }

func dimensionBounds(resources *ResourceSet, dimensionTypeID string, settings NoiseSettings) verticalBounds {
	if dimensionType, ok := resources.dimensionTypes[dimensionTypeID]; ok {
		return verticalBounds{minY: dimensionType.MinY, height: dimensionType.Height}
	}
	return verticalBounds{minY: settings.Noise.MinY, height: settings.Noise.Height}
}
