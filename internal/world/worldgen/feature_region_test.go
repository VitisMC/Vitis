package worldgen

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vitismc/vitis/internal/block"
	blockentity "github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	"github.com/vitismc/vitis/internal/world/worldgen/density"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// regionTestDensity supplies isolated terrain shapes without involving the full noise graph.
type regionTestDensity func(density.Position) float64

// Sample evaluates the test terrain at one world-coordinate position.
func (f regionTestDensity) Sample(p density.Position) float64 { return f(p) }

// regionTestGenerator prepares a bounded flat terrain stage with no decorations or recursion.
func regionTestGenerator(state int32, function regionTestDensity) (*TerrainGenerator, *terrainSnapshot) {
	g := &TerrainGenerator{settingsID: "test:flat"}
	snapshot := &terrainSnapshot{resources: &ResourceSet{}, settings: NoiseSettings{Noise: NoiseShape{MinY: 0, Height: 64, SizeHorizontal: 1, SizeVertical: 1}}, minY: 0, height: 64, blockState: state, finalDensity: function, biomes: fixedBiomeSource{0}}
	return g, snapshot
}

// TestMonsterRoomCrossesChunkBoundary checks neighboring ceiling/floor support and room carving.
func TestMonsterRoomCrossesChunkBoundary(t *testing.T) {
	feature := loadSmallFeature(t, "monster_room").(*monsterRoomFeature)
	stone := block.DefaultStateID("minecraft:stone")
	g, snapshot := regionTestGenerator(stone, func(density.Position) float64 { return 1 })
	source := regionTestWorkspace(t, g, snapshot, -1, -2)
	random := worldrandom.NewLegacy(42)
	radius := int(random.NextIntBound(2)) + 2
	source.SetFeatureBlock(-1+radius+1, 30, -24, feature.air)
	source.SetFeatureBlock(-1+radius+1, 31, -24, feature.air)
	source.SetFeatureStep(3, 0)
	if !feature.placeFeature(nil, source, worldrandom.NewLegacy(42), blockPosition{-1, 30, -24}) {
		t.Fatal("room rejected neighboring floor or ceiling")
	}
	entry := &rawChunkEntry{writes: source.FeatureBorderWrites(), entities: make(map[section.BorderPosition]*nbt.Compound)}
	for p, entity := range source.BorderBlockEntities() {
		entry.entities[p] = entity.(*blockentity.RawBlockEntity).PersistentNBT()
	}
	target := regionTestWorkspace(t, g, snapshot, 0, -2)
	mergeRegionBorders(target, -1, -2, entry)
	if !block.IsAir(target.GetBlock(0, 31, -24)) || target.GetBlock(0, 34, -24) != stone {
		t.Fatal("room interior or preserved roof differs across the border")
	}
	floor := target.GetBlock(0, 29, -24)
	if floor != feature.cobble && floor != feature.mossy {
		t.Fatal("neighbor room floor was clipped")
	}
	for p, root := range entry.entities {
		actual := target.GetBlockEntity(int32(p.X), int32(p.Y), int32(p.Z))
		if actual == nil {
			t.Fatal("neighbor room loot entity lost")
		}
		root.PutString("mutated", "yes")
		if _, _, found := actual.(*blockentity.RawBlockEntity).PersistentNBT().Get("mutated"); found {
			t.Fatal("neighbor room NBT aliased")
		}
	}
}

