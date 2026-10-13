package worldgen

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/vitismc/vitis/internal/block"
	generatedbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/worldgen/density"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// netherFossilDefinition owns snapshot-specific biome selection and fourteen template variants.
type netherFossilDefinition struct {
	id          string
	biomes      map[int32]bool
	biomeNames  map[string]bool
	height      featureHeightProvider
	step, reach int
	adjustment  terrainAdjustment
	templates   [14]*structureTemplate
}

// compileNetherFossils validates definitions and resources before snapshot publication.
func compileNetherFossils(snapshot *datapack.Snapshot, resources *ResourceSet) error {
	resources.netherFossils = make(map[string]*netherFossilDefinition)
	ids := make([]string, 0, len(resources.resources["structure"]))
	for id := range resources.resources["structure"] {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		var value struct {
			Type, Step string
			Biomes     json.RawMessage
			Height     json.RawMessage
			Adjustment string `json:"terrain_adaptation"`
		}
		if err := json.Unmarshal(resources.resources["structure"][id], &value); err != nil {
			return err
		}
		if value.Type != "minecraft:nether_fossil" {
			continue
		}
		f := &netherFossilDefinition{id: id, biomes: make(map[int32]bool), biomeNames: make(map[string]bool)}
		var err error
		f.step, err = structureStep(value.Step)
		if err != nil {
			return fmt.Errorf("structure %s: %w", id, err)
		}
		f.adjustment, err = decodeTerrainAdjustment(value.Adjustment)
		if err != nil {
			return fmt.Errorf("structure %s: %w", id, err)
		}
		f.height, err = decodeFeatureHeight(value.Height)
		if err != nil {
			return fmt.Errorf("structure %s height: %w", id, err)
		}
		var entries []string
		if err := json.Unmarshal(value.Biomes, &entries); err != nil {
			var entry string
			if err := json.Unmarshal(value.Biomes, &entry); err != nil {
				return fmt.Errorf("structure %s biomes: %w", id, err)
			}
			entries = []string{entry}
		}
		if len(entries) == 0 {
			return fmt.Errorf("structure %s: missing biomes", id)
		}
		for _, entry := range entries {
			names := []string{entry}
			if strings.HasPrefix(entry, "#") {
				names, err = snapshot.ResolveTag("worldgen/biome", strings.TrimPrefix(entry, "#"))
				if err != nil {
					return fmt.Errorf("structure %s biomes: %w", id, err)
				}
			}
			for _, name := range names {
				name = normalizeResourceID(name)
				biome := generatedbiome.BiomeIDByName(name)
				if biome < 0 {
					return fmt.Errorf("structure %s: unknown biome %s", id, name)
				}
				f.biomes[biome] = true
				f.biomeNames[name] = true
			}
		}
		for index := range f.templates {
			f.templates[index], err = resources.templates.load(fmt.Sprintf("minecraft:nether_fossils/fossil_%d", index+1))
			if err != nil {
				return err
			}
			size := f.templates[index].size
			margin := 0
			if f.adjustment != terrainAdjustmentNone {
				margin = 12
			}
			f.reach = max(f.reach, min(8, (max(size.x, size.z)-1+margin+15)/16))
		}
		resources.netherFossils[id] = f
	}
	return nil
}

// plan chooses a site from unmodified noise columns, then validates the biome at the generation point.
func (f *netherFossilDefinition) plan(g *TerrainGenerator, snapshot *terrainSnapshot, x, z int32) (*structureStartPlan, error) {
	return f.planWithColumn(g.seed, x, z, max(snapshot.minY, snapshot.settings.Noise.MinY), min(snapshot.height, snapshot.settings.Noise.Height), snapshot.settings.SeaLevel, snapshot.biomes, func(x, z int) noiseColumn { return g.structureNoiseColumn(snapshot, x, z) })
}

// noiseColumn contains the exact generated vertical interval without surface, carvers or structures.
type noiseColumn struct {
	minY   int
	blocks []int32
}

// block returns air beyond the generator's vertical range.
func (c noiseColumn) block(y int) int32 {
	index := y - c.minY
	if index < 0 || index >= len(c.blocks) {
		return 0
	}
	return c.blocks[index]
}

