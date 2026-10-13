package spawning

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/entity"
)

// slimeTestEnvironment provides a floor at the tested depth with optional obstacles.
type slimeTestEnvironment struct {
	spawnTestEnvironment
	floorY int
	states map[[3]int]int32
}

// GetBlock supplies the same resident geometry to spawn-height selection.
func (e *slimeTestEnvironment) GetBlock(x, y, z int) int32 { return e.GetBlockStateAt(x, y, z) }

// GetBlockStateAt supplies stone below the floor and individually controlled cells.
func (e *slimeTestEnvironment) GetBlockStateAt(x, y, z int) int32 {
	if state, ok := e.states[[3]int{x, y, z}]; ok {
		return state
	}
	if y < e.floorY {
		return block.DefaultStateID("minecraft:stone")
	}
	return 0
}

// TestSlimePlacementUsesBiomeMoonSeedAndLight exercises complete admission at both spawn heights.
func TestSlimePlacementUsesBiomeMoonSeedAndLight(t *testing.T) {
	definitions := spawningTestRuntime(t).current()
	rules := definitions.dimensions["minecraft:overworld"]
	for _, test := range []struct {
		name                      string
		x, y                      int
		biome                     string
		day                       int64
		light                     uint8
		peaceful, active, allowed bool
	}{
		{"swamp full moon", 0, 64, "minecraft:swamp", 0, 0, false, true, true},
		{"swamp half moon", 0, 64, "minecraft:swamp", 2 * 24000, 0, false, true, true},
		{"new moon", 0, 64, "minecraft:swamp", 4 * 24000, 0, false, true, false},
		{"bright swamp", 0, 64, "minecraft:swamp", 0, 8, false, true, false},
		{"plains surface", 0, 64, "minecraft:plains", 0, 0, false, true, false},
		{"surface lower boundary", 0, 50, "minecraft:swamp", 0, 0, false, true, false},
		{"surface upper boundary", 0, 70, "minecraft:swamp", 0, 0, false, true, false},
		{"seeded underground", -32, 39, "minecraft:plains", 4 * 24000, 15, false, true, true},
		{"underground boundary", -32, 40, "minecraft:plains", 0, 0, false, true, false},
		{"non-slime chunk", 0, 39, "minecraft:plains", 0, 0, false, true, false},
		{"peaceful", -32, 39, "minecraft:plains", 0, 0, true, true, false},
		{"inactive", -32, 39, "minecraft:plains", 0, 0, false, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			environment := &slimeTestEnvironment{spawnTestEnvironment: spawnTestEnvironment{active: test.active, biome: test.biome, brightness: test.light, settings: Settings{Dimension: "minecraft:overworld", Peaceful: test.peaceful, DayTime: test.day}}, floorY: test.y}
			entry := SpawnEntry{TypeName: "slime", Group: Monster, slimeSize: 4}
			rng := rand.New(rand.NewSource(31))
			accepted := false
			for range 300 {
				accepted = canPlace(environment, entry, entity.Vec3{X: float64(test.x) + .5, Y: float64(test.y), Z: .5}, rules, rng, definitions) || accepted
			}
			if accepted != test.allowed {
				t.Fatalf("admitted=%v want=%v", accepted, test.allowed)
			}
		})
	}
}

// TestSlimePlacementChecksFullSizedBody rejects walls, ceilings and unloaded cells outside the center column.
func TestSlimePlacementChecksFullSizedBody(t *testing.T) {
	definitions := spawningTestRuntime(t).current()
	for _, obstacle := range [][3]int{{-33, 39, 0}, {-32, 41, 0}, {-31, 39, 0}} {
		for _, state := range []int32{block.DefaultStateID("minecraft:stone"), block.DefaultStateID("minecraft:water"), -1} {
			environment := &slimeTestEnvironment{spawnTestEnvironment: spawnTestEnvironment{active: true, biome: "minecraft:plains"}, floorY: 39, states: map[[3]int]int32{obstacle: state}}
			entry := SpawnEntry{TypeName: "slime", Group: Monster, slimeSize: 4}
			rng := rand.New(rand.NewSource(31))
			for range 100 {
				if canPlace(environment, entry, entity.Vec3{X: -31.5, Y: 39, Z: .5}, definitions.dimensions["minecraft:overworld"], rng, definitions) {
					t.Fatalf("large slime intersected cell %v, state %d", obstacle, state)
				}
			}
		}
	}
}