// TestOreRegionMasksPreserveTranslatedGeometryAndRandomState exercises veins wider than one chunk.
func TestOreRegionMasksPreserveTranslatedGeometryAndRandomState(t *testing.T) {
	feature := loadSmallFeature(t, "ore_diamond_large").(*configuredOre)
	feature.size = 64
	stone := block.DefaultStateID("minecraft:stone")
	g, snapshot := regionTestGenerator(stone, func(density.Position) float64 { return 1 })
	a := regionTestWorkspace(t, g, snapshot, 0, 0)
	b := regionTestWorkspace(t, g, snapshot, -1, -2)
	a.SetFeatureStep(6, 0)
	b.SetFeatureStep(6, 0)
	ra, rb := worldrandom.NewLegacy(123), worldrandom.NewLegacy(123)
	if !feature.place(a, ra, blockPosition{15, 30, 8}) || !feature.place(b, rb, blockPosition{-1, 30, -24}) {
		t.Fatal("large ore vein not placed")
	}
	if ra.NextLong() != rb.NextLong() {
		t.Fatal("translated ore changed RNG sequence")
	}
	for x := 0; x < 32; x++ {
		for z := -8; z < 25; z++ {
			for y := 18; y < 41; y++ {
				if getChunkBlock(a, blockPosition{x, y, z}) != getChunkBlock(b, blockPosition{x - 16, y, z - 32}) {
					t.Fatalf("translated ore geometry differs at (%d,%d,%d)", x, y, z)
				}
			}
		}
	}
	target := regionTestWorkspace(t, g, snapshot, 1, 0)
	mergeRegionBorders(target, 0, 0, &rawChunkEntry{writes: a.FeatureBorderWrites()})
	count := 0
	a.FeatureBorderWrites().ForChunk(1, 0, func(p section.BorderPosition, write section.FeatureBorderWrite) {
		count++
		if target.GetBlock(p.X, p.Y, p.Z) != write.State || write.State == stone {
			t.Fatal("ore border replacement lost")
		}
	})
	if count == 0 {
		t.Fatal("fixture did not exercise neighboring ore writes")
	}
}

// TestVegetationPatchCrossesChunkBoundary checks adjacent cave ground and canonical replacement.
func TestVegetationPatchCrossesChunkBoundary(t *testing.T) {
	feature := loadSmallFeature(t, "moss_patch").(*vegetationPatchFeature)
	feature.vegetationChance = 0
	g, snapshot := regionTestGenerator(block.DefaultStateID("minecraft:stone"), func(p density.Position) float64 {
		if p.Y <= 15 {
			return 1
		}
		return -1
	})
	source := regionTestWorkspace(t, g, snapshot, -1, -2)
	source.SetFeatureStep(8, 0)
	if !feature.placeFeature(&featureSystem{}, source, worldrandom.NewLegacy(4), blockPosition{-1, 16, -24}) {
		t.Fatal("crossing moss patch rejected")
	}
	target := regionTestWorkspace(t, g, snapshot, 0, -2)
	mergeRegionBorders(target, -1, -2, &rawChunkEntry{writes: source.FeatureBorderWrites()})
	count := 0
	for x := 0; x < 16; x++ {
		for z := -32; z < -16; z++ {
			if block.NameFromState(target.GetBlock(x, 15, z)) == "minecraft:moss_block" {
				count++
			}
		}
	}
	if count < 2 {
		t.Fatal("moss ground was clipped at the border")
	}
}

// TestFeatureWriteOrderRejectsUnrepresentableStages prevents collisions with imported write stamps.
func TestFeatureWriteOrderRejectsUnrepresentableStages(t *testing.T) {
	for _, count := range []int{-1, (1 << 15) + 1} {
		if _, err := sortFeatureSteps(nil, nil, count); err == nil {
			t.Fatal("unrepresentable feature order accepted")
		}
	}
}

// TestTerrainBaseHeightmapsAreConcurrentAndImmutable compares all cached predicates with direct scans.
func TestTerrainBaseHeightmapsAreConcurrentAndImmutable(t *testing.T) {
	c := section.NewChunkWithBounds(-2, 3, -64, 128, 0)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			if x == 0 && z == 0 {
				continue
			}
			c.SetBlock(x, -40+x+z, z, block.DefaultStateID("minecraft:stone"))
			c.SetBlock(x, -39+x+z, z, block.DefaultStateID("minecraft:water"))
			c.SetBlock(x, -38+x+z, z, block.DefaultStateID("minecraft:oak_leaves"))
		}
	}
	e := &baseChunkEntry{chunk: c}
	var wg sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for kind := section.HeightmapKind(0); kind < 4; kind++ {
				for x := 0; x < 16; x++ {
					for z := 0; z < 16; z++ {
						want, found := c.Heightmap(x, z, kind)
						got, actual := e.height(-32+x, 48+z, kind)
						if got != want || actual != found {
							t.Errorf("height (%d,%d,%d) = %d/%v, want %d/%v", x, z, kind, got, actual, want, found)
						}
					}
				}
			}
		}()
	}
	wg.Wait()
	if value, found := e.height(0, 0, 255); value != -64 || found {
		t.Fatal("invalid predicate accepted")
	}
}

