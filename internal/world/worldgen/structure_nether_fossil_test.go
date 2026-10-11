package worldgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	generatedbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/datapack"
	worldchunk "github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// TestNetherFossilSiteReference checks raw-column site and template choices against Minecraft 1.21.4.
func TestNetherFossilSiteReference(t *testing.T) {
	data, err := os.ReadFile("testdata/nether_fossil_sites_1_21_4.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Seed     int64
		Chunk    [2]int32
		Shape    int
		Point    []int
		Rotation string
		Template int
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 576 {
		t.Fatalf("reference cases = %d, want 576", len(cases))
	}
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	fossil := runtime.current().netherFossils["minecraft:nether_fossil"]
	if fossil == nil {
		t.Fatal("Nether fossil not compiled")
	}
	biomes := fixedBiomeSource{generatedbiome.BiomeSoulSandValley}
	for index, example := range cases {
		column := fossilSiteColumn(example.Shape)
		start, err := fossil.planWithColumn(example.Seed, example.Chunk[0], example.Chunk[1], 0, 128, 32, biomes,
			func(_, _ int) noiseColumn { return column })
		if err != nil {
			t.Fatalf("case %d: %v", index, err)
		}
		if len(example.Point) == 0 {
			if start != nil {
				t.Errorf("case %d: unexpected start %+v", index, start.box)
			}
			continue
		}
		if start == nil {
			t.Errorf("case %d: missing start at %v", index, example.Point)
			continue
		}
		piece := start.pieces[0]
		point := []int{intFromTag(t, piece.nbt, "TPX"), intFromTag(t, piece.nbt, "TPY"), intFromTag(t, piece.nbt, "TPZ")}
		if !reflect.DeepEqual(point, example.Point) {
			t.Errorf("case %d: site %v, want %v", index, point, example.Point)
		}
		rotation, _ := piece.nbt.GetString("Rot")
		name, _ := piece.nbt.GetString("Template")
		if rotation != example.Rotation || name != fmt.Sprintf("minecraft:nether_fossils/fossil_%d", example.Template) {
			t.Errorf("case %d: rotation/template %s/%s, want %s/%d", index, rotation, name, example.Rotation, example.Template)
		}
		if start.id != "minecraft:nether_fossil" || start.step != 7 || start.adjustment != terrainAdjustmentThin || len(piece.blocks) == 0 {
			t.Errorf("case %d: incomplete structure start", index)
		}
	}
}

// TestNetherFossilTemplateReference checks all fourteen templates and four rotations against Minecraft geometry.
func TestNetherFossilTemplateReference(t *testing.T) {
	data, err := os.ReadFile("testdata/nether_fossil_templates_1_21_4.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Template int
		Rotation string
		Box      [6]int
		Bones    [][]json.RawMessage
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 56 {
		t.Fatalf("template rotations = %d, want 56", len(cases))
	}
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	fossil := runtime.current().netherFossils["minecraft:nether_fossil"]
	origin := blockPosition{-17, 50, 22}
	for _, example := range cases {
		if example.Template < 1 || example.Template > 14 {
			t.Fatalf("bad template number %d", example.Template)
		}
		rotation := slices.Index([]string{"NONE", "CLOCKWISE_90", "CLOCKWISE_180", "COUNTERCLOCKWISE_90"}, example.Rotation)
		if rotation < 0 {
			t.Fatalf("unknown rotation %s", example.Rotation)
		}
		template := fossil.templates[example.Template-1]
		transform := templateTransform{rotation: templateRotation(rotation)}
		low, high := template.bounds(origin, transform)
		if got := [6]int{low.x, low.y, low.z, high.x, high.y, high.z}; got != example.Box {
			t.Errorf("template %d rotation %s bounds=%v want=%v", example.Template, example.Rotation, got, example.Box)
		}
		random := worldrandom.NewLegacy(worldrandom.PositionSeed(int32(origin.x), int32(origin.y), int32(origin.z)))
		palette := template.palettes[random.NextIntBound(int32(len(template.palettes)))]
		var got []string
		for _, item := range palette {
			if block.NameFromState(item.state) != "minecraft:bone_block" {
				continue
			}
			position := transform.position(item.position)
			state, err := transform.state(item.state)
			if err != nil {
				t.Fatal(err)
			}
			value, _ := block.BlockStateFromID(state)
			axis, ok := value.Property("axis")
			if !ok {
				t.Fatal("bone without axis")
			}
			got = append(got, fmt.Sprintf("%d,%d,%d,%s", position.x+origin.x, position.y+origin.y, position.z+origin.z, axis))
		}
		var want []string
		for _, bone := range example.Bones {
			if len(bone) != 4 {
				t.Fatal("malformed reference bone")
			}
			var x, y, z int
			var axis string
			if err := json.Unmarshal(bone[0], &x); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(bone[1], &y); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(bone[2], &z); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(bone[3], &axis); err != nil {
				t.Fatal(err)
			}
			want = append(want, fmt.Sprintf("%d,%d,%d,%s", x, y, z, axis))
		}
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("template %d rotation %s: %d bones differ from %d reference bones", example.Template, example.Rotation, len(got), len(want))
		}
	}
}

// fossilSiteColumn supplies the same nine controlled noise-column shapes as the reference exporter.
func fossilSiteColumn(shape int) noiseColumn {
	column := noiseColumn{minY: 0, blocks: make([]int32, 128)}
	floor := 50
	if shape == 7 {
		floor = 32
	} else if shape == 8 {
		floor = 105
	}
	for y := range column.blocks {
		if shape == 2 || shape != 1 && y > floor {
			continue
		}
		name := "minecraft:netherrack"
		if y == floor && shape != 1 {
			switch shape {
			case 3:
				name = "minecraft:soul_sand"
			case 4, 5:
				name = "minecraft:oak_slab"
			case 6:
				name = "minecraft:lava"
			}
		}
		column.blocks[y] = block.DefaultStateID(name)
		if shape == 5 && y == floor {
			state, err := block.BlockStateFromProperties("minecraft:oak_slab", map[string]string{"type": "top", "waterlogged": "false"})
			if err != nil {
				panic(err)
			}
			column.blocks[y] = state.ID()
		}
	}
	return column
}

// intFromTag reads one required integer from a persisted structure piece.
func intFromTag(t *testing.T, root interface{ GetInt(string) (int32, bool) }, key string) int {
	t.Helper()
	value, ok := root.GetInt(key)
	if !ok {
		t.Fatalf("missing %s", key)
	}
	return int(value)
}

// TestNetherFossilColumnMatchesTerrain checks that site scans use the same raw density as terrain generation.
func TestNetherFossilColumnMatchesTerrain(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	for _, seed := range []int64{0, 17} {
		g, err := runtime.NewTerrainGenerator("minecraft:nether", seed, generatedbiome.BiomeSoulSandValley)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := g.snapshot()
		if err != nil {
			t.Fatal(err)
		}
		for _, chunk := range [][2]int32{{0, 0}, {-2, 3}} {
			c := section.NewChunkWithBounds(chunk[0], chunk[1], snapshot.minY, snapshot.height, generatedbiome.BiomeSoulSandValley)
			shape := snapshot.settings.Noise
			g.generateTerrainRange(snapshot, c, int(chunk[0])*16, int(chunk[1])*16, shape.MinY, shape.MinY+shape.Height)
			for _, local := range [][2]int{{0, 0}, {7, 11}, {15, 15}} {
				x, z := int(chunk[0])*16+local[0], int(chunk[1])*16+local[1]
				column := g.structureNoiseColumn(snapshot, x, z)
				for y := snapshot.minY; y < snapshot.minY+snapshot.height; y++ {
					if got, want := column.block(y), c.GetBlock(x, y, z); got != want {
						t.Fatalf("seed=%d chunk=%v at (%d,%d,%d): column=%s terrain=%s", seed, chunk, x, y, z, block.NameFromState(got), block.NameFromState(want))
					}
				}
			}
		}
	}
}

// TestNetherFossilGeneratesStartAndBones verifies the complete structure path on real Nether noise.
func TestNetherFossilGeneratesStartAndBones(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	g, err := runtime.newTerrainGenerator("minecraft:nether", "minecraft:the_nether", json.RawMessage(`{"type":"minecraft:fixed","biome":"minecraft:soul_sand_valley"}`), 0, generatedbiome.BiomeSoulSandValley)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := g.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.structures == nil {
		t.Fatal("Nether fossils disabled for Soul Sand Valley")
	}
	var selected *structureStartPlan
	for x := int32(-12); x <= 12 && selected == nil; x++ {
		for z := int32(-12); z <= 12; z++ {
			starts, err := snapshot.structures.plan(x, z)
			if err != nil {
				t.Fatal(err)
			}
			if len(starts) > 0 {
				selected = starts[0]
				break
			}
		}
	}
	if selected == nil {
		t.Fatal("no structure started across checked candidate chunks")
	}
	chunk, err := g.Generate(selected.x, selected.z)
	if err != nil {
		t.Fatal(err)
	}
	structures, ok := chunk.PersistentNBT().GetCompound("structures")
	if !ok {
		t.Fatal("missing structure NBT")
	}
	starts, ok := structures.GetCompound("starts")
	if !ok {
		t.Fatal("missing starts")
	}
	if _, ok := starts.GetCompound("minecraft:nether_fossil"); !ok {
		t.Fatal("missing owned fossil start")
	}
	refs, ok := structures.GetCompound("References")
	if !ok {
		t.Fatal("missing references")
	}
	if _, _, ok := refs.Get("minecraft:nether_fossil"); !ok {
		t.Fatal("missing fossil references")
	}
	bones := 0
	for _, piece := range selected.pieces {
		for _, item := range piece.blocks {
			p := item.position
			if p.x>>4 == int(selected.x) && p.z>>4 == int(selected.z) && block.NameFromState(chunk.GetBlock(p.x, p.y, p.z)) == "minecraft:bone_block" {
				bones++
			}
		}
	}
	if bones == 0 {
		t.Fatal("fossil geometry was not placed in its owning chunk")
	}
	owned := worldchunk.New(selected.x, selected.z)
	owned.SetData(chunk)
	encoded, err := worldchunk.EncodeChunkNBT(owned)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := worldchunk.DecodeChunkNBTWithBounds(encoded, selected.x, selected.z, generatedbiome.BiomeSoulSandValley, snapshot.minY, snapshot.height)
	if err != nil {
		t.Fatal(err)
	}
	root, ok := loaded.Data().PersistentNBT().GetCompound("structures")
	if !ok {
		t.Fatal("reloaded fossil structure NBT missing")
	}
	loadedStarts, ok := root.GetCompound("starts")
	if !ok {
		t.Fatal("reloaded fossil starts missing")
	}
	if _, ok := loadedStarts.GetCompound("minecraft:nether_fossil"); !ok {
		t.Fatal("reloaded owned fossil start missing")
	}
	for _, piece := range selected.pieces {
		for _, item := range piece.blocks {
			p := item.position
			if p.x>>4 == int(selected.x) && p.z>>4 == int(selected.z) && loaded.Data().GetBlock(p.x, p.y, p.z) != chunk.GetBlock(p.x, p.y, p.z) {
				t.Fatalf("reloaded fossil block differs at %+v", p)
			}
		}
	}
}

// TestNetherFossilAppearsInDefaultNether checks reachability through the shipped climate source.
func TestNetherFossilAppearsInDefaultNether(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	g, err := runtime.NewTerrainGenerator("minecraft:nether", 0, generatedbiome.BiomeNetherWastes)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := g.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.structures == nil {
		t.Fatal("Nether fossils unreachable from default biome source")
	}
	var selected *structureStartPlan
	for x := int32(-20); x <= 20 && selected == nil; x += 2 {
		for z := int32(-20); z <= 20; z += 2 {
			starts, err := snapshot.structures.plan(x, z)
			if err != nil {
				t.Fatal(err)
			}
			if len(starts) > 0 {
				selected = starts[0]
				break
			}
		}
	}
	if selected == nil {
		t.Fatal("no fossil in checked default Nether area")
	}
	generated, err := g.Generate(selected.x, selected.z)
	if err != nil {
		t.Fatal(err)
	}
	root := generated.PersistentNBT()
	structures, ok := root.GetCompound("structures")
	if !ok {
		t.Fatal("missing default Nether structures")
	}
	starts, ok := structures.GetCompound("starts")
	if !ok {
		t.Fatal("missing default Nether starts")
	}
	if _, ok := starts.GetCompound("minecraft:nether_fossil"); !ok {
		t.Fatal("selected fossil was not saved")
	}
}

// TestNetherFossilPlannerSkipsUnreachableDimensions avoids structure work in vanilla Overworld and End.
func TestNetherFossilPlannerSkipsUnreachableDimensions(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	for _, dimension := range []string{"minecraft:overworld", "minecraft:end"} {
		g, err := runtime.NewTerrainGenerator(dimension, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := g.snapshot()
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.structures != nil {
			t.Fatalf("Nether fossil planner enabled in %s", dimension)
		}
	}
}

// TestNetherFossilCrossesChunkBoundary checks owned and imported stage order for a real template.
func TestNetherFossilCrossesChunkBoundary(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	fossil := runtime.current().netherFossils["minecraft:nether_fossil"]
	column := fossilSiteColumn(0)
	var selected *structureStartPlan
	var outside templateBlock
	for seed := int64(0); seed < 128 && selected == nil; seed++ {
		start, err := fossil.planWithColumn(seed, 0, 0, 0, 128, 32, fixedBiomeSource{generatedbiome.BiomeSoulSandValley}, func(_, _ int) noiseColumn { return column })
		if err != nil {
			t.Fatal(err)
		}
		if start == nil {
			continue
		}
		for _, item := range start.pieces[0].blocks {
			if item.position.x>>4 != 0 || item.position.z>>4 != 0 {
				selected, outside = start, item
				break
			}
		}
	}
	if selected == nil {
		t.Fatal("no crossing fossil among checked seeds")
	}
	reader := func(int, int, int) int32 { return 0 }
	height := func(int, int, section.HeightmapKind) (int, bool) { return 0, false }
	source := section.NewChunkWithBounds(0, 0, 0, 128, generatedbiome.BiomeSoulSandValley)
	if err := source.UseFeatureRegion(reader, height, make([]uint32, 128*256)); err != nil {
		t.Fatal(err)
	}
	entry := &baseChunkEntry{structures: []*structureStartPlan{selected}}
	snapshot := &terrainSnapshot{structures: &structurePlanner{stages: 1 << 7}}
	region := &featureRegion{generator: &TerrainGenerator{}, snapshot: snapshot}
	region.neighbors[4] = entry
	region.placeStructureStage(source, 7)
	p := outside.position
	count := 0
	source.FeatureBorderWrites().ForChunk(int32(p.x>>4), int32(p.z>>4), func(section.BorderPosition, section.FeatureBorderWrite) { count++ })
	if count != 0 {
		t.Fatalf("source chunk wrote destination-owned bone at %+v", p)
	}
	target := section.NewChunkWithBounds(int32(p.x>>4), int32(p.z>>4), 0, 128, generatedbiome.BiomeSoulSandValley)
	if err := target.UseFeatureRegion(reader, height, make([]uint32, 128*256)); err != nil {
		t.Fatal(err)
	}
	target.SetFeatureStep(6, 0)
	target.SetFeatureBlock(p.x, p.y, p.z, block.DefaultStateID("minecraft:netherrack"))
	targetRegion := &featureRegion{generator: &TerrainGenerator{}, snapshot: snapshot, sourceX: target.X, sourceZ: target.Z}
	targetRegion.neighbors[4] = entry
	targetRegion.placeStructureStage(target, 7)
	if got := target.GetBlock(p.x, p.y, p.z); got != outside.state {
		t.Fatalf("stage 7 fossil did not replace stage 6 ore at %+v: %s", p, block.NameFromState(got))
	}
	target.SetFeatureStep(7, 0)
	target.SetFeatureBlock(p.x, p.y, p.z, block.DefaultStateID("minecraft:glowstone"))
	if got := target.GetBlock(p.x, p.y, p.z); got != block.DefaultStateID("minecraft:glowstone") {
		t.Fatal("later stage 7 feature did not replace fossil")
	}
}

// TestNetherFossilReloadIsolatesPlans verifies valid and failed datapack publications.
func TestNetherFossilReloadIsolatesPlans(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	vanilla := "../../../.mcdata/1.21.4/vanilla"
	if err := manager.Reload([]string{vanilla}); err != nil {
		t.Fatal(err)
	}
	g, err := runtime.NewTerrainGenerator("minecraft:nether", 0, generatedbiome.BiomeSoulSandValley)
	if err != nil {
		t.Fatal(err)
	}
	before, err := g.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	old := before.resources.netherFossils["minecraft:nether_fossil"]
	if old == nil || !old.biomes[generatedbiome.BiomeSoulSandValley] {
		t.Fatal("vanilla fossil biome missing")
	}
	pack := t.TempDir()
	if err := os.WriteFile(filepath.Join(pack, "pack.mcmeta"), []byte(`{"pack":{"pack_format":61,"description":"Nether fossil reload test"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(pack, "data", "minecraft", "worldgen", "structure", "nether_fossil.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	modified := []byte(`{"type":"minecraft:nether_fossil","biomes":"minecraft:nether_wastes","height":{"type":"minecraft:uniform","min_inclusive":{"absolute":32},"max_inclusive":{"below_top":2}},"step":"underground_decoration","terrain_adaptation":"beard_thin"}`)
	if err := os.WriteFile(path, modified, 0644); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{vanilla, pack}); err != nil {
		t.Fatal(err)
	}
	after, err := g.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	current := after.resources.netherFossils["minecraft:nether_fossil"]
	if before == after || old == current || current.biomes[generatedbiome.BiomeSoulSandValley] || !current.biomes[generatedbiome.BiomeNetherWastes] || !old.biomes[generatedbiome.BiomeSoulSandValley] {
		t.Fatal("fossil biome change leaked across snapshots")
	}
	if err := os.WriteFile(path, []byte(`{"type":"minecraft:nether_fossil","biomes":"minecraft:nether_wastes","height":{"type":"test:missing"},"step":"underground_decoration"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{vanilla, pack}); err == nil {
		t.Fatal("invalid structure definition published")
	}
	still, err := g.snapshot()
	if err != nil || still != after {
		t.Fatalf("failed reload replaced snapshot: %p %p %v", after, still, err)
	}
	if err := manager.Reload([]string{vanilla}); err != nil {
		t.Fatal(err)
	}
	restored, err := g.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if restored == after || !restored.resources.netherFossils["minecraft:nether_fossil"].biomes[generatedbiome.BiomeSoulSandValley] {
		t.Fatal("vanilla fossil did not restore")
	}
}

// TestNetherFossilParallelDeterminism compares independent sequential and concurrent chunk generation.
func TestNetherFossilParallelDeterminism(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	source := json.RawMessage(`{"type":"minecraft:fixed","biome":"minecraft:soul_sand_valley"}`)
	first, err := runtime.newTerrainGenerator("minecraft:nether", "minecraft:the_nether", source, 0, generatedbiome.BiomeSoulSandValley)
	if err != nil {
		t.Fatal(err)
	}
	second, err := runtime.newTerrainGenerator("minecraft:nether", "minecraft:the_nether", source, 0, generatedbiome.BiomeSoulSandValley)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := first.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	var site *structureStartPlan
	for x := int32(-12); x <= 12 && site == nil; x++ {
		for z := int32(-12); z <= 12; z++ {
			starts, err := snapshot.structures.plan(x, z)
			if err != nil {
				t.Fatal(err)
			}
			if len(starts) > 0 {
				site = starts[0]
				break
			}
		}
	}
	if site == nil {
		t.Fatal("no fossil site in checked area")
	}
	positions := [5][2]int32{{site.x, site.z}, {site.x - 1, site.z}, {site.x + 1, site.z}, {site.x, site.z - 1}, {site.x, site.z + 1}}
	var sequential, parallel [5]*section.Chunk
	for index, p := range positions {
		sequential[index], err = first.Generate(p[0], p[1])
		if err != nil {
			t.Fatal(err)
		}
	}
	var failures [5]error
	var group sync.WaitGroup
	for index, p := range positions {
		group.Add(1)
		go func(index int, p [2]int32) {
			defer group.Done()
			parallel[index], failures[index] = second.Generate(p[0], p[1])
		}(index, p)
	}
	group.Wait()
	for index, p := range positions {
		if failures[index] != nil {
			t.Fatal(failures[index])
		}
		a, b := sequential[index], parallel[index]
		if !reflect.DeepEqual(a.PersistentNBT(), b.PersistentNBT()) {
			t.Fatalf("structure NBT differs at %v", p)
		}
		for sectionIndex, one := range a.Sections {
			two := b.Sections[sectionIndex]
			for x := 0; x < 4; x++ {
				for y := 0; y < 4; y++ {
					for z := 0; z < 4; z++ {
						if one.GetBiome(x, y, z) != two.GetBiome(x, y, z) {
							t.Fatalf("biomes differ at %v section %d", p, sectionIndex)
						}
					}
				}
			}
		}
		for y := a.MinY(); y <= a.MaxY(); y++ {
			for lx := 0; lx < 16; lx++ {
				for lz := 0; lz < 16; lz++ {
					x, z := int(p[0])*16+lx, int(p[1])*16+lz
					if a.GetBlock(x, y, z) != b.GetBlock(x, y, z) || !reflect.DeepEqual(a.GetBlockEntity(int32(x), int32(y), int32(z)), b.GetBlockEntity(int32(x), int32(y), int32(z))) {
						t.Fatalf("chunk content differs at (%d,%d,%d)", x, y, z)
					}
				}
			}
		}
	}
}
