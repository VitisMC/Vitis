package spawning

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/entity"
)

// spawnTestEnvironment provides resident geometry and controllable environmental state.
type spawnTestEnvironment struct {
	settings                    Settings
	biome                       string
	blockLight, sky, brightness uint8
	active                      bool
	substrate                   int32
}

// GetBlock supplies a solid floor and air above it.
func (e *spawnTestEnvironment) GetBlock(x, y, z int) int32 { return e.GetBlockStateAt(x, y, z) }

// GetBlockStateAt supplies the same resident geometry to collision checks.
func (e *spawnTestEnvironment) GetBlockStateAt(x, y, z int) int32 {
	if y < 64 {
		return e.substrate
	}
	return 0
}

// IsEntityTicking controls whether spawn and despawn may change a position.
func (e *spawnTestEnvironment) IsEntityTicking(int32, int32) bool { return e.active }

// SpawnSettings exposes the test's authoritative gamerules and dimension.
func (e *spawnTestEnvironment) SpawnSettings() Settings { return e.settings }

// SpawnBiome supplies the test biome at all resident positions.
func (e *spawnTestEnvironment) SpawnBiome(int, int, int) string { return e.biome }

// SpawnLight supplies separately controlled block, sky and adjusted light.
func (e *spawnTestEnvironment) SpawnLight(int, int, int) (uint8, uint8, uint8, bool) {
	return e.blockLight, e.sky, e.brightness, e.active
}

// SpawnChunks returns one complete vanilla local-cap neighborhood.
func (e *spawnTestEnvironment) SpawnChunks([]PlayerPos) []ChunkPos {
	if !e.active {
		return nil
	}
	var chunks []ChunkPos
	for x := int32(-8); x <= 8; x++ {
		for z := int32(-8); z <= 8; z++ {
			chunks = append(chunks, ChunkPos{x, z})
		}
	}
	return chunks
}

// spawningTestRuntime loads real biome and dimension definitions.
func spawningTestRuntime(t *testing.T) *Runtime {
	t.Helper()
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	return runtime
}

// TestBiomeDefinitionsSeparateSpawnAndBehaviorCategories verifies snapshot-based species selection.
func TestBiomeDefinitionsSeparateSpawnAndBehaviorCategories(t *testing.T) {
	definitions := spawningTestRuntime(t).current()
	if len(definitions.biomes) != 65 || definitions.categories["spider"] != Monster || definitions.categories["enderman"] != Monster || definitions.categories["wolf"] != Creature {
		t.Fatal("incorrect biome or category definitions")
	}
	if hasEntry(definitions.biomes["minecraft:the_end"][Monster], "zombie") || !hasEntry(definitions.biomes["minecraft:the_end"][Monster], "enderman") || hasEntry(definitions.biomes["minecraft:nether_wastes"][Creature], "cow") {
		t.Fatal("dimension biome used the overworld spawn list")
	}
	if rules := definitions.dimensions["minecraft:the_nether"]; rules.MinY != 0 || rules.Height != 256 || rules.SkyLight {
		t.Fatal("incorrect nether placement bounds")
	}
}

// TestSpawnPlacementChecksShapesLightAndSubstrate covers monster darkness and animal ground.
func TestSpawnPlacementChecksShapesLightAndSubstrate(t *testing.T) {
	rules := spawningTestRuntime(t).current().dimensions["minecraft:overworld"]
	environment := &spawnTestEnvironment{active: true, substrate: block.DefaultStateID("minecraft:stone"), settings: Settings{Dimension: "minecraft:overworld"}}
	entry := SpawnEntry{TypeName: "zombie", Group: Monster}
	pos := entity.Vec3{X: .5, Y: 64, Z: .5}
	check := func() bool { return canPlace(environment, entry, pos, rules, rand.New(rand.NewSource(1))) }
	if !check() {
		t.Fatal("dark supported monster placement rejected")
	}
	environment.blockLight = 1
	if check() {
		t.Fatal("monster spawned under artificial light")
	}
	environment.blockLight, environment.brightness = 0, 15
	if check() {
		t.Fatal("monster spawned in daylight")
	}
	entry.TypeName, entry.Group = "cow", Creature
	if check() {
		t.Fatal("animal spawned on stone")
	}
	environment.substrate = block.DefaultStateID("minecraft:grass_block")
	if !check() {
		t.Fatal("bright grass placement rejected")
	}
	environment.substrate = block.DefaultStateID("minecraft:water")
	if check() {
		t.Fatal("fluid counted as solid support")
	}
	pos.Y = 320
	if check() {
		t.Fatal("mob spawned outside dimension bounds")
	}
}

