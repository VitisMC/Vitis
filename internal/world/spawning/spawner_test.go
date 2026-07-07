package spawning

import (
	"reflect"
	"testing"

	genbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/data/generated/block"
	"github.com/vitismc/vitis/internal/entity"
)

type fakeWorld struct {
	surfaceY   int
	surface    int32
	biomeID    int32
	blockLight uint8
	skyLight   uint8
	loaded     bool
	minY       int
	maxY       int
	timeOfDay  int64
}

func newFakeWorld() *fakeWorld {
	return &fakeWorld{
		surfaceY:   63,
		surface:    block.GrassBlockDefaultState,
		biomeID:    genbiome.BiomePlains,
		blockLight: 0,
		skyLight:   15,
		loaded:     true,
		minY:       -64,
		maxY:       80,
		timeOfDay:  18000,
	}
}

func (w *fakeWorld) GetBlock(_, y, _ int) int32 {
	if y == w.surfaceY {
		return w.surface
	}
	return 0
}

func (w *fakeWorld) GetBiome(_, _, _ int) int32      { return w.biomeID }
func (w *fakeWorld) GetBlockLight(_, _, _ int) uint8 { return w.blockLight }
func (w *fakeWorld) GetSkyLight(_, _, _ int) uint8   { return w.skyLight }
func (w *fakeWorld) IsChunkLoaded(_, _ int32) bool   { return w.loaded }
func (w *fakeWorld) MinY() int                       { return w.minY }
func (w *fakeWorld) MaxY() int                       { return w.maxY }
func (w *fakeWorld) TimeOfDay() int64                { return w.timeOfDay }

func TestSpawnerIntervalGating(t *testing.T) {
	spawner := NewSpawnerWithConfig(testConfig(2, []SpawnEntry{pigEntry()}))
	mobs := newMobManager()
	players := []PlayerInfo{{X: 0, Y: 64, Z: 0}}
	world := newFakeWorld()

	if got := spawner.Tick(mobs, players, world); len(got.Requests) != 0 {
		t.Fatalf("first tick requests = %d, want 0", len(got.Requests))
	}

	if got := spawner.Tick(mobs, players, world); len(got.Requests) == 0 {
		t.Fatalf("second tick produced no spawn requests")
	}
}

func TestSpawnerRejectsMissingInputsAndUnloadedChunks(t *testing.T) {
	spawner := NewSpawnerWithConfig(testConfig(1, []SpawnEntry{pigEntry()}))
	world := newFakeWorld()

	if got := spawner.Tick(newMobManager(), nil, world); len(got.Requests) != 0 || len(got.DespawnIDs) != 0 {
		t.Fatalf("empty players result = %+v, want empty", got)
	}
	if got := spawner.Tick(nil, []PlayerInfo{{X: 0, Y: 64, Z: 0}}, world); len(got.Requests) != 0 || len(got.DespawnIDs) != 0 {
		t.Fatalf("nil mobs result = %+v, want empty", got)
	}

	world.loaded = false
	if got := spawner.Tick(newMobManager(), []PlayerInfo{{X: 0, Y: 64, Z: 0}}, world); len(got.Requests) != 0 {
		t.Fatalf("unloaded world requests = %d, want 0", len(got.Requests))
	}
}

func TestSpawnerHonorsMobCaps(t *testing.T) {
	spawner := NewSpawnerWithConfig(Config{
		Seed:                1,
		IntervalTicks:       1,
		AttemptsPerCategory: 8,
		MinSpawnDistance:    24,
		MaxSpawnDistance:    32,
		MobCaps:             map[entity.MobCategory]int{entity.MobCategoryHostile: 1},
		SpawnTable:          []SpawnEntry{zombieEntry()},
	})
	mobs := newMobManager()
	if mob := mobs.SpawnMob("zombie", entity.Vec3{X: 0, Y: 64, Z: 0}); mob == nil {
		t.Fatalf("failed to create existing zombie")
	}

	result := spawner.Tick(mobs, []PlayerInfo{{X: 0, Y: 64, Z: 0}}, newFakeWorld())
	if len(result.Requests) != 0 {
		t.Fatalf("requests with full cap = %d, want 0", len(result.Requests))
	}
}

func TestSpawnerDeterministicWithFixedSeed(t *testing.T) {
	config := testConfig(1, []SpawnEntry{pigEntry()})
	a := NewSpawnerWithConfig(config)
	b := NewSpawnerWithConfig(config)
	players := []PlayerInfo{{X: 0, Y: 64, Z: 0}}
	world := newFakeWorld()

	gotA := a.Tick(newMobManager(), players, world)
	gotB := b.Tick(newMobManager(), players, world)

	if !reflect.DeepEqual(gotA.Requests, gotB.Requests) {
		t.Fatalf("requests differ for fixed seed:\n%+v\n%+v", gotA.Requests, gotB.Requests)
	}
}

