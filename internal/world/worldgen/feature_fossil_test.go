package worldgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// fossilTestFeature creates a small paired template with the shipped fossil processors.
func fossilTestFeature(t *testing.T, diamonds bool) *fossilFeature {
	t.Helper()
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	baseProcessors, err := decodeTemplateProcessors(manager.Current(), json.RawMessage(`"minecraft:fossil_rot"`))
	if err != nil {
		t.Fatal(err)
	}
	name := "minecraft:fossil_coal"
	if diamonds {
		name = "minecraft:fossil_diamonds"
	}
	encoded, err := json.Marshal(name)
	if err != nil {
		t.Fatal(err)
	}
	overlayProcessors, err := decodeTemplateProcessors(manager.Current(), encoded)
	if err != nil {
		t.Fatal(err)
	}
	bones, err := block.BlockStateFromProperties("minecraft:bone_block", map[string]string{"axis": "x"})
	if err != nil {
		t.Fatal(err)
	}
	base := &structureTemplate{size: blockPosition{3, 2, 3}, palettes: [][]templateBlock{{
		{position: blockPosition{0, 0, 0}, state: bones.ID()},
		{position: blockPosition{1, 0, 0}, state: bones.ID()},
		{position: blockPosition{2, 0, 0}, state: bones.ID()},
		{position: blockPosition{1, 1, 1}, state: bones.ID()},
	}}}
	overlay := &structureTemplate{size: base.size, palettes: [][]templateBlock{{{position: blockPosition{1, 0, 0}, state: block.DefaultStateID("minecraft:coal_ore")}}}}
	return &fossilFeature{base: []*structureTemplate{base}, overlay: []*structureTemplate{overlay}, baseProcessors: baseProcessors, overlayProcessors: overlayProcessors, maxEmptyCorners: 4}
}

// fossilTestGround builds a uniform burial site in positive or negative chunk coordinates.
func fossilTestGround(coordinates [2]int32) *section.Chunk {
	target := section.NewChunkWithBounds(coordinates[0], coordinates[1], -64, 384, 0)
	stone := block.DefaultStateID("minecraft:stone")
	for x := int(target.X) * 16; x < int(target.X)*16+16; x++ {
		for z := int(target.Z) * 16; z < int(target.Z)*16+16; z++ {
			for y := -64; y <= 60; y++ {
				target.SetBlock(x, y, z, stone)
			}
		}
	}
	target.EnableHeightmapCache()
	return target
}

// TestFossilGeometryAndRNG checks burial, bone axes, rarity processors, and diamond overlays.
func TestFossilGeometryAndRNG(t *testing.T) {
	for _, diamonds := range []bool{false, true} {
		feature := fossilTestFeature(t, diamonds)
		overlays := 0
		for seed := int64(0); seed < 40; seed++ {
			coordinates := [2]int32{-2, -3}
			target := fossilTestGround(coordinates)
			origin := blockPosition{int(coordinates[0])*16 + 8, 40, int(coordinates[1])*16 + 8}
			random, expected := worldrandom.NewLegacy(seed), worldrandom.NewLegacy(seed)
			rotation := templateRotation(expected.NextIntBound(4))
			expected.NextIntBound(1)
			y := max(origin.y-15-int(expected.NextIntBound(10)), target.MinY()+10)
			expected.NextIntBound(1)
			retained := [4]bool{}
			for index := range retained {
				retained[index] = expected.NextFloat() <= 0.9
			}
			expected.NextIntBound(1)
			overlay := expected.NextFloat() <= 0.1
			if !feature.placeFeature(nil, target, random, origin) {
				t.Fatal("supported fossil rejected")
			}
			if random.NextLong() != expected.NextLong() {
				t.Fatal("fossil consumed extra RNG")
			}
			transform := templateTransform{rotation: rotation}
			base := feature.base[0]
			anchor := base.anchoredOrigin(blockPosition{origin.x - 1, y, origin.z - 1}, transform)
			for index, entry := range base.palettes[0] {
				p := transform.position(entry.position)
				p = blockPosition{anchor.x + p.x, anchor.y + p.y, anchor.z + p.z}
				name := block.NameFromState(getChunkBlock(target, p))
				if index == 1 && overlay {
					want := "minecraft:coal_ore"
					if diamonds {
						want = "minecraft:deepslate_diamond_ore"
					}
					if name != want {
						t.Fatalf("overlay block %s, want %s", name, want)
					}
					overlays++
				} else if retained[index] {
					if name != "minecraft:bone_block" {
						t.Fatalf("retained bone missing at %v", p)
					}
					state, _ := block.BlockStateFromID(getChunkBlock(target, p))
					axis, _ := state.Property("axis")
					want := "x"
					if rotation == templateClockwise90 || rotation == templateCounterclockwise90 {
						want = "z"
					}
					if axis != want {
						t.Fatal("bone axis not rotated")
					}
				} else if name != "minecraft:stone" {
					t.Fatal("rot processor placed a discarded bone")
				}
			}
		}
		if overlays == 0 {
			t.Fatal("no ore overlays generated")
		}
	}
}

