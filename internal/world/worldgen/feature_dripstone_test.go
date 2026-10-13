package worldgen

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type pointedDripstoneChange struct {
	Position   [3]int            `json:"position"`
	Name       string            `json:"name"`
	Properties map[string]string `json:"properties"`
}

type pointedDripstoneFixture struct {
	Shape   int                      `json:"shape"`
	Seed    int64                    `json:"seed"`
	Origin  [3]int                   `json:"origin"`
	Placed  bool                     `json:"placed"`
	Tail    int64                    `json:"tail"`
	Changes []pointedDripstoneChange `json:"changes"`
}

// pointedDripstoneInput mirrors the controlled cave configurations in the versioned exporter.
func pointedDripstoneInput(origin blockPosition, shape int) map[blockPosition]int32 {
	input := make(map[blockPosition]int32)
	stone := block.DefaultStateID("minecraft:stone")
	if shape == 5 {
		input[blockPosition{origin.x, origin.y + 1, origin.z}] = block.DefaultStateID("minecraft:dripstone_block")
	}
	for _, layer := range []struct {
		matches bool
		y       int
	}{{shape == 0 || shape == 2 || shape == 4, origin.y + 1}, {shape == 1 || shape == 2, origin.y - 1}} {
		if !layer.matches {
			continue
		}
		for x := origin.x - 3; x <= origin.x+3; x++ {
			for z := origin.z - 3; z <= origin.z+3; z++ {
				input[blockPosition{x, layer.y, z}] = stone
			}
		}
	}
	if shape == 4 {
		water := block.DefaultStateID("minecraft:water")
		input[origin] = water
		input[blockPosition{origin.x, origin.y - 1, origin.z}] = water
	}
	return input
}

// TestPointedDripstoneMatchesMinecraft1214 checks geometry, state properties, and RNG.
func TestPointedDripstoneMatchesMinecraft1214(t *testing.T) {
	data, err := os.ReadFile("testdata/pointed_dripstone_1_21_4.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []pointedDripstoneFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 48 {
		t.Fatalf("pointed dripstone reference cases = %d, want 48", len(fixtures))
	}
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	if runtime.current().features.configured["minecraft:pointed_dripstone"] == nil ||
		runtime.current().features.placed["minecraft:pointed_dripstone"] == nil {
		t.Fatal("pointed dripstone selector or placement is missing")
	}
	raw, ok := manager.Current().Get("data/minecraft/worldgen/configured_feature/pointed_dripstone.json")
	if !ok {
		t.Fatal("missing pointed dripstone definition")
	}
	var definition struct {
		Config struct {
			Features []struct {
				Feature json.RawMessage `json:"feature"`
			} `json:"features"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &definition); err != nil || len(definition.Config.Features) == 0 {
		t.Fatalf("decode pointed dripstone selector: %v", err)
	}
	f, err := decodePointedDripstoneFeature(manager.Current(), definition.Config.Features[0].Feature)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		origin := blockPosition{fixture.Origin[0], fixture.Origin[1], fixture.Origin[2]}
		input := pointedDripstoneInput(origin, fixture.Shape)
		chunk := multifaceWorkspace(t, int32(origin.x>>4), int32(origin.z>>4), input)
		random := worldrandom.NewLegacy(fixture.Seed)
		if placed := f.placeFeature(nil, chunk, random, origin); placed != fixture.Placed {
			t.Fatalf("shape %d seed %d: placed %v, want %v", fixture.Shape, fixture.Seed, placed, fixture.Placed)
		}
		if tail := random.NextLong(); tail != fixture.Tail {
			t.Fatalf("shape %d seed %d: RNG tail %d, want %d", fixture.Shape, fixture.Seed, tail, fixture.Tail)
		}
		changes := make([]pointedDripstoneChange, 0)
		for x := origin.x - 3; x <= origin.x+3; x++ {
			for z := origin.z - 3; z <= origin.z+3; z++ {
				for y := origin.y - 3; y <= origin.y+3; y++ {
					p := blockPosition{x, y, z}
					state := getChunkBlock(chunk, p)
					if state == input[p] {
						continue
					}
					properties := block.PropertiesFromState(state)
					if properties == nil {
						properties = map[string]string{}
					}
					changes = append(changes, pointedDripstoneChange{[3]int{x, y, z}, block.NameFromState(state), properties})
				}
			}
		}
		if !reflect.DeepEqual(changes, fixture.Changes) {
			t.Fatalf("shape %d seed %d: changed states = %v, want %v", fixture.Shape, fixture.Seed, changes, fixture.Changes)
		}
	}
}
