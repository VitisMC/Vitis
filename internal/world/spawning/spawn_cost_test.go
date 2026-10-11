package spawning

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/entity"
)

// TestSpawnPotentialUsesBlockPositionsAndAllCharges verifies distance budgets and reservations.
func TestSpawnPotentialUsesBlockPositionsAndAllCharges(t *testing.T) {
	var potential spawnPotential
	potential.add(entity.Vec3{X: .01, Y: 64.1, Z: .01}, 2)
	if potential.allows(entity.Vec3{X: .99, Y: 64.99, Z: .99}, spawnCost{charge: 1, budget: 100}) {
		t.Fatal("overlapping block positions admitted another charge")
	}
	position := entity.Vec3{X: 1.9, Y: 64, Z: .5}
	if !potential.allows(position, spawnCost{charge: 1, budget: 2}) || potential.allows(position, spawnCost{charge: 1, budget: 1.99}) {
		t.Fatal("inclusive energy budget or integer distance incorrect")
	}
	potential.add(entity.Vec3{X: -1, Y: 64}, 2)
	if potential.allows(position, spawnCost{charge: 1, budget: 2}) {
		t.Fatal("newly reserved charge was ignored")
	}
	if !potential.allows(entity.Vec3{Y: 64}, spawnCost{}) {
		t.Fatal("uncharged species was restricted by other mobs")
	}
	potential.add(position, 0)
	if len(potential) != 2 {
		t.Fatal("zero charge retained an overlapping point")
	}
}

// costBiomeEnvironment chooses existing and candidate costs from distinct biomes.
type costBiomeEnvironment struct{ spawnTestEnvironment }

// SpawnBiome exposes one fixed biome boundary for potential tests.
func (e *costBiomeEnvironment) SpawnBiome(x, y, z int) string {
	if x < 0 {
		return "minecraft:soul_sand_valley"
	}
	return "minecraft:warped_forest"
}

// TestSpawnChargesUseEachMobsOwnBiome verifies resources at both sides of a boundary.
func TestSpawnChargesUseEachMobsOwnBiome(t *testing.T) {
	definitions := spawningTestRuntime(t).current()
	environment := &costBiomeEnvironment{}
	existing := entity.Vec3{X: -5.5, Y: 64, Z: .5}
	candidate := entity.Vec3{X: 1.5, Y: 64, Z: .5}
	charge := definitions.costAt(environment, "enderman", existing)
	cost := definitions.costAt(environment, "enderman", candidate)
	if charge.charge != .7 || cost.charge != 1 || cost.budget != .12 {
		t.Fatalf("existing=%+v candidate=%+v", charge, cost)
	}
	var potential spawnPotential
	potential.add(existing, charge.charge)
	if !potential.allows(candidate, cost) {
		t.Fatal("existing mob was charged using the candidate biome")
	}
	if definitions.costAt(environment, "cow", candidate) != (spawnCost{}) {
		t.Fatal("absent species cost was not unrestricted")
	}
}

// costTestRuntime creates one restrictive biome overlay and the real dimension definitions.
func costTestRuntime(t *testing.T) (*Runtime, *datapack.Manager, []string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pack.mcmeta"), []byte(`{"pack":{"pack_format":61,"description":"costs"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "data", "test", "worldgen", "biome")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "cost.json")
	if err := os.WriteFile(path, []byte(`{"spawners":{"monster":[{"type":"minecraft:zombie","weight":1,"minCount":4,"maxCount":4}]},"spawn_costs":{"minecraft:zombie":{"charge":1,"energy_budget":0}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	packs := []string{"../../../.mcdata/1.21.4/vanilla", root}
	if err := resources.Reload(packs); err != nil {
		t.Fatal(err)
	}
	return runtime, resources, packs, path
}

// TestResourceSpawnCostsReserveRequestsAndIgnorePersistentMobs verifies runtime admission.
func TestResourceSpawnCostsReserveRequestsAndIgnorePersistentMobs(t *testing.T) {
	runtime, _, _, _ := costTestRuntime(t)
	manager := entity.NewManager()
	mobs := entity.NewMobManager(manager, manager.AllocateID)
	spawner := NewSpawner(nil, 1)
	spawner.SetRuntime(runtime)
	spawner.rng = rand.New(rand.NewSource(12))
	environment := &spawnTestEnvironment{active: true, biome: "test:cost", substrate: block.DefaultStateID("minecraft:stone"), settings: Settings{Dimension: "minecraft:overworld", Enabled: true, Spawn: entity.Vec3{X: 10000}}}
	players := []PlayerPos{{Y: 64}}
	var spawned *entity.MobEntity
	for range 1000 {
		requests := spawner.Tick(mobs, players, environment)
		if len(requests) > 1 {
			t.Fatal("same-cycle requests bypassed the zero energy budget")
		}
		if len(requests) == 1 {
			request := requests[0]
			spawned = mobs.SpawnMob(request.TypeName, entity.Vec3{X: request.X, Y: request.Y, Z: request.Z})
			break
		}
	}
	if spawned == nil {
		t.Fatal("empty biome did not admit its first charge")
	}
	for range 500 {
		if len(spawner.Tick(mobs, players, environment)) != 0 {
			t.Fatal("existing mob did not restrict new charges")
		}
	}
	spawned.SetPersistenceRequired(true)
	for range 1000 {
		if requests := spawner.Tick(mobs, players, environment); len(requests) == 1 {
			return
		}
	}
	t.Fatal("persistent mob retained an energy charge")
}

// TestInvalidSpawnCostReloadKeepsSnapshot verifies validation and atomic resource rollback.
func TestInvalidSpawnCostReloadKeepsSnapshot(t *testing.T) {
	runtime, resources, packs, path := costTestRuntime(t)
	previous := resources.Current()
	for _, raw := range []string{
		`{"minecraft:zombie":{"charge":"invalid","energy_budget":0}}`,
		`{"minecraft:zombie":{"charge":1,"energy_budget":null}}`,
		`{"minecraft:zombie":{"charge":1e309,"energy_budget":0}}`,
		`{"minecraft:zombie":{}}`,
		`{"test:unknown":{"charge":1,"energy_budget":0}}`,
	} {
		if err := os.WriteFile(path, []byte(`{"spawn_costs":`+raw+`}`), 0600); err != nil {
			t.Fatal(err)
		}
		if err := resources.Reload(packs); err == nil {
			t.Fatalf("invalid cost accepted: %s", raw)
		}
		if resources.Current() != previous || runtime.current().costs["test:cost"]["zombie"].charge != 1 {
			t.Fatal("failed reload replaced the active cost snapshot")
		}
	}
}

// TestSpawnCostSignedCodecValues verifies finite signed doubles retain their resource semantics.
func TestSpawnCostSignedCodecValues(t *testing.T) {
	values, err := compileSpawnCosts(map[string]json.RawMessage{"minecraft:zombie": json.RawMessage(`{"charge":-1,"energy_budget":-1}`)})
	if err != nil {
		t.Fatal(err)
	}
	var potential spawnPotential
	potential.add(entity.Vec3{Y: 64}, 2)
	if !potential.allows(entity.Vec3{X: 1, Y: 64}, values["zombie"]) || !potential.allows(entity.Vec3{Y: 64}, values["zombie"]) {
		t.Fatal("signed potential or coincident-block energy incorrect")
	}
	if potential.allows(entity.Vec3{Y: 64}, spawnCost{budget: -1}) {
		t.Fatal("zero energy ignored a negative budget")
	}
}