// regionTestWorkspace clones a terrain stage and attaches neighboring readers for feature placement.
func regionTestWorkspace(t *testing.T, g *TerrainGenerator, snapshot *terrainSnapshot, x, z int32) *section.Chunk {
	t.Helper()
	base := g.terrainBase(snapshot, x, z, nil)
	if base.err != nil {
		t.Fatal(base.err)
	}
	chunk, err := cloneTerrainBase(base.chunk)
	if err != nil {
		t.Fatal(err)
	}
	region := &featureRegion{generator: g, snapshot: snapshot, sourceX: x, sourceZ: z}
	if err := chunk.UseFeatureRegion(region.block, region.height, make([]uint32, len(chunk.Sections)*section.BlocksPerSection)); err != nil {
		t.Fatal(err)
	}
	return chunk
}

// TestTerrainBaseCoalescesConcurrentRequests verifies single generation and independent mutable clones.
func TestTerrainBaseCoalescesConcurrentRequests(t *testing.T) {
	var samples atomic.Int64
	g, snapshot := regionTestGenerator(block.DefaultStateID("minecraft:stone"), func(p density.Position) float64 {
		samples.Add(1)
		if p.Y < 16 {
			return 1
		}
		return -1
	})
	g.EnableDiagnostics()
	var wg sync.WaitGroup
	results := make(chan *baseChunkEntry, 16)
	for range 16 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- g.terrainBase(snapshot, -2, 3, nil) }()
	}
	wg.Wait()
	close(results)
	var first *baseChunkEntry
	for entry := range results {
		if entry.err != nil {
			t.Fatal(entry.err)
		}
		if first == nil {
			first = entry
		}
		if entry != first {
			t.Fatal("duplicate terrain stage published")
		}
	}
	if samples.Load() != 16*16*64 {
		t.Fatalf("terrain samples = %d", samples.Load())
	}
	a, err := cloneTerrainBase(first.chunk)
	if err != nil {
		t.Fatal(err)
	}
	b, err := cloneTerrainBase(first.chunk)
	if err != nil {
		t.Fatal(err)
	}
	a.SetBlock(-32, 15, 48, 0)
	if block.IsAir(b.GetBlock(-32, 15, 48)) || block.IsAir(first.chunk.GetBlock(-32, 15, 48)) {
		t.Fatal("decoration clone changed shared terrain")
	}
	if stats := g.Diagnostics(); stats.BaseGenerations != 1 || stats.BaseCacheHits != 15 || stats.RepeatedBaseGenerations != 0 {
		t.Fatalf("incorrect base counters %+v", stats)
	}
}