func TestSpawnerRejectsHostileWhenBlockLightPresent(t *testing.T) {
	spawner := NewSpawnerWithConfig(testConfig(1, []SpawnEntry{zombieEntry()}))
	world := newFakeWorld()
	world.blockLight = 1
	world.skyLight = 0

	result := spawner.Tick(newMobManager(), []PlayerInfo{{X: 0, Y: 64, Z: 0}}, world)
	if len(result.Requests) != 0 {
		t.Fatalf("hostile requests with block light = %d, want 0", len(result.Requests))
	}
}

func TestSpawnerRejectsPassiveOnWrongSurfaceAndBiome(t *testing.T) {
	spawner := NewSpawnerWithConfig(testConfig(1, []SpawnEntry{pigEntry()}))
	players := []PlayerInfo{{X: 0, Y: 64, Z: 0}}

	stoneWorld := newFakeWorld()
	stoneWorld.surface = block.StoneDefaultState
	if result := spawner.Tick(newMobManager(), players, stoneWorld); len(result.Requests) != 0 {
		t.Fatalf("passive requests on stone = %d, want 0", len(result.Requests))
	}

	oceanWorld := newFakeWorld()
	oceanWorld.biomeID = genbiome.BiomeOcean
	if result := NewSpawnerWithConfig(testConfig(1, []SpawnEntry{pigEntry()})).Tick(newMobManager(), players, oceanWorld); len(result.Requests) != 0 {
		t.Fatalf("passive requests in ocean biome = %d, want 0", len(result.Requests))
	}
}

func TestSpawnerDistanceLimits(t *testing.T) {
	spawner := NewSpawnerWithConfig(testConfig(1, []SpawnEntry{pigEntry()}))
	world := newFakeWorld()
	entry := pigEntry()
	player := PlayerInfo{X: 0, Y: 64, Z: 0}

	if spawner.canSpawnAt(entry, 10, 64, 0, player, world) {
		t.Fatalf("spawn inside minimum radius was accepted")
	}
	if !spawner.canSpawnAt(entry, 25, 64, 0, player, world) {
		t.Fatalf("spawn inside allowed radius was rejected")
	}
	if spawner.canSpawnAt(entry, 129, 64, 0, player, world) {
		t.Fatalf("spawn outside maximum radius was accepted")
	}
}

func TestSpawnerDespawnsByDistance(t *testing.T) {
	spawner := NewSpawnerWithConfig(Config{
		Seed:                  1,
		IntervalTicks:         100,
		RandomDespawnDistance: 32,
		HardDespawnDistance:   128,
		RandomDespawnChance:   1,
		SpawnTable:            []SpawnEntry{zombieEntry()},
	})
	mobs := newMobManager()
	hard := mobs.SpawnMob("zombie", entity.Vec3{X: 200, Y: 64, Z: 0})
	random := mobs.SpawnMob("zombie", entity.Vec3{X: 40, Y: 64, Z: 0})

	result := spawner.Tick(mobs, []PlayerInfo{{X: 0, Y: 64, Z: 0}}, newFakeWorld())
	if !containsID(result.DespawnIDs, hard.ID()) {
		t.Fatalf("hard despawn IDs = %v, missing %d", result.DespawnIDs, hard.ID())
	}
	if !containsID(result.DespawnIDs, random.ID()) {
		t.Fatalf("random despawn IDs = %v, missing %d", result.DespawnIDs, random.ID())
	}
}

func testConfig(interval int, table []SpawnEntry) Config {
	return Config{
		Seed:                7,
		IntervalTicks:       interval,
		AttemptsPerCategory: 8,
		MinSpawnDistance:    24,
		MaxSpawnDistance:    32,
		MobCaps: map[entity.MobCategory]int{
			entity.MobCategoryHostile: 8,
			entity.MobCategoryPassive: 8,
			entity.MobCategoryAmbient: 8,
		},
		SpawnTable: table,
	}
}

func pigEntry() SpawnEntry {
	return SpawnEntry{
		TypeName:        "pig",
		Weight:          1,
		MinGroup:        1,
		MaxGroup:        1,
		Category:        entity.MobCategoryPassive,
		BiomeCategories: landBiomeCategories(),
	}
}

func zombieEntry() SpawnEntry {
	return SpawnEntry{
		TypeName: "zombie",
		Weight:   1,
		MinGroup: 1,
		MaxGroup: 1,
		Category: entity.MobCategoryHostile,
	}
}

func newMobManager() *entity.MobManager {
	manager := entity.NewManager()
	return entity.NewMobManager(manager, manager.AllocateID)
}

func containsID(ids []int32, id int32) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}