// TestFossilRejectsEmptyCorners checks early rejection and vertical minimum burial depth.
func TestFossilRejectsEmptyCorners(t *testing.T) {
	feature := fossilTestFeature(t, false)
	random, expected := worldrandom.NewLegacy(42), worldrandom.NewLegacy(42)
	target := section.NewChunkWithBounds(0, 0, -64, 384, 0)
	if feature.placeFeature(nil, target, random, blockPosition{8, 40, 8}) {
		t.Fatal("unsupported fossil accepted")
	}
	expected.NextIntBound(4)
	expected.NextIntBound(1)
	expected.NextIntBound(10)
	if random.NextLong() != expected.NextLong() {
		t.Fatal("rejected fossil consumed placement RNG")
	}
	target = fossilTestGround([2]int32{})
	if !feature.placeFeature(nil, target, worldrandom.NewLegacy(42), blockPosition{8, -64, 8}) {
		t.Fatal("minimum depth fossil rejected")
	}
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			for y := -64; y < -54; y++ {
				if block.NameFromState(target.GetBlock(x, y, z)) != "minecraft:stone" {
					t.Fatal("fossil crossed minimum burial margin")
				}
			}
		}
	}
}

// TestFossilProcessorsProtectTerrain ensures both skeleton and ore passes retain protected blocks.
func TestFossilProcessorsProtectTerrain(t *testing.T) {
	feature := fossilTestFeature(t, true)
	target := fossilTestGround([2]int32{})
	bedrock := block.DefaultStateID("minecraft:bedrock")
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			for y := -54; y < 40; y++ {
				target.SetBlock(x, y, z, bedrock)
			}
		}
	}
	if !feature.placeFeature(nil, target, worldrandom.NewLegacy(3), blockPosition{8, 40, 8}) {
		t.Fatal("protected site rejected instead of filtering blocks")
	}
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			for y := -54; y < 40; y++ {
				if target.GetBlock(x, y, z) != bedrock {
					t.Fatal("fossil replaced protected bedrock")
				}
			}
		}
	}
}

// TestShippedFossilFeatures checks both resource definitions and deterministic real template placement.
func TestShippedFossilFeatures(t *testing.T) {
	for _, id := range []string{"fossil_coal", "fossil_diamonds"} {
		feature := loadSmallFeature(t, id).(*fossilFeature)
		if len(feature.base) != 8 || len(feature.overlay) != 8 {
			t.Fatal("missing fossil variants")
		}
		placed := 0
		for seed := int64(0); seed < 24; seed++ {
			a, b := fossilTestGround([2]int32{}), fossilTestGround([2]int32{})
			origin := blockPosition{8, 40, 8}
			r1, r2 := worldrandom.NewLegacy(seed), worldrandom.NewLegacy(seed)
			ok1, ok2 := feature.placeFeature(nil, a, r1, origin), feature.placeFeature(nil, b, r2, origin)
			if ok1 != ok2 || r1.NextLong() != r2.NextLong() {
				t.Fatal("nondeterministic fossil placement")
			}
			for index := range a.Sections {
				if !reflect.DeepEqual(a.Sections[index].Blocks, b.Sections[index].Blocks) {
					t.Fatal("nondeterministic fossil blocks")
				}
			}
			if ok1 {
				placed++
			}
		}
		if placed == 0 {
			t.Fatal("no shipped fossil variant placed")
		}
	}
}

