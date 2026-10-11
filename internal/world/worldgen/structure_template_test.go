package worldgen

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// templateTestPosition creates the standard NBT coordinate list used by templates.
func templateTestPosition(x, y, z int32) *nbt.List {
	return nbt.NewList(nbt.TagInt).Add(x).Add(y).Add(z)
}

// templateTestRoot includes deliberately unordered blocks and an entity payload.
func templateTestRoot() *nbt.Compound {
	palette := nbt.NewList(nbt.TagCompound).
		Add(nbt.NewCompound().PutString("Name", "minecraft:stone")).
		Add(nbt.NewCompound().PutString("Name", "minecraft:wall_torch").PutCompound("Properties", nbt.NewCompound().PutString("facing", "north"))).
		Add(nbt.NewCompound().PutString("Name", "minecraft:chest"))
	blocks := nbt.NewList(nbt.TagCompound).
		Add(nbt.NewCompound().PutList("pos", templateTestPosition(2, 0, 1)).PutInt("state", 2).PutCompound("nbt", nbt.NewCompound().PutString("id", "minecraft:chest").PutString("LootTable", "minecraft:chests/simple_dungeon"))).
		Add(nbt.NewCompound().PutList("pos", templateTestPosition(0, 0, 1)).PutInt("state", 1)).
		Add(nbt.NewCompound().PutList("pos", templateTestPosition(1, 1, 0)).PutInt("state", 0)).
		Add(nbt.NewCompound().PutList("pos", templateTestPosition(1, 0, 0)).PutInt("state", 0))
	entity := nbt.NewCompound().PutList("pos", nbt.NewList(nbt.TagDouble).Add(0.25).Add(1.0).Add(1.75)).
		PutList("blockPos", templateTestPosition(0, 1, 1)).PutCompound("nbt", nbt.NewCompound().PutString("id", "minecraft:armor_stand"))
	return nbt.NewCompound().PutList("size", templateTestPosition(3, 2, 2)).PutList("palette", palette).PutList("blocks", blocks).PutList("entities", nbt.NewList(nbt.TagCompound).Add(entity))
}