// TestSlimeNaturalSizesUseLocalDifficulty checks all vanilla sizes and increased large-slime frequency.
func TestSlimeNaturalSizesUseLocalDifficulty(t *testing.T) {
	for _, test := range []struct {
		settings  Settings
		inhabited int64
		want      float32
	}{
		{Settings{Difficulty: 1, WorldAge: 1512000}, 3600000, 0},
		{Settings{Difficulty: 2}, 0, 0},
		{Settings{Difficulty: 3}, 0, .125},
		{Settings{Difficulty: 3, WorldAge: 1512000}, 3600000, 1},
	} {
		if got := specialDifficulty(test.settings, test.inhabited); got != test.want {
			t.Fatalf("local difficulty=%v want=%v", got, test.want)
		}
	}
	counts := func(settings Settings, inhabited int64) map[int]int {
		rng := rand.New(rand.NewSource(7))
		result := map[int]int{}
		for range 10000 {
			size := naturalSlimeSize(rng, settings, inhabited)
			if size != 1 && size != 2 && size != 4 {
				t.Fatalf("invalid natural size %d", size)
			}
			result[size]++
		}
		return result
	}
	easy := counts(Settings{Difficulty: 1}, 0)
	hard := counts(Settings{Difficulty: 3, WorldAge: 1512000}, 3600000)
	if easy[1] == 0 || easy[2] == 0 || easy[4] == 0 || hard[4] <= easy[4] || hard[1] >= easy[1] {
		t.Fatalf("difficulty did not affect sizes: easy=%v hard=%v", easy, hard)
	}
	if moonBrightness(-24000) != .75 || moonBrightness(8*24000) != 1 {
		t.Fatal("moon cycle wrapped incorrectly")
	}
}

// TestSlimeSpawnRequestsReserveActualSizes verifies resource-backed selection and executable requests.
func TestSlimeSpawnRequestsReserveActualSizes(t *testing.T) {
	runtime, resources, packs, path := costTestRuntime(t)
	if err := os.WriteFile(path, []byte(`{"spawners":{"monster":[{"type":"minecraft:slime","weight":1,"minCount":1,"maxCount":1}]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload(packs); err != nil {
		t.Fatal(err)
	}
	manager := entity.NewManager()
	mobs := entity.NewMobManager(manager, manager.AllocateID)
	spawner := NewSpawner(nil, 1)
	spawner.SetRuntime(runtime)
	spawner.rng = rand.New(rand.NewSource(31))
	environment := &slimeTestEnvironment{spawnTestEnvironment: spawnTestEnvironment{active: true, biome: "test:cost", settings: Settings{Dimension: "minecraft:overworld", Enabled: true, Seed: 0, Spawn: entity.Vec3{X: 10000}}}, floorY: 30}
	players := []PlayerPos{{Y: 30}}
	for range 3000 {
		for _, request := range spawner.Tick(mobs, players, environment).Requests {
			if request.TypeName != "slime" || request.SlimeSize != 1 && request.SlimeSize != 2 && request.SlimeSize != 4 || request.Y != 30 {
				t.Fatalf("invalid executable slime request: %+v", request)
			}
			mob := mobs.SpawnMob(request.TypeName, entity.Vec3{X: request.X, Y: request.Y, Z: request.Z})
			mob.SetSlimeSize(request.SlimeSize, true)
			if mob.SlimeSize() != request.SlimeSize {
				t.Fatal("request size was lost during spawn")
			}
			return
		}
	}
	t.Fatal("resource-backed slime chunk never spawned a slime")
}

// TestSlimeBiomeTagReloadKeepsPreviousSnapshot validates new surface memberships atomically.
func TestSlimeBiomeTagReloadKeepsPreviousSnapshot(t *testing.T) {
	runtime, resources, packs, _ := costTestRuntime(t)
	directory := filepath.Join(packs[1], "data", "minecraft", "tags", "worldgen", "biome")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "allows_surface_slime_spawns.json")
	if err := os.WriteFile(path, []byte(`{"replace":true,"values":["test:cost"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload(packs); err != nil {
		t.Fatal(err)
	}
	previous := resources.Current()
	if !runtime.current().biomeTags["allows_surface_slime_spawns"]["test:cost"] || runtime.current().biomeTags["allows_surface_slime_spawns"]["minecraft:swamp"] {
		t.Fatal("replacement surface biome tag was not applied")
	}
	if err := os.WriteFile(path, []byte(`{"values":["#test:missing"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload(packs); err == nil || resources.Current() != previous {
		t.Fatal("failed biome tag reload replaced the active snapshot")
	}
}
