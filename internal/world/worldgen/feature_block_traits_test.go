package worldgen

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// TestGenerationFaceGeometry distinguishes support from collision and full from partial faces.
func TestGenerationFaceGeometry(t *testing.T) {
	for _, test := range []struct {
		name            string
		properties      map[string]string
		face            featureFace
		support, attach bool
	}{
		{"minecraft:stone", nil, featureUp, true, true},
		{"minecraft:oak_leaves", nil, featureUp, false, true},
		{"minecraft:mud", nil, featureUp, true, true},
		{"minecraft:soul_sand", nil, featureUp, true, true},
		{"minecraft:oak_slab", map[string]string{"type": "bottom"}, featureDown, true, true},
		{"minecraft:oak_slab", map[string]string{"type": "bottom"}, featureUp, false, false},
		{"minecraft:oak_slab", map[string]string{"type": "top"}, featureUp, true, true},
		{"minecraft:oak_slab", map[string]string{"type": "top"}, featureDown, false, false},
		{"minecraft:oak_slab", map[string]string{"type": "double"}, featureEast, true, true},
		{"minecraft:snow", map[string]string{"layers": "1"}, featureDown, true, true},
		{"minecraft:snow", map[string]string{"layers": "1"}, featureUp, false, false},
		{"minecraft:snow", map[string]string{"layers": "8"}, featureUp, true, true},
		{"minecraft:oak_fence", nil, featureUp, false, false},
		{"minecraft:air", nil, featureUp, false, false},
	} {
		state, err := block.BlockStateFromProperties(test.name, test.properties)
		if err != nil {
			t.Fatal(err)
		}
		if support, attach := featureFullFace(state.ID(), test.face, false), featureFullFace(state.ID(), test.face, true); support != test.support || attach != test.attach {
			t.Errorf("%s %v face=%s: support=%v attach=%v, want %v/%v", test.name, test.properties, featureFaceNames[test.face], support, attach, test.support, test.attach)
		}
	}
}

// TestFeaturePredicatesUseStateTraits checks offsets, replacement, solidity and directional support.
func TestFeaturePredicatesUseStateTraits(t *testing.T) {
	for _, test := range []struct {
		kind, name, direction string
		properties            map[string]string
		want                  bool
	}{
		{"replaceable", "minecraft:air", "", nil, true},
		{"replaceable", "minecraft:water", "", nil, true},
		{"replaceable", "minecraft:short_grass", "", nil, true},
		{"replaceable", "minecraft:stone", "", nil, false},
		{"replaceable", "minecraft:snow", "", map[string]string{"layers": "1"}, true},
		{"replaceable", "minecraft:snow", "", map[string]string{"layers": "2"}, true},
		{"solid", "minecraft:stone", "", nil, true},
		{"solid", "minecraft:glass", "", nil, true},
		{"solid", "minecraft:air", "", nil, false},
		{"solid", "minecraft:oak_sign", "", nil, true},
		{"solid", "minecraft:short_grass", "", nil, false},
		{"has_sturdy_face", "minecraft:oak_leaves", "up", nil, false},
		{"has_sturdy_face", "minecraft:mud", "up", nil, true},
		{"has_sturdy_face", "minecraft:oak_slab", "up", map[string]string{"type": "bottom"}, false},
		{"has_sturdy_face", "minecraft:oak_slab", "down", map[string]string{"type": "bottom"}, true},
	} {
		state, err := block.BlockStateFromProperties(test.name, test.properties)
		if err != nil {
			t.Fatal(err)
		}
		position := blockPosition{-1, -20, -16}
		input := map[blockPosition]int32{featureEast.offset(position): state.ID()}
		c := multifaceWorkspace(t, -1, -1, input)
		raw := json.RawMessage(fmt.Sprintf(`{"type":"minecraft:%s","offset":[1,0,0],"direction":%q}`, test.kind, test.direction))
		predicate, err := decodeBlockPredicate(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := predicate.test(nil, c, position); got != test.want {
			t.Errorf("%s %s %v %s = %v, want %v", test.kind, test.name, test.properties, test.direction, got, test.want)
		}
	}
	for _, direction := range []string{"", "UP", "invalid"} {
		if _, err := decodeBlockPredicate(json.RawMessage(fmt.Sprintf(`{"type":"minecraft:has_sturdy_face","direction":%q}`, direction))); err == nil {
			t.Fatalf("invalid direction accepted: %q", direction)
		}
	}
}

// TestMultifaceRejectedOriginsKeepRandomState checks solids, existing growth and vertical write bounds.
func TestMultifaceRejectedOriginsKeepRandomState(t *testing.T) {
	f := loadSmallFeature(t, "glow_lichen").(*multifaceGrowthFeature)
	for _, state := range []int32{block.DefaultStateID("minecraft:stone"), f.states[1<<featureEast]} {
		c := section.NewChunkWithBounds(0, 0, -64, 128, 0)
		origin := blockPosition{8, -20, 8}
		setChunkBlock(c, origin, state)
		random, want := worldrandom.NewLegacy(42), worldrandom.NewLegacy(42)
		if f.placeFeature(nil, c, random, origin) || random.NextLong() != want.NextLong() || getChunkBlock(c, origin) != state {
			t.Fatal("rejected origin changed the world or random stream")
		}
	}
	c := section.NewChunkWithBounds(0, 0, -64, 128, 0)
	for _, y := range []int{-65, 64} {
		random, want := worldrandom.NewLegacy(42), worldrandom.NewLegacy(42)
		if f.placeFeature(nil, c, random, blockPosition{8, y, 8}) || random.NextLong() != want.NextLong() {
			t.Fatal("out-of-height placement accepted or consumed random values")
		}
	}
}