// TestFossilConfigValidation rejects absent templates, processors, pairs, and corner limits.
func TestFossilConfigValidation(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := manager.Current().Get("data/minecraft/worldgen/configured_feature/fossil_coal.json")
	for _, entry := range []struct {
		key   string
		value any
	}{
		{"fossil_structures", []string{}},
		{"overlay_structures", []string{"minecraft:fossil/missing"}},
		{"fossil_processors", "minecraft:missing"},
		{"max_empty_corners_allowed", 8},
	} {
		var modified map[string]any
		if err := json.Unmarshal(raw, &modified); err != nil {
			t.Fatal(err)
		}
		modified["config"].(map[string]any)[entry.key] = entry.value
		encoded, err := json.Marshal(modified)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := decodeConfiguredFeature(manager.Current(), encoded); err == nil {
			t.Fatalf("invalid %s accepted", entry.key)
		}
	}
}

// TestFossilTemplateReloadRejectsCorruption keeps the previous compiled snapshot on invalid overrides.
func TestFossilTemplateReloadRejectsCorruption(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	original := runtime.current()
	pack := writeTemplateTestPack(t, encodeTemplateTest(t, templateTestRoot(), true))
	path := filepath.Join(pack, "data", "minecraft", "structure", "fossil")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "spine_1.nbt"), []byte{0x1f, 0x8b}, 0600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla", pack}); err == nil {
		t.Fatal("invalid fossil template published")
	}
	if runtime.current() != original || original.features.configured["minecraft:fossil_coal"] == nil {
		t.Fatal("failed reload replaced valid fossil resources")
	}
}

// TestTemplateProcessorOrdering reads original terrain before any accepted template block is written.
func TestTemplateProcessorOrdering(t *testing.T) {
	target := section.NewChunkWithBounds(0, 0, -64, 384, 0)
	stone := block.DefaultStateID("minecraft:stone")
	coal := block.DefaultStateID("minecraft:coal_ore")
	target.SetBlock(8, 30, 8, stone)
	template := &structureTemplate{size: blockPosition{1, 1, 1}, palettes: [][]templateBlock{{{position: blockPosition{}, state: coal}, {position: blockPosition{}, state: coal}}}}
	processor := templateProcessor{kind: "minecraft:rule", rules: []templateRule{{location: templateRuleTest{states: map[int32]struct{}{stone: {}}}, output: block.DefaultStateID("minecraft:diamond_ore")}}}
	if !template.place(target, worldrandom.NewLegacy(0), blockPosition{8, 30, 8}, templateTransform{}, []templateProcessor{processor}) {
		t.Fatal("template failed to place")
	}
	if name := block.NameFromState(target.GetBlock(8, 30, 8)); name != "minecraft:diamond_ore" {
		t.Fatal("processors observed earlier writes from same template")
	}
}

// TestTemplateRotIntegrityIncludesZero verifies the reference's inclusive retention comparison.
func TestTemplateRotIntegrityIncludesZero(t *testing.T) {
	processor := templateProcessor{kind: "minecraft:block_rot", integrity: 0}
	entry := templateBlock{state: block.DefaultStateID("minecraft:stone")}
	random := templateZeroFloatSource{}
	if _, ok := processor.apply(section.NewChunkWithBounds(0, 0, 0, 256, 0), random, entry, entry); !ok {
		t.Fatal("zero random value must be retained at zero integrity")
	}
}

// templateZeroFloatSource supplies the inclusive zero boundary without advancing another RNG.
type templateZeroFloatSource struct{ worldrandom.Source }

// NextFloat returns the smallest representable RNG sample.
func (templateZeroFloatSource) NextFloat() float32 { return 0 }

// TestFossilParallelDeterminism compares concurrent placements sharing immutable templates and processors.
func TestFossilParallelDeterminism(t *testing.T) {
	feature := loadSmallFeature(t, "fossil_diamonds").(*fossilFeature)
	origin := blockPosition{8, 40, 8}
	reference := fossilTestGround([2]int32{})
	random := worldrandom.NewLegacy(0)
	expectedPlacement := feature.placeFeature(nil, reference, random, origin)
	expectedRandom := random.NextLong()
	type result struct {
		chunk  *section.Chunk
		placed bool
		next   int64
	}
	results := make(chan result, 8)
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			target := fossilTestGround([2]int32{})
			random := worldrandom.NewLegacy(0)
			placed := feature.placeFeature(nil, target, random, origin)
			results <- result{target, placed, random.NextLong()}
		}()
	}
	workers.Wait()
	close(results)
	for result := range results {
		if result.placed != expectedPlacement || result.next != expectedRandom {
			t.Fatal("parallel placement or RNG differs")
		}
		assertGeneratedContent(t, result.chunk, reference)
	}
}
