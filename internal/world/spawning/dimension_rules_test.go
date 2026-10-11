package spawning

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/entity"
)

// writeSpawnTestResource writes one overlay used by spawn-definition reload tests.
func writeSpawnTestResource(t *testing.T, root, path, data string) {
	t.Helper()
	path = filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

// spawnEdgeEnvironment supplies one adjacent block without changing the candidate's center.
type spawnEdgeEnvironment struct {
	spawnTestEnvironment
	edge int32
}

// GetBlockStateAt exposes edge states intersecting a wide mob's bounding box.
func (e *spawnEdgeEnvironment) GetBlockStateAt(x, y, z int) int32 {
	if x == -1 && y >= 64 {
		return e.edge
	}
	return e.spawnTestEnvironment.GetBlockStateAt(x, y, z)
}

// TestSpawnPlacementChecksWholeBodyRejectsFluidsAndUnloadedEdges verifies bounding-box admission.
func TestSpawnPlacementChecksWholeBodyRejectsFluidsAndUnloadedEdges(t *testing.T) {
	rules := spawningTestRuntime(t).current().dimensions["minecraft:overworld"]
	environment := &spawnEdgeEnvironment{spawnTestEnvironment: spawnTestEnvironment{active: true, substrate: block.DefaultStateID("minecraft:stone"), settings: Settings{Dimension: "minecraft:overworld"}}}
	entry := SpawnEntry{TypeName: "spider", Group: Monster}
	check := func() bool {
		return canPlace(environment, entry, entity.Vec3{X: .5, Y: 64, Z: .5}, rules, rand.New(rand.NewSource(1)))
	}
	if !check() {
		t.Fatal("empty supported volume rejected")
	}
	for _, state := range []int32{-1, block.DefaultStateID("minecraft:water"), block.DefaultStateID("minecraft:seagrass"), block.DefaultStateID("minecraft:lava"), block.DefaultStateID("minecraft:stone")} {
		environment.edge = state
		if check() {
			t.Fatalf("intersecting state %d outside center column was accepted", state)
		}
	}
}

// TestCustomSpawnDimensionsResolveTypeBounds verifies gameplay rules for dimension aliases.
func TestCustomSpawnDimensionsResolveTypeBounds(t *testing.T) {
	runtime, resources, packs, _ := costTestRuntime(t)
	path := "data/test/dimension/underworld.json"
	writeSpawnTestResource(t, packs[1], path, `{"type":"minecraft:the_nether","generator":{"type":"minecraft:noise","settings":"minecraft:nether","biome_source":{"type":"minecraft:multi_noise","preset":"minecraft:nether"}}}`)
	if err := resources.Reload(packs); err != nil {
		t.Fatal(err)
	}
	definitions := runtime.current()
	if definitions.dimensions["test:underworld"] != definitions.dimensions["minecraft:the_nether"] {
		t.Fatal("custom dimension ignored type rules")
	}
	environment := &spawnTestEnvironment{active: true, substrate: block.DefaultStateID("minecraft:netherrack"), settings: Settings{Dimension: "test:underworld"}}
	rules := definitions.dimensions[environment.settings.Dimension]
	entry := SpawnEntry{TypeName: "zombie", Group: Monster}
	if !canPlace(environment, entry, entity.Vec3{X: .5, Y: 64, Z: .5}, rules, rand.New(rand.NewSource(1))) || canPlace(environment, entry, entity.Vec3{X: .5, Y: 256, Z: .5}, rules, rand.New(rand.NewSource(1))) {
		t.Fatal("custom dimension bounds were not enforced")
	}
	previous := resources.Current()
	writeSpawnTestResource(t, packs[1], path, `{"type":"test:missing"}`)
	if err := resources.Reload(packs); err == nil || resources.Current() != previous {
		t.Fatal("unknown dimension type replaced spawn snapshot")
	}
}

// TestSpeciesLightRulesDoNotBypassMonsterDarkness verifies blaze and wither skeleton placement.
func TestSpeciesLightRulesDoNotBypassMonsterDarkness(t *testing.T) {
	definitions := spawningTestRuntime(t).current()
	environment := &spawnTestEnvironment{active: true, substrate: block.DefaultStateID("minecraft:netherrack"), settings: Settings{Dimension: "minecraft:the_nether"}, blockLight: 15, brightness: 15}
	pos := entity.Vec3{X: .5, Y: 64, Z: .5}
	check := func(name string) bool {
		return canPlace(environment, SpawnEntry{TypeName: name, Group: Monster}, pos, definitions.dimensions[environment.settings.Dimension], rand.New(rand.NewSource(1)))
	}
	if !check("blaze") {
		t.Fatal("blaze incorrectly required darkness")
	}
	if check("wither_skeleton") {
		t.Fatal("wither skeleton bypassed dimension light test")
	}
	environment.blockLight, environment.brightness = 7, 7
	if !check("wither_skeleton") {
		t.Fatal("wither skeleton rejected at inclusive Nether light limit")
	}
	environment.blockLight, environment.brightness = 8, 8
	if check("wither_skeleton") {
		t.Fatal("wither skeleton spawned beyond Nether light limit")
	}
	environment.settings.Peaceful = true
	if check("blaze") || check("wither_skeleton") {
		t.Fatal("special placement bypassed Peaceful")
	}
}
