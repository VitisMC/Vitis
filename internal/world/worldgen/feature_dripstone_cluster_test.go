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

// clusterDripstoneInput mirrors the controlled caves in the versioned exporter.
func clusterDripstoneInput(origin blockPosition, shape int) map[blockPosition]int32 {
	input := make(map[blockPosition]int32)
	stone := block.DefaultStateID("minecraft:stone")
	for x := origin.x - 10; x <= origin.x+10; x++ {
		for z := origin.z - 10; z <= origin.z+10; z++ {
			for offset := 5; offset <= 9; offset++ {
				if shape == 0 || shape == 1 || shape == 3 {
					input[blockPosition{x, origin.y + offset, z}] = stone
				}
				if shape == 0 || shape == 2 || shape == 3 {
					input[blockPosition{x, origin.y - offset, z}] = stone
				}
			}
		}
	}
	if shape == 3 {
		input[origin] = stone
	}
	return input
}

// TestDripstoneClusterMatchesMinecraft1214 checks changed states and RNG tails.
func TestDripstoneClusterMatchesMinecraft1214(t *testing.T) {
	data, err := os.ReadFile("testdata/dripstone_cluster_1_21_4.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []pointedDripstoneFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 16 {
		t.Fatalf("dripstone cluster reference cases = %d, want 16", len(fixtures))
	}
	manager := &datapack.Manager{}
	if _, err := NewRuntime(manager); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	raw, ok := manager.Current().Get("data/minecraft/worldgen/configured_feature/dripstone_cluster.json")
	if !ok {
		t.Fatal("missing dripstone cluster definition")
	}
	f, err := decodeDripstoneClusterFeature(manager.Current(), raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		origin := blockPosition{fixture.Origin[0], fixture.Origin[1], fixture.Origin[2]}
		input := clusterDripstoneInput(origin, fixture.Shape)
		chunk := multifaceWorkspace(t, int32(origin.x>>4), int32(origin.z>>4), input)
		random := worldrandom.NewLegacy(fixture.Seed)
		if placed := f.placeFeature(nil, chunk, random, origin); placed != fixture.Placed {
			t.Fatalf("shape %d seed %d: placed %v, want %v", fixture.Shape, fixture.Seed, placed, fixture.Placed)
		}
		if tail := random.NextLong(); tail != fixture.Tail {
			t.Fatalf("shape %d seed %d: RNG tail %d, want %d", fixture.Shape, fixture.Seed, tail, fixture.Tail)
		}
		changes := make([]pointedDripstoneChange, 0)
		for x := origin.x - 10; x <= origin.x+10; x++ {
			for z := origin.z - 10; z <= origin.z+10; z++ {
				for y := origin.y - 12; y <= origin.y+12; y++ {
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