// planWithColumn preserves coordinate, height, rotation and template selection RNG order.
func (f *netherFossilDefinition) planWithColumn(seed int64, chunkX, chunkZ int32, minY, height, seaLevel int, biomes biomeSource, column func(int, int) noiseColumn) (*structureStartPlan, error) {
	random := worldrandom.NewWorldgen(worldrandom.NewLegacy(0))
	random.SetLargeFeatureSeed(seed, chunkX, chunkZ)
	x, z := int(chunkX)*16+int(random.NextIntBound(16)), int(chunkZ)*16+int(random.NextIntBound(16))
	y := f.height.sample(random, minY, height)
	base := column(x, z)
	for y > seaLevel {
		upper := base.block(y)
		y--
		lower := base.block(y)
		if block.IsAir(upper) && (block.NameFromState(lower) == "minecraft:soul_sand" || featureFullFace(lower, featureUp, false)) {
			break
		}
	}
	if y <= seaLevel || !f.biomes[biomes.sample(x>>2, y>>2, z>>2)] {
		return nil, nil
	}
	transform := templateTransform{rotation: templateRotation(random.NextIntBound(4))}
	index := random.NextIntBound(14)
	template := f.templates[index]
	if template == nil {
		return nil, fmt.Errorf("structure %s: missing fossil template %d", f.id, index+1)
	}
	origin := blockPosition{x, y, z}
	low, high := template.bounds(origin, transform)
	box := structureBox{low, high}
	paletteRandom := worldrandom.NewLegacy(worldrandom.PositionSeed(int32(x), int32(y), int32(z)))
	palette := template.palettes[paletteRandom.NextIntBound(int32(len(template.palettes)))]
	piece := structurePiecePlan{box: box}
	for _, item := range palette {
		name := block.NameFromState(item.state)
		if name == "minecraft:air" || name == "minecraft:structure_block" || name == "minecraft:structure_void" {
			continue
		}
		p := transform.position(item.position)
		state, err := transform.state(item.state)
		if err != nil {
			return nil, err
		}
		piece.blocks = append(piece.blocks, templateBlock{position: blockPosition{p.x + x, p.y + y, p.z + z}, state: state, data: item.data})
	}
	rotations := [4]string{"NONE", "CLOCKWISE_90", "CLOCKWISE_180", "COUNTERCLOCKWISE_90"}
	piece.nbt = structurePieceTag("minecraft:nefos", box).PutString("Template", fmt.Sprintf("minecraft:nether_fossils/fossil_%d", index+1)).PutInt("TPX", int32(x)).PutInt("TPY", int32(y)).PutInt("TPZ", int32(z)).PutString("Rot", rotations[transform.rotation])
	return &structureStartPlan{id: f.id, x: chunkX, z: chunkZ, step: f.step, adjustment: f.adjustment, box: box, pieces: []structurePiecePlan{piece}}, nil
}

// structureNoiseColumn samples the same interpolation graph used by terrain without adaptation.
func (g *TerrainGenerator) structureNoiseColumn(snapshot *terrainSnapshot, x, z int) noiseColumn {
	start := max(snapshot.minY, snapshot.settings.Noise.MinY)
	end := min(snapshot.minY+snapshot.height, snapshot.settings.Noise.MinY+snapshot.settings.Noise.Height)
	result := noiseColumn{minY: start, blocks: make([]int32, max(0, end-start))}
	if end <= start {
		return result
	}
	var evaluation *density.Evaluation
	if cached := snapshot.evaluationPool.Get(); cached != nil {
		evaluation = cached.(*density.Evaluation)
		evaluation.ResetChunk((x>>4)*16, start, (z>>4)*16, end-start)
	} else {
		evaluation = density.NewChunkEvaluation((x>>4)*16, start, (z>>4)*16, end-start)
	}
	defer snapshot.evaluationPool.Put(evaluation)
	var aquifer *chunkAquifer
	if cached := snapshot.aquiferPool.Get(); cached != nil {
		aquifer = cached.(*chunkAquifer)
		aquifer.reset()
	} else {
		aquifer = newChunkAquifer(snapshot.aquifer)
	}
	defer snapshot.aquiferPool.Put(aquifer)
	var scratch worldrandom.PositionalScratch
	for y := end - 1; y >= start; y-- {
		position := density.Position{X: x, Y: y, Z: z, Evaluation: evaluation}
		state := aquifer.substance(position, snapshot.finalDensity.Sample(position))
		if state < 0 {
			if ore, ok := snapshot.oreVeins.stateWithScratch(position, &scratch); ok {
				state = ore
			} else {
				state = snapshot.blockState
			}
		}
		result.blocks[y-start] = state
	}
	return result
}
