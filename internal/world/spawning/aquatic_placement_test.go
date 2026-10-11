package spawning

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/block/fluid"
	"github.com/vitismc/vitis/internal/entity"
)

// aquaticTestEnvironment supplies an ocean with individually overridden resident states.
type aquaticTestEnvironment struct {
	spawnTestEnvironment
	states map[[3]int]int32
}

// GetBlock supplies the same ocean geometry to height and collision queries.
func (e *aquaticTestEnvironment) GetBlock(x, y, z int) int32 { return e.GetBlockStateAt(x, y, z) }

// GetBlockStateAt exposes sand below a water column ending at sea level.
func (e *aquaticTestEnvironment) GetBlockStateAt(x, y, z int) int32 {
	if state, ok := e.states[[3]int{x, y, z}]; ok {
		return state
	}
	if y < 40 {
		return block.DefaultStateID("minecraft:sand")
	}
	if y < 63 {
		return fluid.WaterSourceState
	}
	return 0
}

// TestDrownedPlacementChecksDepthWaterLightAndActivity exercises complete placement admission.
func TestDrownedPlacementChecksDepthWaterLightAndActivity(t *testing.T) {
	definitions := spawningTestRuntime(t).current()
	entry := SpawnEntry{TypeName: "drowned", Group: Monster}
	rules := definitions.dimensions["minecraft:overworld"]
	for _, test := range []struct {
		name    string
		biome   string
		y       int
		change  func(*aquaticTestEnvironment)
		allowed bool
	}{
		{"river surface", "minecraft:river", 61, nil, true},
		{"ocean depth", "minecraft:ocean", 57, nil, true},
		{"ocean depth boundary", "minecraft:ocean", 58, nil, false},
		{"unknown sea level", "minecraft:ocean", 57, func(e *aquaticTestEnvironment) { e.settings.SeaLevelKnown = false }, false},
		{"river ignores sea level", "minecraft:river", 61, func(e *aquaticTestEnvironment) { e.settings.SeaLevelKnown = false }, true},
		{"artificial light", "minecraft:river", 57, func(e *aquaticTestEnvironment) { e.blockLight = 1 }, false},
		{"daylight", "minecraft:river", 57, func(e *aquaticTestEnvironment) { e.brightness = 15 }, false},
		{"peaceful", "minecraft:river", 57, func(e *aquaticTestEnvironment) { e.settings.Peaceful = true }, false},
		{"inactive", "minecraft:river", 57, func(e *aquaticTestEnvironment) { e.active = false }, false},
		{"dry feet", "minecraft:river", 57, func(e *aquaticTestEnvironment) { e.states[[3]int{0, 57, 0}] = 0 }, false},
		{"dry below", "minecraft:river", 57, func(e *aquaticTestEnvironment) { e.states[[3]int{0, 56, 0}] = 0 }, false},
		{"solid head", "minecraft:river", 57, func(e *aquaticTestEnvironment) { e.states[[3]int{0, 58, 0}] = block.DefaultStateID("minecraft:stone") }, false},
		{"unavailable head", "minecraft:river", 57, func(e *aquaticTestEnvironment) { e.states[[3]int{0, 58, 0}] = -1 }, false},
		{"submerged vegetation", "minecraft:river", 57, func(e *aquaticTestEnvironment) {
			e.states[[3]int{0, 57, 0}] = block.DefaultStateID("minecraft:seagrass")
		}, true},
		{"lava", "minecraft:river", 57, func(e *aquaticTestEnvironment) { e.states[[3]int{0, 57, 0}] = fluid.LavaSourceState }, false},
		{"dimension floor", "minecraft:river", -64, nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			environment := &aquaticTestEnvironment{spawnTestEnvironment: spawnTestEnvironment{active: true, biome: test.biome, settings: Settings{Dimension: "minecraft:overworld", SeaLevel: 63, SeaLevelKnown: true}}, states: map[[3]int]int32{}}
			if test.change != nil {
				test.change(environment)
			}
			rng := rand.New(rand.NewSource(19))
			accepted := false
			for range 2000 {
				accepted = canPlace(environment, entry, entity.Vec3{X: .5, Y: float64(test.y), Z: .5}, rules, rng, definitions) || accepted
			}
			if accepted != test.allowed {
				t.Fatalf("admitted=%v want=%v", accepted, test.allowed)
			}
		})
	}
}

// TestDrownedSpawnRequestsCreateMobs verifies natural river requests reach entity creation.
func TestDrownedSpawnRequestsCreateMobs(t *testing.T) {
	runtime := spawningTestRuntime(t)
	manager := entity.NewManager()
	mobs := entity.NewMobManager(manager, manager.AllocateID)
	spawner := NewSpawner(nil, 1)
	spawner.SetRuntime(runtime)
	spawner.rng = rand.New(rand.NewSource(19))
	environment := &aquaticTestEnvironment{spawnTestEnvironment: spawnTestEnvironment{active: true, biome: "minecraft:river", settings: Settings{Dimension: "minecraft:overworld", Enabled: true, SeaLevel: 63, SeaLevelKnown: true, Spawn: entity.Vec3{X: 10000}}}}
	players := []PlayerPos{{Y: 60}}
	for range 3000 {
		for _, request := range spawner.Tick(mobs, players, environment) {
			if request.TypeName != "drowned" {
				t.Fatalf("land mob spawned in ocean: %+v", request)
			}
			if request.Y < 41 || request.Y >= 63 {
				t.Fatalf("invalid drowned depth: %+v", request)
			}
			mob := mobs.SpawnMob(request.TypeName, entity.Vec3{X: request.X, Y: request.Y, Z: request.Z})
			if mob == nil || mob.TypeName() != "drowned" {
				t.Fatal("accepted aquatic request did not create drowned")
			}
			return
		}
	}
	t.Fatal("ocean never produced an executable drowned request")
}

// TestDrownedBiomeTagsReloadAtomically verifies custom tag membership and rollback.
func TestDrownedBiomeTagsReloadAtomically(t *testing.T) {
	runtime, resources, packs, _ := costTestRuntime(t)
	previous := resources.Current()
	directory := filepath.Join(packs[1], "data", "minecraft", "tags", "worldgen", "biome")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "more_frequent_drowned_spawns.json")
	if err := os.WriteFile(path, []byte(`{"replace":true,"values":["test:cost"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload(packs); err != nil {
		t.Fatal(err)
	}
	definitions := runtime.current()
	if resources.Current() == previous || !definitions.biomeTags["more_frequent_drowned_spawns"]["test:cost"] || definitions.biomeTags["more_frequent_drowned_spawns"]["minecraft:river"] {
		t.Fatal("tag reload did not replace river membership")
	}
	previous = resources.Current()
	if err := os.WriteFile(path, []byte(`{"replace":true,"values":["#test:missing"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload(packs); err == nil {
		t.Fatal("missing required tag accepted")
	}
	if resources.Current() != previous || runtime.current() != definitions {
		t.Fatal("failed tag reload replaced spawn snapshot")
	}
}