// TestResourceSpawnerEmitsBiomeMatchingNightMobs verifies the request path beyond placement helpers.
func TestResourceSpawnerEmitsBiomeMatchingNightMobs(t *testing.T) {
	runtime := spawningTestRuntime(t)
	manager := entity.NewManager()
	mobs := entity.NewMobManager(manager, manager.AllocateID)
	spawner := NewSpawner(nil, 1)
	spawner.SetRuntime(runtime)
	spawner.rng = rand.New(rand.NewSource(12))
	environment := &spawnTestEnvironment{active: true, biome: "minecraft:plains", sky: 15, brightness: 15, substrate: block.DefaultStateID("minecraft:stone"), settings: Settings{Dimension: "minecraft:overworld", Enabled: true, Spawn: entity.Vec3{X: 10000}}}
	players := []PlayerPos{{Y: 64}}
	for range 200 {
		if len(spawner.Tick(mobs, players, environment).Requests) != 0 {
			t.Fatal("daytime stone terrain produced a spawn request")
		}
	}
	environment.sky, environment.brightness = 0, 0
	for range 1000 {
		requests := spawner.Tick(mobs, players, environment).Requests
		if len(requests) == 0 {
			continue
		}
		for _, request := range requests {
			if !hasEntry(runtime.current().biomes[environment.biome][Monster], request.TypeName) || request.Y != 64 {
				t.Fatalf("invalid biome request: %+v", request)
			}
			if mob := mobs.SpawnMob(request.TypeName, entity.Vec3{X: request.X, Y: request.Y, Z: request.Z}); mob == nil {
				t.Fatal("accepted request did not create a mob")
			}
		}
		return
	}
	t.Fatal("dark biome never produced an executable spawn request")
}

// TestResourceSpawnerHonorsPersistencePeacefulAndCategoryCaps covers connected policy decisions.
func TestResourceSpawnerHonorsPersistencePeacefulAndCategoryCaps(t *testing.T) {
	runtime := spawningTestRuntime(t)
	manager := entity.NewManager()
	mobs := entity.NewMobManager(manager, manager.AllocateID)
	spawner := NewSpawner(nil, 1)
	spawner.SetRuntime(runtime)
	spawner.rng = rand.New(rand.NewSource(12))
	environment := &spawnTestEnvironment{active: true, biome: "minecraft:plains", substrate: block.DefaultStateID("minecraft:stone"), settings: Settings{Dimension: "minecraft:overworld", Enabled: true, Spawn: entity.Vec3{X: 10000}}}
	players := []PlayerPos{{Y: 64}}
	for range 70 {
		mobs.SpawnMob("spider", entity.Vec3{X: 30, Y: 64})
	}
	for range 500 {
		if requests := spawner.Tick(mobs, players, environment).Requests; len(requests) != 0 {
			t.Fatal("neutral spiders bypassed the monster cap")
		}
	}
	persistent := mobs.SpawnMob("zombie", entity.Vec3{X: 200, Y: 64})
	persistent.SetPersistenceRequired(true)
	animal := mobs.SpawnMob("cow", entity.Vec3{X: 200, Y: 64})
	environment.settings.Enabled = false
	spawner.Tick(mobs, players, environment)
	if persistent.Removed() || animal.Removed() {
		t.Fatal("persistent mob or animal distance-despawned")
	}
	environment.settings.Peaceful = true
	spawner.Tick(mobs, players, environment)
	if !persistent.Removed() || animal.Removed() {
		t.Fatal("Peaceful failed to remove a persistent monster")
	}
	inactive := mobs.SpawnMob("zombie", entity.Vec3{X: 200, Y: 64})
	environment.active = false
	spawner.Tick(mobs, players, environment)
	if inactive.Removed() {
		t.Fatal("inactive mob despawned")
	}
}

// TestLocalCapsReserveAllNearbyPlayers verifies overlapping and independent budgets.
func TestLocalCapsReserveAllNearbyPlayers(t *testing.T) {
	players := []PlayerPos{{X: 0}, {X: 32}, {X: 1024}}
	local := []map[Category]int{{Monster: 69}, {Monster: 70}, {Monster: 0}}
	pos := entity.Vec3{X: 48}
	if !localAllows(local, players, pos, Monster, 70) {
		t.Fatal("available overlapping player budget rejected")
	}
	addLocal(local, players, pos, Monster)
	if localAllows(local, players, pos, Monster, 70) || local[2][Monster] != 0 {
		t.Fatal("spawn failed to reserve local budgets")
	}
}

// TestInvalidSpawningReloadKeepsSnapshot rejects corrupted spawn definitions atomically.
func TestInvalidSpawningReloadKeepsSnapshot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pack.mcmeta"), []byte(`{"pack":{"pack_format":61,"description":"spawning test"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "data", "test", "worldgen", "biome")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(path, "biome.json")
	if err := os.WriteFile(file, []byte(`{"spawners":{"monster":[{"type":"minecraft:zombie","weight":1,"minCount":1,"maxCount":2}]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{root}); err != nil {
		t.Fatal(err)
	}
	previous := resources.Current()
	if err := os.WriteFile(file, []byte(`{"spawners":{"monster":[{"type":"minecraft:zombie","weight":0,"minCount":5,"maxCount":2}]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{root}); err == nil || resources.Current() != previous || runtime.current() == nil {
		t.Fatal("invalid reload replaced active spawn data")
	}
}