// TestTerrainBaseEvictionAndReload keeps running work alive while retiring old completed snapshots.
func TestTerrainBaseEvictionAndReload(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	g, snapshot := regionTestGenerator(1, func(p density.Position) float64 {
		if p.X == 0 {
			once.Do(func() { close(started); <-release })
		}
		return 1
	})
	done := make(chan *baseChunkEntry, 1)
	go func() { done <- g.terrainBase(snapshot, 0, 0, nil) }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("terrain request did not start")
	}
	for x := int32(1); x <= terrainBaseCacheLimit+4; x++ {
		if entry := g.terrainBase(snapshot, x, 0, nil); entry.err != nil {
			t.Fatal(entry.err)
		}
	}
	g.baseMu.Lock()
	count := len(g.baseCache)
	pinned := g.baseCache[rawChunkKey{snapshot: snapshot}] != nil
	g.baseMu.Unlock()
	if count != terrainBaseCacheLimit+1 || !pinned {
		close(release)
		t.Fatal("running request evicted or completed cache unbounded")
	}
	fresh := &terrainSnapshot{}
	g.pruneTerrainBases(fresh)
	close(release)
	select {
	case entry := <-done:
		if entry.err != nil {
			t.Fatal(entry.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("old request lost its completion channel")
	}
	g.baseMu.Lock()
	remaining := len(g.baseCache)
	g.baseMu.Unlock()
	if remaining != 0 {
		t.Fatal("old terrain published into the new snapshot cache")
	}
}

// TestTerrainBaseSeamEviction retains distant band-edge terrain while bounding completed entries.
func TestTerrainBaseSeamEviction(t *testing.T) {
	g := &TerrainGenerator{baseCache: make(map[rawChunkKey]*baseChunkEntry)}
	g.baseBandZ.Store(-625)
	snapshot := &terrainSnapshot{}
	seamKey := rawChunkKey{snapshot: snapshot, x: 0, z: -595}
	if !g.isBaseSeam(seamKey.z) || g.isBaseSeam(-609) {
		t.Fatal("incorrect band-edge classification")
	}
	seam := &baseChunkEntry{seam: true}
	seam.lru = g.baseSeamLRU.PushBack(seamKey)
	g.baseCache[seamKey] = seam
	for x := int32(1); x <= terrainBaseCacheLimit+1; x++ {
		key := rawChunkKey{snapshot: snapshot, x: x, z: -609}
		entry := &baseChunkEntry{}
		entry.lru = g.baseLRU.PushBack(key)
		g.baseCache[key] = entry
		g.evictTerrainBases()
	}
	if len(g.baseCache) != terrainBaseCacheLimit || g.baseCache[seamKey] != seam {
		t.Fatal("band-edge entry was lost or cache exceeded its limit")
	}
	g.pruneTerrainBases(&terrainSnapshot{})
	if len(g.baseCache) != 0 || g.baseLRU.Len() != 0 || g.baseSeamLRU.Len() != 0 {
		t.Fatal("snapshot reload retained stale terrain")
	}
}

// TestTerrainBaseCloneKeepsStructuresAndEntities preserves persistent starts, loot, and entity ownership.
func TestTerrainBaseCloneKeepsStructuresAndEntities(t *testing.T) {
	base := section.NewChunkWithBounds(-1, 0, 0, 64, 0)
	base.SetBlock(-8, 20, 8, block.DefaultStateID("minecraft:chest"))
	root := featureBlockEntityNBT("minecraft:chest", blockPosition{-8, 20, 8}).PutString("LootTable", "minecraft:chests/buried_treasure")
	base.AddBlockEntity(blockentity.NewRawBlockEntity(root))
	base.SetPersistentNBT(nbt.NewCompound().PutCompound("structures", nbt.NewCompound().PutCompound("starts", nbt.NewCompound().PutString("test", "kept"))))
	clone, err := cloneTerrainBase(base)
	if err != nil {
		t.Fatal(err)
	}
	if clone.GetBlockEntity(-8, 20, 8) == base.GetBlockEntity(-8, 20, 8) || clone.PersistentNBT() == nil {
		t.Fatal("base entity or structure state lost or aliased")
	}
	clone.RemoveBlockEntity(-8, 20, 8)
	if base.GetBlockEntity(-8, 20, 8) == nil {
		t.Fatal("clone removal changed cached terrain")
	}
}

// TestFeatureRegionPreparationSkipsActiveTerrain proves another worker can help before waiting.
func TestFeatureRegionPreparationSkipsActiveTerrain(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	g, snapshot := regionTestGenerator(block.DefaultStateID("minecraft:stone"), func(p density.Position) float64 {
		if p.X == 0 && p.Z == 0 {
			once.Do(func() { close(started); <-release })
		}
		return 1
	})
	done := make(chan *baseChunkEntry, 1)
	go func() { done <- g.terrainBase(snapshot, 0, 0, nil) }()
	<-started
	region := &featureRegion{generator: g, snapshot: snapshot, sourceX: 1, sourceZ: 0}
	prepared := make(chan struct{})
	go func() { region.prepare(); close(prepared) }()
	select {
	case <-prepared:
	case <-time.After(5 * time.Second):
		close(release)
		<-done
		<-prepared
		t.Fatal("worker waited instead of preparing unclaimed terrain")
	}
	if region.neighbors[1] != nil || region.neighbors[8] == nil {
		close(release)
		<-done
		t.Fatal("active terrain was not deferred or remaining terrain was skipped")
	}
	close(release)
	entry := <-done
	if region.neighbor(0, 0) != entry || region.err != nil {
		t.Fatal("deferred dependency was regenerated or lost")
	}
}

// TestDesertWellCrossesChunkBoundary validates support, replacing sand and transferring archaeology NBT.
func TestDesertWellCrossesChunkBoundary(t *testing.T) {
	feature := loadSmallFeature(t, "desert_well").(*desertWellFeature)
	g, snapshot := regionTestGenerator(feature.sand, func(p density.Position) float64 {
		if p.Y <= 15 {
			return 1
		}
		return -1
	})
	source := regionTestWorkspace(t, g, snapshot, -1, -2)
	source.SetFeatureStep(4, 0)
	if !feature.placeFeature(nil, source, worldrandom.NewLegacy(6), blockPosition{-1, 15, -24}) {
		t.Fatal("well rejected valid neighboring sand support")
	}
	target := regionTestWorkspace(t, g, snapshot, 0, -2)
	entry := &rawChunkEntry{writes: source.FeatureBorderWrites(), entities: make(map[section.BorderPosition]*nbt.Compound)}
	for p, entity := range source.BorderBlockEntities() {
		entry.entities[p] = entity.(*blockentity.RawBlockEntity).PersistentNBT()
	}
	if len(entry.entities) == 0 {
		t.Fatal("fixture did not generate neighboring archaeology entities")
	}
	mergeRegionBorders(target, -1, -2, entry)
	if target.GetBlock(1, 13, -24) != feature.stone || target.GetBlock(0, 15, -24) != feature.water {
		t.Fatal("well foundation or water clipped at the chunk boundary")
	}
	for p, root := range entry.entities {
		if p.X >= 0 && p.X < 16 {
			if target.GetBlockEntity(int32(p.X), int32(p.Y), int32(p.Z)) == nil {
				t.Fatal("neighbor archaeology entity lost")
			}
			root.PutString("mutated", "yes")
			if _, _, present := smallFeatureEntityNBT(t, target, blockPosition{p.X, p.Y, p.Z}).Get("mutated"); present {
				t.Fatal("shared neighbor archaeology NBT")
			}
		}
	}
}

// TestFossilCrossesChunkBoundary keeps the full skeleton footprint and reads neighboring burial heights.
func TestFossilCrossesChunkBoundary(t *testing.T) {
	feature := fossilTestFeature(t, true)
	g, snapshot := regionTestGenerator(block.DefaultStateID("minecraft:stone"), func(p density.Position) float64 {
		if p.Y <= 15 {
			return 1
		}
		return -1
	})
	source := regionTestWorkspace(t, g, snapshot, 0, 0)
	source.SetFeatureStep(3, 0)
	if !feature.placeFeature(nil, source, worldrandom.NewLegacy(0), blockPosition{15, 20, 8}) {
		t.Fatal("fossil rejected solid neighboring corners")
	}
	target := regionTestWorkspace(t, g, snapshot, 1, 0)
	entry := &rawChunkEntry{writes: source.FeatureBorderWrites()}
	mergeRegionBorders(target, 0, 0, entry)
	count := 0
	for p, state := range source.BorderBlocks() {
		if p.X >= 16 && p.X < 32 {
			count++
			if target.GetBlock(p.X, p.Y, p.Z) != state {
				t.Fatal("neighbor fossil block lost")
			}
		}
	}
	if count == 0 {
		t.Fatal("fixture did not exercise a crossing fossil")
	}
}

// regionReadFeature verifies that reading dependencies never recursively runs another feature pass.
type regionReadFeature struct {
	calls *atomic.Int64
	state int32
}

// placeFeature reads a neighboring terrain block and records one owned feature block.
func (f regionReadFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, _ worldrandom.Source, origin blockPosition) bool {
	f.calls.Add(1)
	if getChunkBlock(chunk, blockPosition{origin.x + 16, 1, origin.z}) != f.state {
		return false
	}
	setChunkBlock(chunk, blockPosition{origin.x, 16, origin.z}, f.state)
	return true
}

// TestFeatureRegionDoesNotRecurseIntoDecorations prevents mutual worker dependency cycles.
func TestFeatureRegionDoesNotRecurseIntoDecorations(t *testing.T) {
	var calls atomic.Int64
	stone := block.DefaultStateID("minecraft:stone")
	g, snapshot := regionTestGenerator(stone, func(p density.Position) float64 {
		if p.Y < 16 {
			return 1
		}
		return -1
	})
	snapshot.features = &featureSystem{resources: &featureSystemResources{placed: map[string]*placedFeature{"test:reader": {feature: regionReadFeature{&calls, stone}}}, steps: []featureStep{{ids: []string{"test:reader"}, allBiomeFiltered: true}}}}
	result, err := g.generateRaw(snapshot, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || result.GetBlock(0, 16, 0) != stone {
		t.Fatal("neighbor read ran another decoration task or failed to resolve terrain")
	}
}