// encodeTemplateTest serializes a template using either named raw NBT or gzip NBT.
func encodeTemplateTest(t *testing.T, root *nbt.Compound, compressed bool) []byte {
	t.Helper()
	encoder := nbt.NewEncoder(1024)
	if err := encoder.WriteNamedRootCompound("", root); err != nil {
		t.Fatal(err)
	}
	if !compressed {
		return encoder.Bytes()
	}
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(encoder.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// TestStructureTemplateDecoding verifies gzip, multiple palettes, ordering, and preserved entity data.
func TestStructureTemplateDecoding(t *testing.T) {
	for _, compressed := range []bool{false, true} {
		root := templateTestRoot()
		palette, _ := root.GetList("palette")
		alternate := palette.Clone()
		alternate.Elements()[0] = nbt.NewCompound().PutString("Name", "minecraft:dirt")
		root.PutList("palettes", nbt.NewList(nbt.TagList).Add(palette).Add(alternate))
		root.Delete("palette")
		template, err := decodeStructureTemplate(encodeTemplateTest(t, root, compressed))
		if err != nil {
			t.Fatal(err)
		}
		if len(template.palettes) != 2 || len(template.entities) != 1 {
			t.Fatal("template palettes or entity lost")
		}
		if template.palettes[0][0].position != (blockPosition{1, 0, 0}) || template.palettes[0][1].position != (blockPosition{1, 1, 0}) || template.palettes[0][2].position != (blockPosition{0, 0, 1}) || template.palettes[0][3].data == nil {
			t.Fatal("template blocks not sorted by class and Y/X/Z")
		}
		if block.NameFromState(template.palettes[1][0].state) != "minecraft:dirt" {
			t.Fatal("alternate palette discarded")
		}
		if template.entities[0].position != [3]float64{0.25, 1, 1.75} {
			t.Fatal("entity position lost precision")
		}
	}
}

// TestStructureTemplateEntityOnly accepts valid templates with entities and an empty block palette.
func TestStructureTemplateEntityOnly(t *testing.T) {
	root := templateTestRoot().PutList("blocks", nbt.NewList(nbt.TagEnd)).PutList("palette", nbt.NewList(nbt.TagEnd))
	template, err := decodeStructureTemplate(encodeTemplateTest(t, root, true))
	if err != nil {
		t.Fatal(err)
	}
	if len(template.palettes) != 1 || len(template.palettes[0]) != 0 || len(template.entities) != 1 {
		t.Fatal("entity-only template data lost")
	}
}

// TestStructureTemplateDefaultNamespace resolves valid unqualified block names.
func TestStructureTemplateDefaultNamespace(t *testing.T) {
	root := templateTestRoot()
	palette, _ := root.GetList("palette")
	palette.Elements()[0].(*nbt.Compound).PutString("Name", "stone")
	template, err := decodeStructureTemplate(encodeTemplateTest(t, root, true))
	if err != nil {
		t.Fatal(err)
	}
	if block.NameFromState(template.palettes[0][0].state) != "minecraft:stone" {
		t.Fatal("default namespace not resolved")
	}
}

// TestStructureTemplateRejectsMalformedNBT checks the allocation and state validation boundaries.
func TestStructureTemplateRejectsMalformedNBT(t *testing.T) {
	cases := map[string]func(*nbt.Compound){
		"zero size": func(root *nbt.Compound) { root.PutList("size", templateTestPosition(0, 2, 2)) },
		"huge size": func(root *nbt.Compound) { root.PutList("size", templateTestPosition(513, 2, 2)) },
		"bad index": func(root *nbt.Compound) {
			blocks, _ := root.GetList("blocks")
			blocks.Elements()[0].(*nbt.Compound).PutInt("state", 99)
		},
		"bad position": func(root *nbt.Compound) {
			blocks, _ := root.GetList("blocks")
			blocks.Elements()[0].(*nbt.Compound).PutList("pos", templateTestPosition(-1, 0, 0))
		},
		"unknown state": func(root *nbt.Compound) {
			palette, _ := root.GetList("palette")
			palette.Elements()[0].(*nbt.Compound).PutString("Name", "minecraft:missing")
		},
		"bad property": func(root *nbt.Compound) {
			palette, _ := root.GetList("palette")
			palette.Elements()[1].(*nbt.Compound).PutCompound("Properties", nbt.NewCompound().PutString("facing", "diagonal"))
		},
		"wrong property type": func(root *nbt.Compound) {
			palette, _ := root.GetList("palette")
			palette.Elements()[1].(*nbt.Compound).PutCompound("Properties", nbt.NewCompound().PutInt("facing", 1))
		},
		"bad payload": func(root *nbt.Compound) {
			blocks, _ := root.GetList("blocks")
			blocks.Elements()[0].(*nbt.Compound).PutString("nbt", "bad")
		},
		"wrong entity coordinate": func(root *nbt.Compound) {
			entities, _ := root.GetList("entities")
			entities.Elements()[0].(*nbt.Compound).PutList("pos", templateTestPosition(0, 0, 0))
		},
	}
	for name, modify := range cases {
		t.Run(name, func(t *testing.T) {
			root := templateTestRoot()
			modify(root)
			if _, err := decodeStructureTemplate(encodeTemplateTest(t, root, true)); err == nil {
				t.Fatal("malformed template accepted")
			}
		})
	}
	data := append(encodeTemplateTest(t, templateTestRoot(), false), 0)
	if _, err := decodeStructureTemplate(data); err == nil {
		t.Fatal("trailing data accepted")
	}
	var bomb bytes.Buffer
	writer := gzip.NewWriter(&bomb)
	if _, err := writer.Write(make([]byte, maxTemplateBytes+1)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeStructureTemplate(bomb.Bytes()); err == nil {
		t.Fatal("oversized decompressed template accepted")
	}
	data = encodeTemplateTest(t, templateTestRoot(), true)
	data[len(data)-1] ^= 1
	if _, err := decodeStructureTemplate(data); err == nil {
		t.Fatal("invalid gzip checksum accepted")
	}
}

// TestVanillaStructureTemplates validates every shipped template against current block states.
func TestVanillaStructureTemplates(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	store := newStructureTemplates(manager.Current())
	if len(store.entries) != 1201 {
		t.Fatalf("template count %d", len(store.entries))
	}
	ids := make([]string, 0, len(store.entries))
	for id := range store.entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	states := make(map[int32]struct{})
	for _, id := range ids {
		template, err := store.load(id)
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		for _, palette := range template.palettes {
			for _, entry := range palette {
				states[entry.state] = struct{}{}
			}
		}
	}
	for state := range states {
		for rotation := templateRotationNone; rotation <= templateCounterclockwise90; rotation++ {
			for mirror := templateMirrorNone; mirror <= templateMirrorFrontBack; mirror++ {
				transform := templateTransform{rotation: rotation, mirror: mirror}
				if _, err := transform.state(state); err != nil {
					t.Errorf("state %d rotation %d mirror %d: %v", state, rotation, mirror, err)
				}
			}
		}
		rotated := state
		for range 4 {
			var err error
			rotated, err = (templateTransform{rotation: templateClockwise90}).state(rotated)
			if err != nil {
				t.Fatal(err)
			}
		}
		if rotated != state {
			t.Errorf("four rotations changed state %d into %d", state, rotated)
		}
		for _, mirror := range []templateMirror{templateMirrorLeftRight, templateMirrorFrontBack} {
			transform := templateTransform{mirror: mirror}
			mirrored, err := transform.state(state)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := transform.state(mirrored)
			if err != nil {
				t.Fatal(err)
			}
			if restored != state {
				t.Errorf("two mirrors changed state %d into %d", state, restored)
			}
		}
	}
	t.Logf("validated %d templates and %d distinct block states", len(ids), len(states))
}

// writeTemplateTestPack writes a minimal override pack used by reload and feature tests.
func writeTemplateTestPack(t *testing.T, data []byte) string {
	t.Helper()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "pack.mcmeta"), []byte(`{"pack":{"pack_format":61,"description":"template test"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "data", "test", "structure")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "sample.nbt"), data, 0600); err != nil {
		t.Fatal(err)
	}
	return directory
}

// TestStructureTemplateConcurrentLoadAndReload checks single decoding and old snapshot isolation.
func TestStructureTemplateConcurrentLoadAndReload(t *testing.T) {
	pack := writeTemplateTestPack(t, encodeTemplateTest(t, templateTestRoot(), true))
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{pack}); err != nil {
		t.Fatal(err)
	}
	old := newStructureTemplates(manager.Current())
	var wg sync.WaitGroup
	results := make(chan *structureTemplate, 32)
	errors := make(chan error, 32)
	for range 32 {
		wg.Add(1)
		go func() { defer wg.Done(); value, err := old.load("test:sample"); results <- value; errors <- err }()
	}
	wg.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var original *structureTemplate
	for value := range results {
		if original == nil {
			original = value
		}
		if value != original {
			t.Fatal("template decoded more than once")
		}
	}
	root := templateTestRoot()
	palette, _ := root.GetList("palette")
	palette.Elements()[0].(*nbt.Compound).PutString("Name", "minecraft:dirt")
	if err := os.WriteFile(filepath.Join(pack, "data", "test", "structure", "sample.nbt"), encodeTemplateTest(t, root, true), 0600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{pack}); err != nil {
		t.Fatal(err)
	}
	fresh, err := newStructureTemplates(manager.Current()).load("test:sample")
	if err != nil {
		t.Fatal(err)
	}
	oldValue, err := old.load("test:sample")
	if err != nil || oldValue != original || block.NameFromState(oldValue.palettes[0][0].state) != "minecraft:stone" || block.NameFromState(fresh.palettes[0][0].state) != "minecraft:dirt" {
		t.Fatal("reload changed an existing template snapshot")
	}
}

// TestStructureTemplatePlacementCopiesNBT protects cached templates from loot and coordinate mutations.
func TestStructureTemplatePlacementCopiesNBT(t *testing.T) {
	template, err := decodeStructureTemplate(encodeTemplateTest(t, templateTestRoot(), true))
	if err != nil {
		t.Fatal(err)
	}
	for _, coordinates := range [][2]int32{{0, 0}, {-2, -3}} {
		for rotation := templateRotationNone; rotation <= templateCounterclockwise90; rotation++ {
			target := section.NewChunkWithBounds(coordinates[0], coordinates[1], -64, 384, 0)
			transform := templateTransform{rotation: rotation, mirror: templateMirrorFrontBack}
			origin := template.anchoredOrigin(blockPosition{int(coordinates[0])*16 + 3, 30, int(coordinates[1])*16 + 3}, transform)
			random, expected := worldrandom.NewLegacy(42), worldrandom.NewLegacy(42)
			expected.NextIntBound(1)
			lootSeed := expected.NextLong()
			if !template.place(target, random, origin, transform, nil) {
				t.Fatal("template rejected")
			}
			chest := transform.position(blockPosition{2, 0, 1})
			chest = blockPosition{origin.x + chest.x, origin.y + chest.y, origin.z + chest.z}
			payload := smallFeatureEntityNBT(t, target, chest)
			if seed, _ := payload.GetLong("LootTableSeed"); seed != lootSeed {
				t.Fatal("incorrect generated loot seed")
			}
			if x, _ := payload.GetInt("x"); x != int32(chest.x) {
				t.Fatal("template block entity position not transformed")
			}
			payload.PutString("LootTable", "changed")
			if _, ok := template.palettes[0][3].data.GetLong("LootTableSeed"); ok {
				t.Fatal("template NBT mutated by placement")
			}
			if random.NextLong() != expected.NextLong() {
				t.Fatal("template consumed unexpected RNG")
			}
		}
	}
	if table, _ := template.palettes[0][3].data.GetString("LootTable"); table != "minecraft:chests/simple_dungeon" {
		t.Fatal("template loot table mutated")
	}
}

// TestTemplateContainerSeedConsumption distinguishes empty containers from brushable loot payloads.
func TestTemplateContainerSeedConsumption(t *testing.T) {
	for _, entry := range []struct {
		name, id  string
		container bool
	}{
		{"minecraft:chest", "minecraft:chest", true},
		{"minecraft:decorated_pot", "minecraft:decorated_pot", true},
		{"minecraft:crafter", "minecraft:crafter", true},
		{"minecraft:suspicious_sand", "minecraft:brushable_block", false},
	} {
		t.Run(entry.name, func(t *testing.T) {
			data := nbt.NewCompound().PutString("id", entry.id)
			if !entry.container {
				data.PutString("LootTable", "minecraft:archaeology/desert_well").PutLong("LootTableSeed", 123)
			}
			template := &structureTemplate{size: blockPosition{1, 1, 1}, palettes: [][]templateBlock{{{state: block.DefaultStateID(entry.name), data: data}}}}
			target := section.NewChunkWithBounds(0, 0, -64, 384, 0)
			random, expected := worldrandom.NewLegacy(42), worldrandom.NewLegacy(42)
			expected.NextIntBound(1)
			if entry.container {
				expected.NextLong()
			}
			if !template.place(target, random, blockPosition{8, 30, 8}, templateTransform{}, nil) {
				t.Fatal("template placement failed")
			}
			if random.NextLong() != expected.NextLong() {
				t.Fatal("unexpected container seed RNG")
			}
			payload := smallFeatureEntityNBT(t, target, blockPosition{8, 30, 8})
			if seed, exists := payload.GetLong("LootTableSeed"); entry.container && exists || !entry.container && (!exists || seed != 123) {
				t.Fatal("incorrect persistent loot seed")
			}
		})
	}
}

// TestTemplateTransformGeometry checks negative coordinates, nonzero pivots, mirrors, and bounds.
func TestTemplateTransformGeometry(t *testing.T) {
	template := &structureTemplate{size: blockPosition{5, 3, 7}}
	for rotation := templateRotationNone; rotation <= templateCounterclockwise90; rotation++ {
		for mirror := templateMirrorNone; mirror <= templateMirrorFrontBack; mirror++ {
			transform := templateTransform{rotation: rotation, mirror: mirror, pivot: blockPosition{3, 0, -2}}
			origin := template.anchoredOrigin(blockPosition{-19, -30, -27}, transform)
			low, high := template.bounds(origin, transform)
			size := template.rotatedSize(rotation)
			if low != (blockPosition{-19, -30, -27}) || high != (blockPosition{-19 + size.x - 1, -28, -27 + size.z - 1}) {
				t.Fatalf("incorrect transform bounds %v %v", low, high)
			}
			for x := 0; x < template.size.x; x++ {
				for z := 0; z < template.size.z; z++ {
					p := transform.position(blockPosition{x, 0, z})
					e := transform.entityPosition([3]float64{float64(x) + 0.5, 0.5, float64(z) + 0.5})
					if e != [3]float64{float64(p.x) + 0.5, 0.5, float64(p.z) + 0.5} {
						t.Fatal("entity and block transform centers differ")
					}
				}
			}
		}
	}
	position := (templateTransform{mirror: templateMirrorLeftRight, pivot: blockPosition{50, 0, 40}}).position(blockPosition{2, 1, 3})
	if position != (blockPosition{2, 1, -3}) {
		t.Fatal("mirror plane incorrectly uses rotation pivot")
	}
}

// TestTemplateDirectionalStates checks rail, stair, door, orientation, and connection transforms.
func TestTemplateDirectionalStates(t *testing.T) {
	cases := []struct {
		name               string
		props              map[string]string
		transform          templateTransform
		property, expected string
	}{
		{"minecraft:bone_block", map[string]string{"axis": "x"}, templateTransform{rotation: templateClockwise90}, "axis", "z"},
		{"minecraft:rail", map[string]string{"shape": "south_east"}, templateTransform{rotation: templateClockwise90}, "shape", "south_west"},
		{"minecraft:powered_rail", map[string]string{"shape": "ascending_north"}, templateTransform{mirror: templateMirrorLeftRight}, "shape", "ascending_south"},
		{"minecraft:oak_stairs", map[string]string{"facing": "north", "shape": "inner_left"}, templateTransform{mirror: templateMirrorLeftRight}, "shape", "inner_right"},
		{"minecraft:oak_stairs", map[string]string{"facing": "east", "shape": "inner_left"}, templateTransform{mirror: templateMirrorFrontBack}, "shape", "inner_left"},
		{"minecraft:oak_stairs", map[string]string{"facing": "east", "shape": "outer_left"}, templateTransform{mirror: templateMirrorFrontBack}, "shape", "outer_right"},
		{"minecraft:oak_door", map[string]string{"hinge": "left"}, templateTransform{mirror: templateMirrorLeftRight}, "hinge", "right"},
		{"minecraft:jigsaw", map[string]string{"orientation": "up_north"}, templateTransform{rotation: templateClockwise90}, "orientation", "up_east"},
		{"minecraft:oak_sign", map[string]string{"rotation": "3"}, templateTransform{mirror: templateMirrorLeftRight, rotation: templateClockwise90}, "rotation", "9"},
		{"minecraft:vine", map[string]string{"north": "true", "east": "false"}, templateTransform{rotation: templateClockwise90}, "east", "true"},
	}
	for index, entry := range cases {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			state, err := block.BlockStateFromProperties(entry.name, entry.props)
			if err != nil {
				t.Fatal(err)
			}
			id, err := entry.transform.state(state.ID())
			if err != nil {
				t.Fatal(err)
			}
			result, _ := block.BlockStateFromID(id)
			if value, _ := result.Property(entry.property); value != entry.expected {
				t.Fatalf("%s = %s, want %s", entry.property, value, entry.expected)
			}
		})
	}
}
