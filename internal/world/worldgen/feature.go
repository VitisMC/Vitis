package worldgen

import (
	"encoding/json"
	"fmt"
	"math"
	"math/bits"
	"sort"
	"sync"

	"github.com/vitismc/vitis/internal/block"
	generatedbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	"github.com/vitismc/vitis/internal/world/worldgen/noise"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type oreTarget struct {
	states map[int32]struct{}
	result int32
}

type configuredOre struct {
	scattered       bool
	size            int
	discard         float32
	targets         []oreTarget
	targetBase      int32
	resolvedTargets []int32
}

// oreScratch owns geometry and visitation without boxing slice headers on every placement.
type oreScratch struct {
	spheres [64][4]float64
	visited []uint64
}

var oreScratchPool = sync.Pool{New: func() any { return &oreScratch{visited: make([]uint64, 256)} }}

type configuredFeature interface {
	placeFeature(*featureSystem, *section.Chunk, worldrandom.Source, blockPosition) bool
}

type intProvider interface{ sample(worldrandom.Source) int }

type constantInt int

func (p constantInt) sample(worldrandom.Source) int { return int(p) }

type uniformInt struct{ min, max int }

func (p uniformInt) sample(random worldrandom.Source) int {
	return p.min + int(random.NextIntBound(int32(p.max-p.min+1)))
}

type biasedInt struct{ min, max int }

func (p biasedInt) sample(random worldrandom.Source) int {
	return p.min + int(random.NextIntBound(random.NextIntBound(int32(p.max-p.min+1))+1))
}

type clampedInt struct {
	source   intProvider
	min, max int
}

type clampedNormalInt struct {
	mean, deviation float32
	min, max        int
}

// sample preserves the reference float conversion before truncating to an integer.
func (p clampedNormalInt) sample(random worldrandom.Source) int {
	value := p.mean + float32(random.NextGaussian())*p.deviation
	return int(max(float32(p.min), min(float32(p.max), value)))
}

func (p clampedInt) sample(random worldrandom.Source) int {
	return max(p.min, min(p.max, p.source.sample(random)))
}

type weightedIntEntry struct {
	provider intProvider
	weight   int
}

type weightedInt struct {
	entries []weightedIntEntry
	total   int
}

func (p *weightedInt) sample(random worldrandom.Source) int {
	value := int(random.NextIntBound(int32(p.total)))
	for _, entry := range p.entries {
		value -= entry.weight
		if value < 0 {
			return entry.provider.sample(random)
		}
	}
	return p.entries[len(p.entries)-1].provider.sample(random)
}

type featurePlacement struct {
	kind                       string
	count                      intProvider
	chance                     int
	height                     featureHeightProvider
	predicate                  blockPredicate
	heightmap                  string
	noiseLevel                 float64
	belowNoise, aboveNoise     int
	target, allowed            blockPredicate
	direction, maxSteps        int
	maxWaterDepth              int
	xzSpread, ySpread          intProvider
	noiseFactor, noiseOffset   float64
	noiseRatio                 int
	thresholdMin, thresholdMax int
	fixedPositions             []blockPosition
}

type featureHeightProvider struct {
	kind           string
	min, max       verticalAnchor
	plateau, inner int
}

func (p featureHeightProvider) sample(random worldrandom.Source, minY, height int) int {
	minimum, maximum := p.min.resolve(minY, height), p.max.resolve(minY, height)
	if minimum >= maximum {
		return minimum
	}
	span := maximum - minimum
	if p.kind == "minecraft:trapezoid" && p.plateau < span {
		lower := (span - p.plateau) / 2
		upper := span - lower
		return minimum + int(random.NextIntBound(int32(upper+1))) + int(random.NextIntBound(int32(lower+1)))
	}
	if p.kind == "minecraft:biased_to_bottom" {
		bound := span - p.inner + 1
		if bound <= 0 {
			return minimum
		}
		return minimum + int(random.NextIntBound(random.NextIntBound(int32(bound))+int32(p.inner)))
	}
	if p.kind == "minecraft:very_biased_to_bottom" {
		if span-p.inner+1 <= 0 {
			return minimum
		}
		upper := minimum + p.inner + int(random.NextIntBound(int32(maximum-(minimum+p.inner)+1)))
		middle := minimum + int(random.NextIntBound(int32(upper-minimum)))
		return minimum + int(random.NextIntBound(int32(middle-1+p.inner-minimum+1)))
	}
	return minimum + int(random.NextIntBound(int32(span+1)))
}

type placedFeature struct {
	feature       configuredFeature
	placement     []featurePlacement
	featureID     string
	allowedBiomes *[generatedbiome.BiomeCount]bool
}

type featureStep struct {
	ids              []string
	index            map[string]int
	allBiomeFiltered bool
}

type featureSystemResources struct {
	configured    map[string]configuredFeature
	placed        map[string]*placedFeature
	steps         []featureStep
	biomeFeatures map[string][][]string
}

type featureSystem struct {
	resources    *featureSystemResources
	seed         int64
	minY, height int
	biomes       biomeSource
	biomeNoise   *noise.PerlinSimplex
	seaLevel     int

	biomeCacheMu    sync.RWMutex
	biomeCache      map[int64][]string
	biomeCacheKeys  []int64
	biomeCacheNext  int
	biomeSamplePool sync.Pool
	biomeMemoPool   sync.Pool
}

const featureBiomeCacheCapacity = 4096

func compileFeatureResources(snapshot *datapack.Snapshot, resources *ResourceSet) (*featureSystemResources, error) {
	result := &featureSystemResources{placed: make(map[string]*placedFeature), biomeFeatures: make(map[string][][]string)}
	configured := make(map[string]configuredFeature)
	result.configured = configured
	for id, raw := range resources.resources["configured_feature"] {
		feature, ok, err := decodeConfiguredFeature(snapshot, raw, resources.templates)
		if err != nil {
			return nil, fmt.Errorf("configured feature %s: %w", id, err)
		}
		if !ok {
			continue
		}
		configured[id] = feature
	}
	for placedID, placedRaw := range resources.resources["placed_feature"] {
		var header struct {
			Feature string `json:"feature"`
		}
		if err := json.Unmarshal(placedRaw, &header); err != nil {
			return nil, fmt.Errorf("placed feature %s: %w", placedID, err)
		}
		feature := configured[normalizeResourceID(header.Feature)]
		if feature == nil {
			continue
		}
		placed, err := decodePlacedFeature(placedRaw, feature)
		if err != nil {
			return nil, fmt.Errorf("placed feature %s: %w", placedID, err)
		}
		result.placed[placedID] = placed
		placed.featureID = placedID
		placed.allowedBiomes = new([generatedbiome.BiomeCount]bool)
	}
	orderedBiomes := make([]string, 0, len(resources.resources["biome"]))
	seen := make(map[string]bool)
	for _, biome := range generatedbiome.Biomes {
		if _, ok := resources.resources["biome"][biome.Name]; ok && !seen[biome.Name] {
			orderedBiomes = append(orderedBiomes, biome.Name)
			seen[biome.Name] = true
		}
	}
	var custom []string
	for id := range resources.resources["biome"] {
		if !seen[id] {
			custom = append(custom, id)
		}
	}
	sort.Strings(custom)
	orderedBiomes = append(orderedBiomes, custom...)
	maxSteps := 0
	for _, id := range orderedBiomes {
		var definition struct {
			Features [][]string `json:"features"`
		}
		if err := json.Unmarshal(resources.resources["biome"][id], &definition); err != nil {
			return nil, fmt.Errorf("biome %s features: %w", id, err)
		}
		for step := range definition.Features {
			for i := range definition.Features[step] {
				definition.Features[step][i] = normalizeResourceID(definition.Features[step][i])
				if _, ok := resources.resources["placed_feature"][definition.Features[step][i]]; !ok {
					return nil, fmt.Errorf("biome %s: unknown placed feature %s", id, definition.Features[step][i])
				}
			}
		}
		result.biomeFeatures[id] = definition.Features
		if biomeID := generatedbiome.BiomeIDByName(id); biomeID >= 0 && biomeID < generatedbiome.BiomeCount {
			for _, step := range definition.Features {
				for _, featureID := range step {
					if placed := result.placed[featureID]; placed != nil {
						placed.allowedBiomes[biomeID] = true
					}
				}
			}
		}
		if len(definition.Features) > maxSteps {
			maxSteps = len(definition.Features)
		}
	}
	steps, err := sortFeatureSteps(orderedBiomes, result.biomeFeatures, maxSteps)
	if err != nil {
		return nil, err
	}
	result.steps = steps
	for step := range result.steps {
		result.steps[step].allBiomeFiltered = true
		for _, id := range result.steps[step].ids {
			placed := result.placed[id]
			if placed != nil && !placed.hasPlacement("minecraft:biome") {
				result.steps[step].allBiomeFiltered = false
				break
			}
		}
	}
	return result, nil
}

// forBiomeOrder compiles feature seed indices for one biome source.
func (r *featureSystemResources) forBiomeOrder(biomes []string) (*featureSystemResources, error) {
	if r == nil || len(biomes) == 0 {
		return r, nil
	}
	maxSteps := 0
	for _, biome := range biomes {
		if _, ok := r.biomeFeatures[biome]; !ok {
			return nil, fmt.Errorf("feature order: unknown biome %s", biome)
		}
		if count := len(r.biomeFeatures[biome]); count > maxSteps {
			maxSteps = count
		}
	}
	steps, err := sortFeatureSteps(biomes, r.biomeFeatures, maxSteps)
	if err != nil {
		return nil, err
	}
	for step := range steps {
		steps[step].allBiomeFiltered = true
		for _, id := range steps[step].ids {
			placed := r.placed[id]
			if placed != nil && !placed.hasPlacement("minecraft:biome") {
				steps[step].allBiomeFiltered = false
				break
			}
		}
	}
	return &featureSystemResources{placed: r.placed, biomeFeatures: r.biomeFeatures, steps: steps}, nil
}

type featureNode struct {
	step int
	id   string
}

// sortFeatureSteps reproduces the dependency graph order used for feature seed indices.
func sortFeatureSteps(biomes []string, features map[string][][]string, stepCount int) ([]featureStep, error) {
	if stepCount < 0 || stepCount > 1<<15 {
		return nil, fmt.Errorf("feature step count must be in [0, 32768]")
	}
	first := make(map[string]int)
	edges := make(map[featureNode]map[featureNode]struct{})
	for _, biome := range biomes {
		var previous featureNode
		hasPrevious := false
		for step, ids := range features[biome] {
			for _, id := range ids {
				if _, ok := first[id]; !ok {
					first[id] = len(first)
				}
				current := featureNode{step: step, id: id}
				if edges[current] == nil {
					edges[current] = make(map[featureNode]struct{})
				}
				if hasPrevious && previous != current {
					edges[previous][current] = struct{}{}
				}
				previous, hasPrevious = current, true
			}
		}
	}
	less := func(left, right featureNode) bool {
		if left.step != right.step {
			return left.step < right.step
		}
		return first[left.id] < first[right.id]
	}
	nodes := make([]featureNode, 0, len(edges))
	for node := range edges {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return less(nodes[i], nodes[j]) })
	state := make(map[featureNode]uint8, len(nodes))
	postorder := make([]featureNode, 0, len(nodes))
	var visit func(featureNode) error
	visit = func(node featureNode) error {
		switch state[node] {
		case 1:
			return fmt.Errorf("feature step %d has conflicting biome feature order", node.step)
		case 2:
			return nil
		}
		state[node] = 1
		neighbors := make([]featureNode, 0, len(edges[node]))
		for next := range edges[node] {
			neighbors = append(neighbors, next)
		}
		sort.Slice(neighbors, func(i, j int) bool { return less(neighbors[i], neighbors[j]) })
		for _, next := range neighbors {
			if err := visit(next); err != nil {
				return err
			}
		}
		state[node] = 2
		postorder = append(postorder, node)
		return nil
	}
	for _, node := range nodes {
		if err := visit(node); err != nil {
			return nil, err
		}
	}
	result := make([]featureStep, stepCount)
	for i := range result {
		result[i].index = make(map[string]int)
	}
	for i := len(postorder) - 1; i >= 0; i-- {
		node := postorder[i]
		if node.step >= stepCount {
			continue
		}
		step := &result[node.step]
		if len(step.ids) >= (1<<16)-2 {
			return nil, fmt.Errorf("feature step %d exceeds 65534 entries", node.step)
		}
		step.index[node.id] = len(step.ids)
		step.ids = append(step.ids, node.id)
	}
	return result, nil
}

// sortFeatureStep retains the single-step helper used by feature ordering tests.
func sortFeatureStep(biomes []string, features map[string][][]string, step int) (featureStep, error) {
	steps, err := sortFeatureSteps(biomes, features, step+1)
	if err != nil {
		return featureStep{}, err
	}
	return steps[step], nil
}

func decodeConfiguredOre(snapshot *datapack.Snapshot, raw json.RawMessage) (*configuredOre, bool, error) {
	var header struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return nil, false, err
	}
	if header.Type != "minecraft:ore" && header.Type != "minecraft:scattered_ore" {
		return nil, false, nil
	}
	var value struct {
		Type   string `json:"type"`
		Config struct {
			Size    int     `json:"size"`
			Discard float32 `json:"discard_chance_on_air_exposure"`
			Targets []struct {
				Target struct {
					Type  string `json:"predicate_type"`
					Tag   string `json:"tag"`
					Block string `json:"block"`
				} `json:"target"`
				State BlockState `json:"state"`
			} `json:"targets"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, false, err
	}
	if value.Config.Size < 0 || value.Config.Size > 64 || value.Config.Discard < 0 || value.Config.Discard > 1 || len(value.Config.Targets) == 0 {
		return nil, true, fmt.Errorf("invalid ore configuration")
	}
	result := &configuredOre{scattered: value.Type == "minecraft:scattered_ore", size: value.Config.Size, discard: value.Config.Discard}
	for _, target := range value.Config.Targets {
		var names []string
		switch target.Target.Type {
		case "minecraft:tag_match":
			resolved, err := snapshot.ResolveTag("block", target.Target.Tag)
			if err != nil {
				return nil, true, err
			}
			names = resolved
		case "minecraft:block_match":
			names = []string{normalizeResourceID(target.Target.Block)}
		default:
			return nil, true, fmt.Errorf("unsupported rule test %q", target.Target.Type)
		}
		states := make(map[int32]struct{})
		for _, name := range names {
			info := block.InfoByName(normalizeResourceID(name))
			if info == nil {
				return nil, true, fmt.Errorf("unknown target block %s", name)
			}
			for state := info.MinStateID; state <= info.MaxStateID; state++ {
				states[state] = struct{}{}
			}
		}
		state, err := block.BlockStateFromProperties(target.State.Name, target.State.Properties)
		if err != nil {
			return nil, true, err
		}
		result.targets = append(result.targets, oreTarget{states: states, result: state.ID()})
	}
	minimum, maximum := int32(math.MaxInt32), int32(-1)
	for _, target := range result.targets {
		for state := range target.states {
			minimum = min(minimum, state)
			maximum = max(maximum, state)
		}
	}
	if maximum >= minimum {
		result.targetBase = minimum
		result.resolvedTargets = make([]int32, maximum-minimum+1)
		for index := range result.resolvedTargets {
			result.resolvedTargets[index] = -1
		}
		for _, target := range result.targets {
			for state := range target.states {
				index := state - minimum
				if result.resolvedTargets[index] < 0 {
					result.resolvedTargets[index] = target.result
				}
			}
		}
	}
	return result, true, nil
}

// decodeConfiguredFeature compiles a definition using optional snapshot-shared template resources.
func decodeConfiguredFeature(snapshot *datapack.Snapshot, raw json.RawMessage, templates ...*structureTemplates) (configuredFeature, bool, error) {
	var header struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return nil, false, err
	}
	if header.Type == "minecraft:fossil" {
		var store *structureTemplates
		if len(templates) != 0 {
			store = templates[0]
		}
		if store == nil {
			store = newStructureTemplates(snapshot)
		}
		feature, err := decodeFossilFeature(snapshot, store, raw)
		return feature, true, err
	}
	if feature, ok, err := decodeBasicConfiguredFeature(snapshot, header.Type, raw); ok || err != nil {
		return feature, ok, err
	}
	return decodeConfiguredOre(snapshot, raw)
}

func decodePlacedFeature(raw json.RawMessage, feature configuredFeature) (*placedFeature, error) {
	var value struct {
		Placement []json.RawMessage `json:"placement"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	result := &placedFeature{feature: feature}
	for _, rawModifier := range value.Placement {
		var header struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(rawModifier, &header); err != nil {
			return nil, err
		}
		modifier := featurePlacement{kind: header.Type}
		switch header.Type {
		case "minecraft:count":
			var data struct {
				Count json.RawMessage `json:"count"`
			}
			_ = json.Unmarshal(rawModifier, &data)
			provider, err := decodeIntProvider(data.Count)
			if err != nil {
				return nil, fmt.Errorf("count: %w", err)
			}
			modifier.count = provider
		case "minecraft:rarity_filter":
			var data struct {
				Chance int `json:"chance"`
			}
			_ = json.Unmarshal(rawModifier, &data)
			if data.Chance <= 0 {
				return nil, fmt.Errorf("invalid rarity chance")
			}
			modifier.chance = data.Chance
		case "minecraft:height_range":
			var data struct {
				Height json.RawMessage `json:"height"`
			}
			_ = json.Unmarshal(rawModifier, &data)
			height, err := decodeFeatureHeight(data.Height)
			if err != nil {
				return nil, err
			}
			modifier.height = height
		case "minecraft:block_predicate_filter":
			var data struct {
				Predicate json.RawMessage `json:"predicate"`
			}
			_ = json.Unmarshal(rawModifier, &data)
			predicate, err := decodeBlockPredicate(data.Predicate)
			if err != nil {
				return nil, err
			}
			modifier.predicate = predicate
		case "minecraft:heightmap":
			var data struct {
				Heightmap string `json:"heightmap"`
			}
			_ = json.Unmarshal(rawModifier, &data)
			if data.Heightmap == "" {
				return nil, fmt.Errorf("missing heightmap")
			}
			modifier.heightmap = data.Heightmap
		case "minecraft:noise_threshold_count":
			var data struct {
				NoiseLevel float64 `json:"noise_level"`
				Below      int     `json:"below_noise"`
				Above      int     `json:"above_noise"`
			}
			_ = json.Unmarshal(rawModifier, &data)
			modifier.noiseLevel, modifier.belowNoise, modifier.aboveNoise = data.NoiseLevel, data.Below, data.Above
		case "minecraft:environment_scan":
			var data struct {
				Direction string          `json:"direction_of_search"`
				Target    json.RawMessage `json:"target_condition"`
				Allowed   json.RawMessage `json:"allowed_search_condition"`
				MaxSteps  int             `json:"max_steps"`
			}
			_ = json.Unmarshal(rawModifier, &data)
			if data.Direction == "up" {
				modifier.direction = 1
			} else if data.Direction == "down" {
				modifier.direction = -1
			} else {
				return nil, fmt.Errorf("invalid scan direction %q", data.Direction)
			}
			modifier.maxSteps = data.MaxSteps
			var err error
			modifier.target, err = decodeBlockPredicate(data.Target)
			if err != nil {
				return nil, err
			}
			modifier.allowed = alwaysBlockPredicate
			if len(data.Allowed) > 0 {
				modifier.allowed, err = decodeBlockPredicate(data.Allowed)
				if err != nil {
					return nil, err
				}
			}
		case "minecraft:random_offset":
			var data struct {
				XZ json.RawMessage `json:"xz_spread"`
				Y  json.RawMessage `json:"y_spread"`
			}
			_ = json.Unmarshal(rawModifier, &data)
			var err error
			modifier.xzSpread, err = decodeIntProviderRange(data.XZ, -16, 16)
			if err != nil {
				return nil, err
			}
			modifier.ySpread, err = decodeIntProviderRange(data.Y, -16, 16)
			if err != nil {
				return nil, err
			}
		case "minecraft:surface_water_depth_filter":
			var data struct {
				Max int `json:"max_water_depth"`
			}
			_ = json.Unmarshal(rawModifier, &data)
			modifier.maxWaterDepth = data.Max
		case "minecraft:noise_based_count":
			var data struct {
				Ratio  int     `json:"noise_to_count_ratio"`
				Factor float64 `json:"noise_factor"`
				Offset float64 `json:"noise_offset"`
			}
			_ = json.Unmarshal(rawModifier, &data)
			if data.Factor == 0 {
				return nil, fmt.Errorf("zero noise factor")
			}
			modifier.noiseRatio, modifier.noiseFactor, modifier.noiseOffset = data.Ratio, data.Factor, data.Offset
		case "minecraft:surface_relative_threshold_filter":
			var data struct {
				Heightmap string `json:"heightmap"`
				Min       *int   `json:"min_inclusive"`
				Max       *int   `json:"max_inclusive"`
			}
			_ = json.Unmarshal(rawModifier, &data)
			modifier.heightmap = data.Heightmap
			modifier.thresholdMin, modifier.thresholdMax = -int(^uint(0)>>1)-1, int(^uint(0)>>1)
			if data.Min != nil {
				modifier.thresholdMin = *data.Min
			}
			if data.Max != nil {
				modifier.thresholdMax = *data.Max
			}
		case "minecraft:fixed_placement":
			var data struct {
				Positions [][]int `json:"positions"`
			}
			_ = json.Unmarshal(rawModifier, &data)
			for _, position := range data.Positions {
				if len(position) != 3 {
					return nil, fmt.Errorf("invalid fixed position")
				}
				modifier.fixedPositions = append(modifier.fixedPositions, blockPosition{x: position[0], y: position[1], z: position[2]})
			}
		case "minecraft:count_on_every_layer":
			var data struct {
				Count json.RawMessage `json:"count"`
			}
			_ = json.Unmarshal(rawModifier, &data)
			provider, err := decodeIntProvider(data.Count)
			if err != nil {
				return nil, err
			}
			modifier.count = provider
		case "minecraft:in_square", "minecraft:biome":
		default:
			return nil, fmt.Errorf("unsupported ore placement %q", header.Type)
		}
		result.placement = append(result.placement, modifier)
	}
	return result, nil
}

func decodeIntProvider(raw json.RawMessage) (intProvider, error) {
	return decodeIntProviderRange(raw, 0, 256)
}

func decodeIntProviderRange(raw json.RawMessage, minimum, maximum int) (intProvider, error) {
	var constant int
	if err := json.Unmarshal(raw, &constant); err == nil {
		if constant < minimum || constant > maximum {
			return nil, fmt.Errorf("constant out of range")
		}
		return constantInt(constant), nil
	}
	var value struct {
		Type         string `json:"type"`
		Min, Max     int
		MinInclusive int `json:"min_inclusive"`
		MaxInclusive int `json:"max_inclusive"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	if value.Type == "minecraft:weighted_list" {
		var list struct {
			Distribution []struct {
				Data   json.RawMessage `json:"data"`
				Weight int             `json:"weight"`
			} `json:"distribution"`
		}
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, err
		}
		provider := &weightedInt{}
		for _, entry := range list.Distribution {
			child, err := decodeIntProviderRange(entry.Data, minimum, maximum)
			if err != nil {
				return nil, err
			}
			if entry.Weight <= 0 {
				return nil, fmt.Errorf("invalid weighted int weight")
			}
			provider.entries = append(provider.entries, weightedIntEntry{provider: child, weight: entry.Weight})
			provider.total += entry.Weight
		}
		if provider.total == 0 {
			return nil, fmt.Errorf("empty weighted int provider")
		}
		return provider, nil
	}
	if value.Type == "minecraft:clamped" {
		if value.MinInclusive < minimum || value.MaxInclusive < value.MinInclusive || value.MaxInclusive > maximum {
			return nil, fmt.Errorf("clamped int bounds out of range")
		}
		var data struct {
			Source json.RawMessage `json:"source"`
		}
		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, err
		}
		source, err := decodeIntProviderRange(data.Source, -1000000000, 1000000000)
		if err != nil {
			return nil, err
		}
		return clampedInt{source: source, min: value.MinInclusive, max: value.MaxInclusive}, nil
	}
	if value.Type == "minecraft:clamped_normal" {
		var parameters struct {
			Mean      float32 `json:"mean"`
			Deviation float32 `json:"deviation"`
		}
		if err := json.Unmarshal(raw, &parameters); err != nil {
			return nil, err
		}
		if value.MinInclusive < minimum || value.MaxInclusive < value.MinInclusive || value.MaxInclusive > maximum ||
			math.IsNaN(float64(parameters.Mean)) || math.IsInf(float64(parameters.Mean), 0) ||
			math.IsNaN(float64(parameters.Deviation)) || math.IsInf(float64(parameters.Deviation), 0) {
			return nil, fmt.Errorf("invalid clamped normal int provider")
		}
		return clampedNormalInt{parameters.Mean, parameters.Deviation, value.MinInclusive, value.MaxInclusive}, nil
	}
	if value.MinInclusive < minimum || value.MaxInclusive < value.MinInclusive || value.MaxInclusive > maximum {
		return nil, fmt.Errorf("unsupported int provider %q", value.Type)
	}
	switch value.Type {
	case "minecraft:uniform":
		return uniformInt{min: value.MinInclusive, max: value.MaxInclusive}, nil
	case "minecraft:biased_to_bottom":
		return biasedInt{min: value.MinInclusive, max: value.MaxInclusive}, nil
	default:
		return nil, fmt.Errorf("unsupported int provider %q", value.Type)
	}
}

func decodeFeatureHeight(raw json.RawMessage) (featureHeightProvider, error) {
	var value struct {
		Type    string          `json:"type"`
		Min     json.RawMessage `json:"min_inclusive"`
		Max     json.RawMessage `json:"max_inclusive"`
		Plateau int             `json:"plateau"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return featureHeightProvider{}, err
	}
	if value.Type != "minecraft:uniform" && value.Type != "minecraft:trapezoid" && value.Type != "minecraft:biased_to_bottom" && value.Type != "minecraft:very_biased_to_bottom" {
		return featureHeightProvider{}, fmt.Errorf("unsupported height provider %q", value.Type)
	}
	minimum, err := decodeVerticalAnchor(value.Min)
	if err != nil {
		return featureHeightProvider{}, err
	}
	maximum, err := decodeVerticalAnchor(value.Max)
	if err != nil {
		return featureHeightProvider{}, err
	}
	if value.Plateau < 0 {
		return featureHeightProvider{}, fmt.Errorf("negative plateau")
	}
	var inner struct {
		Inner int `json:"inner"`
	}
	_ = json.Unmarshal(raw, &inner)
	if (value.Type == "minecraft:biased_to_bottom" || value.Type == "minecraft:very_biased_to_bottom") && inner.Inner == 0 {
		inner.Inner = 1
	}
	return featureHeightProvider{kind: value.Type, min: minimum, max: maximum, plateau: value.Plateau, inner: inner.Inner}, nil
}

func newFeatureSystem(resources *featureSystemResources, seed int64, minY, height, seaLevel int, biomes biomeSource) *featureSystem {
	biomeNoise, _ := noise.NewPerlinSimplex(worldrandom.NewWorldgen(worldrandom.NewLegacy(2345)), []int{0})
	return &featureSystem{
		resources: resources, seed: seed, minY: minY, height: height, seaLevel: seaLevel,
		biomes: biomes, biomeNoise: biomeNoise,
		biomeCache:     make(map[int64][]string, featureBiomeCacheCapacity),
		biomeCacheKeys: make([]int64, 0, featureBiomeCacheCapacity),
	}
}

func (s *featureSystem) apply(chunk *section.Chunk, chunkX, chunkZ int32, biomes biomeSource) {
	s.applyWithStructures(chunk, chunkX, chunkZ, biomes, nil)
}

// applyWithStructures runs structure placement immediately before the features of each decoration step.
func (s *featureSystem) applyWithStructures(chunk *section.Chunk, chunkX, chunkZ int32, biomes biomeSource, beforeStep func(int)) {
	if s == nil || s.resources == nil || len(s.resources.placed) == 0 {
		if beforeStep != nil {
			for step := 0; step < 11; step++ {
				beforeStep(step)
			}
		}
		return
	}
	random := worldrandom.NewWorldgen(worldrandom.NewXoroshiro(0))
	originX, originZ := int(chunkX)*16, int(chunkZ)*16
	decorationSeed := random.SetDecorationSeed(s.seed, int32(originX), int32(originZ))
	var biomeCache *featureBiomeMemo
	if cached := s.biomeMemoPool.Get(); cached != nil {
		biomeCache = cached.(*featureBiomeMemo)
	} else {
		biomeCache = &featureBiomeMemo{}
	}
	biomeCache.reset()
	defer s.biomeMemoPool.Put(biomeCache)
	if biomes == nil {
		biomes = s.biomes
	}
	var biomeNames map[string]struct{}
	for stepIndex, step := range s.resources.steps {
		if beforeStep != nil {
			beforeStep(stepIndex)
		}
		if step.allBiomeFiltered {
			for index, id := range step.ids {
				placed := s.resources.placed[id]
				if placed == nil {
					continue
				}
				random.SetFeatureSeed(decorationSeed, int32(index), int32(stepIndex))
				chunk.SetFeatureStep(stepIndex, index)
				chunk.SetFeatureWriteRadius(placed.writeRadius())
				placed.place(s, chunk, id, random, blockPosition{x: originX, y: s.minY, z: originZ}, biomeCache, biomes)
			}
			continue
		}
		if biomeNames == nil {
			biomeNames = s.chunkBiomeNames(chunkX, chunkZ)
		}
		wanted := make(map[int]struct{})
		for biome := range biomeNames {
			steps := s.resources.biomeFeatures[biome]
			if stepIndex >= len(steps) {
				continue
			}
			for _, id := range steps[stepIndex] {
				if index, ok := step.index[id]; ok {
					wanted[index] = struct{}{}
				}
			}
		}
		indices := make([]int, 0, len(wanted))
		for index := range wanted {
			indices = append(indices, index)
		}
		sort.Ints(indices)
		for _, index := range indices {
			id := step.ids[index]
			placed := s.resources.placed[id]
			if placed == nil {
				continue
			}
			random.SetFeatureSeed(decorationSeed, int32(index), int32(stepIndex))
			chunk.SetFeatureStep(stepIndex, index)
			chunk.SetFeatureWriteRadius(placed.writeRadius())
			placed.place(s, chunk, id, random, blockPosition{x: originX, y: s.minY, z: originZ}, biomeCache, biomes)
		}
	}
	if beforeStep != nil {
		for step := len(s.resources.steps); step < 11; step++ {
			beforeStep(step)
		}
	}
}

// writeRadius retains the existing border limit for ordinary decorations.
func (p *placedFeature) writeRadius() int {
	if large, ok := p.feature.(*largeDripstoneFeature); ok {
		return large.chunkRadius
	}
	if geode, ok := p.feature.(*geodeFeature); ok {
		return geode.chunkRadius
	}
	return 1
}

func (p *placedFeature) hasPlacement(kind string) bool {
	for _, placement := range p.placement {
		if placement.kind == kind {
			return true
		}
	}
	return false
}

func (s *featureSystem) chunkBiomeNames(chunkX, chunkZ int32) map[string]struct{} {
	result := make(map[string]struct{})
	for x := chunkX - 1; x <= chunkX+1; x++ {
		for z := chunkZ - 1; z <= chunkZ+1; z++ {
			for _, name := range s.biomeNamesInChunk(x, z) {
				result[name] = struct{}{}
			}
		}
	}
	return result
}

func (s *featureSystem) biomeNamesInChunk(chunkX, chunkZ int32) []string {
	key := int64(uint64(uint32(chunkX))<<32 | uint64(uint32(chunkZ)))
	s.biomeCacheMu.RLock()
	names, ok := s.biomeCache[key]
	s.biomeCacheMu.RUnlock()
	if ok {
		return names
	}

	unique := make(map[string]struct{})
	baseQuartX, baseQuartZ := int(chunkX)*4, int(chunkZ)*4
	var sampler *cachedBiomeSource
	if pooled := s.biomeSamplePool.Get(); pooled != nil {
		sampler = pooled.(*cachedBiomeSource)
		sampler.reset(s.biomes)
	} else {
		sampler = newCachedBiomeSource(s.biomes)
	}
	sampler.beginChunk(int(chunkX)*16, int(chunkZ)*16, s.minY, s.height)
	for localX := 0; localX < 4; localX++ {
		for localZ := 0; localZ < 4; localZ++ {
			for quartY := s.minY >> 2; quartY < (s.minY+s.height)>>2; quartY++ {
				biomeID := sampler.sample(baseQuartX+localX, quartY, baseQuartZ+localZ)
				if biome := generatedbiome.BiomeByID(biomeID); biome != nil {
					unique[biome.Name] = struct{}{}
				}
			}
		}
	}
	s.biomeSamplePool.Put(sampler)
	names = make([]string, 0, len(unique))
	for name := range unique {
		names = append(names, name)
	}
	sort.Strings(names)

	s.biomeCacheMu.Lock()
	if existing, exists := s.biomeCache[key]; exists {
		names = existing
	} else {
		if len(s.biomeCacheKeys) < featureBiomeCacheCapacity {
			s.biomeCacheKeys = append(s.biomeCacheKeys, key)
		} else {
			delete(s.biomeCache, s.biomeCacheKeys[s.biomeCacheNext])
			s.biomeCacheKeys[s.biomeCacheNext] = key
			s.biomeCacheNext = (s.biomeCacheNext + 1) % featureBiomeCacheCapacity
		}
		s.biomeCache[key] = names
	}
	s.biomeCacheMu.Unlock()
	return names
}

func (p *placedFeature) place(system *featureSystem, chunk *section.Chunk, id string, random worldrandom.Source, origin blockPosition, biomeCache *featureBiomeMemo, biomes biomeSource) bool {
	if biomes == nil && system != nil {
		biomes = system.biomes
	}
	walk := placementWalk{feature: p, system: system, chunk: chunk, id: id, random: random, biomeCache: biomeCache, biomes: biomes}
	walk.visit(0, origin)
	return walk.placed
}

type placementWalk struct {
	feature    *placedFeature
	system     *featureSystem
	chunk      *section.Chunk
	id         string
	random     worldrandom.Source
	biomeCache *featureBiomeMemo
	biomes     biomeSource
	placed     bool
}

// visit processes one candidate through the remaining placement modifiers.
func (w *placementWalk) visit(index int, position blockPosition) {
	p, system, chunk, id, random, biomeCache, biomes := w.feature, w.system, w.chunk, w.id, w.random, w.biomeCache, w.biomes
	if index == len(p.placement) {
		if surface, ok := p.feature.(*freezeTopLayerFeature); ok {
			w.placed = surface.placeWithBiomes(chunk, position, biomes) || w.placed
			return
		}
		if p.feature.placeFeature(system, chunk, random, position) {
			w.placed = true
		}
		return
	}
	modifier := p.placement[index]
	switch modifier.kind {
	case "minecraft:count":
		for count := modifier.count.sample(random); count > 0; count-- {
			w.visit(index+1, position)
		}
	case "minecraft:rarity_filter":
		if random.NextIntBound(int32(modifier.chance)) == 0 {
			w.visit(index+1, position)
		}
	case "minecraft:in_square":
		position.x += int(random.NextIntBound(16))
		position.z += int(random.NextIntBound(16))
		w.visit(index+1, position)
	case "minecraft:height_range":
		position.y = modifier.height.sample(random, system.minY, system.height)
		w.visit(index+1, position)
	case "minecraft:biome":
		quart := biomeQuartPosition{x: position.x >> 2, y: position.y >> 2, z: position.z >> 2}
		biomeID := biomeCache.sample(biomes, quart)
		if p.acceptsBiome(system.resources, id, biomeID) {
			w.visit(index+1, position)
		}
	case "minecraft:block_predicate_filter":
		if modifier.predicate.test(system, chunk, position) {
			w.visit(index+1, position)
		}
	case "minecraft:heightmap":
		if y, ok := featureHeightmap(chunk, position.x, position.z, modifier.heightmap); ok {
			position.y = y
			w.visit(index+1, position)
		}
	case "minecraft:noise_threshold_count":
		count := modifier.aboveNoise
		if system.biomeNoise.Sample(float64(position.x)/200, float64(position.z)/200, false) < modifier.noiseLevel {
			count = modifier.belowNoise
		}
		for ; count > 0; count-- {
			w.visit(index+1, position)
		}
	case "minecraft:environment_scan":
		if !modifier.allowed.test(system, chunk, position) {
			return
		}
		found := false
		for step := 0; step < modifier.maxSteps; step++ {
			if modifier.target.test(system, chunk, position) {
				found = true
				break
			}
			position.y += modifier.direction
			if position.y < chunk.MinY() || position.y > chunk.MaxY() || !modifier.allowed.test(system, chunk, position) {
				break
			}
		}
		if !found && modifier.target.test(system, chunk, position) {
			found = true
		}
		if found {
			w.visit(index+1, position)
		}
	case "minecraft:random_offset":
		position.x += modifier.xzSpread.sample(random)
		position.y += modifier.ySpread.sample(random)
		position.z += modifier.xzSpread.sample(random)
		w.visit(index+1, position)
	case "minecraft:surface_water_depth_filter":
		oceanFloor, oceanOK := featureHeightmap(chunk, position.x, position.z, "OCEAN_FLOOR")
		worldSurface, surfaceOK := featureHeightmap(chunk, position.x, position.z, "WORLD_SURFACE_WG")
		if oceanOK && surfaceOK && worldSurface-oceanFloor <= modifier.maxWaterDepth {
			w.visit(index+1, position)
		}
	case "minecraft:noise_based_count":
		count := int(math.Ceil((system.biomeNoise.Sample(float64(position.x)/modifier.noiseFactor, float64(position.z)/modifier.noiseFactor, false) + modifier.noiseOffset) * float64(modifier.noiseRatio)))
		for ; count > 0; count-- {
			w.visit(index+1, position)
		}
	case "minecraft:surface_relative_threshold_filter":
		if surface, ok := featureHeightmap(chunk, position.x, position.z, modifier.heightmap); ok && position.y >= surface+modifier.thresholdMin && position.y <= surface+modifier.thresholdMax {
			w.visit(index+1, position)
		}
	case "minecraft:fixed_placement":
		chunkX, chunkZ := floorDiv(position.x, 16), floorDiv(position.z, 16)
		for _, fixed := range modifier.fixedPositions {
			if floorDiv(fixed.x, 16) == chunkX && floorDiv(fixed.z, 16) == chunkZ {
				w.visit(index+1, fixed)
			}
		}
	case "minecraft:count_on_every_layer":
		positions := make([]blockPosition, 0, 16)
		for layer := 0; ; layer++ {
			found := false
			for attempt := 0; attempt < modifier.count.sample(random); attempt++ {
				x := position.x + int(random.NextIntBound(16))
				z := position.z + int(random.NextIntBound(16))
				if y, ok := findGroundLayer(chunk, x, z, layer); ok {
					positions = append(positions, blockPosition{x: x, y: y, z: z})
					found = true
				}
			}
			if !found {
				break
			}
		}
		for _, candidate := range positions {
			w.visit(index+1, candidate)
		}
	}
}

// acceptsBiome uses immutable numeric membership while retaining support for unbound placements.
func (p *placedFeature) acceptsBiome(resources *featureSystemResources, id string, biomeID int32) bool {
	if p.allowedBiomes != nil && p.featureID == id {
		return biomeID >= 0 && biomeID < generatedbiome.BiomeCount && p.allowedBiomes[biomeID]
	}
	biome := generatedbiome.BiomeByID(biomeID)
	return biome != nil && biomeHasFeature(resources.biomeFeatures[biome.Name], id)
}

func biomeHasFeature(steps [][]string, id string) bool {
	for _, features := range steps {
		for _, candidate := range features {
			if candidate == id {
				return true
			}
		}
	}
	return false
}

func (o *configuredOre) place(chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	if o.scattered {
		return o.placeScattered(chunk, random, origin)
	}
	angle := random.NextFloat() * float32(math.Pi)
	radius := float32(o.size) / 8
	x0 := float64(origin.x) + math.Sin(float64(angle))*float64(radius)
	x1 := float64(origin.x) - math.Sin(float64(angle))*float64(radius)
	z0 := float64(origin.z) + math.Cos(float64(angle))*float64(radius)
	z1 := float64(origin.z) - math.Cos(float64(angle))*float64(radius)
	y0 := float64(origin.y + int(random.NextIntBound(3)) - 2)
	y1 := float64(origin.y + int(random.NextIntBound(3)) - 2)
	if !oreMayIntersectSurface(chunk, origin, o.size) {
		return false
	}
	scratch := oreScratchPool.Get().(*oreScratch)
	defer oreScratchPool.Put(scratch)
	var spheres [][4]float64
	if o.size > len(scratch.spheres) {
		spheres = make([][4]float64, o.size)
	} else {
		spheres = scratch.spheres[:o.size]
	}
	for i := 0; i < o.size; i++ {
		t := float32(i) / float32(o.size)
		size := random.NextDouble() * float64(o.size) / 16
		sphereRadius := (float64(mthSin(float32(math.Pi)*t)+1)*size + 1) / 2
		spheres[i] = [4]float64{x0 + float64(t)*(x1-x0), y0 + float64(t)*(y1-y0), z0 + float64(t)*(z1-z0), sphereRadius}
	}
	for i := 0; i < len(spheres)-1; i++ {
		if spheres[i][3] <= 0 {
			continue
		}
		for j := i + 1; j < len(spheres); j++ {
			if spheres[j][3] <= 0 {
				continue
			}
			dx, dy, dz, dr := spheres[i][0]-spheres[j][0], spheres[i][1]-spheres[j][1], spheres[i][2]-spheres[j][2], spheres[i][3]-spheres[j][3]
			if dr*dr > dx*dx+dy*dy+dz*dz {
				if dr > 0 {
					spheres[j][3] = -1
				} else {
					spheres[i][3] = -1
					// A removed sphere cannot remove any remaining positive-radius sphere.
					break
				}
			}
		}
	}
	if len(spheres) == 0 {
		return false
	}
	minX, minY, minZ := math.MaxInt, math.MaxInt, math.MaxInt
	maxX, maxY, maxZ := math.MinInt, math.MinInt, math.MinInt
	for _, sphere := range spheres {
		r := sphere[3]
		if r < 0 {
			continue
		}
		minX = min(minX, int(math.Floor(sphere[0]-r)))
		minY = min(minY, int(math.Floor(sphere[1]-r)))
		minZ = min(minZ, int(math.Floor(sphere[2]-r)))
		maxX = max(maxX, int(math.Floor(sphere[0]+r)))
		maxY = max(maxY, int(math.Floor(sphere[1]+r)))
		maxZ = max(maxZ, int(math.Floor(sphere[2]+r)))
	}
	if minX > maxX {
		return false
	}
	baseX, baseZ := int(chunk.X)*16, int(chunk.Z)*16
	if chunk.FeatureRegionEnabled() {
		baseX -= 16
		baseZ -= 16
	}
	width := 16
	if chunk.FeatureRegionEnabled() {
		width = 48
	}
	minX, maxX = max(minX, baseX), min(maxX, baseX+width-1)
	minZ, maxZ = max(minZ, baseZ), min(maxZ, baseZ+width-1)
	minY, maxY = max(minY, chunk.MinY()), min(maxY, chunk.MaxY())
	if minX > maxX || minY > maxY || minZ > maxZ {
		return false
	}
	ySize := maxY - minY + 1
	wordCount := (maxX - minX + 1) * ySize
	visited := scratch.visited
	if cap(visited) < wordCount {
		visited = make([]uint64, wordCount)
	} else {
		visited = visited[:wordCount]
		clear(visited)
	}
	scratch.visited = visited

	placed := false
	for _, sphere := range spheres {
		r := sphere[3]
		if r < 0 {
			continue
		}
		xStart, xEnd := max(minX, int(math.Floor(sphere[0]-r))), min(maxX, int(math.Floor(sphere[0]+r)))
		yStart, yEnd := max(minY, int(math.Floor(sphere[1]-r))), min(maxY, int(math.Floor(sphere[1]+r)))
		zStart, zEnd := max(minZ, int(math.Floor(sphere[2]-r))), min(maxZ, int(math.Floor(sphere[2]+r)))
		var zSquared [48]float64
		var zMask uint64
		for z := zStart; z <= zEnd; z++ {
			dz := (float64(z) + .5 - sphere[2]) / r
			zSquared[z-baseZ] = dz * dz
			if dz*dz < 1 {
				zMask |= uint64(1) << uint(z-baseZ)
			}
		}
		if zMask == 0 {
			continue
		}
		var ySquared [16]float64
		cacheY := yEnd-yStart < len(ySquared)
		if cacheY {
			for y := yStart; y <= yEnd; y++ {
				dy := (float64(y) + .5 - sphere[1]) / r
				ySquared[y-yStart] = dy * dy
			}
		}
		for x := xStart; x <= xEnd; x++ {
			dx := (float64(x) + .5 - sphere[0]) / r
			if dx*dx >= 1 {
				continue
			}
			for y := yStart; y <= yEnd; y++ {
				row := (x-minX)*ySize + y - minY
				pending := zMask &^ visited[row]
				if pending == 0 {
					continue
				}
				var dySquared float64
				if cacheY {
					dySquared = ySquared[y-yStart]
				} else {
					dy := (float64(y) + .5 - sphere[1]) / r
					dySquared = dy * dy
				}
				distance := dx*dx + dySquared
				if distance >= 1 {
					continue
				}
				for pending != 0 {
					localZ := bits.TrailingZeros64(pending)
					bit := uint64(1) << uint(localZ)
					pending &^= bit
					if distance+zSquared[localZ] < 1 {
						visited[row] |= bit
						placed = o.tryPlaceValid(chunk, random, blockPosition{x, y, baseZ + localZ}) || placed
					}
				}
			}
		}
	}
	return placed
}

// oreMayIntersectSurface applies the ore feature's early ocean-floor check.
func oreMayIntersectSurface(chunk *section.Chunk, origin blockPosition, size int) bool {
	radius := float32(size) / 8
	margin := int(math.Ceil(float64((float32(size)/16*2 + 1) / 2)))
	startX := origin.x - int(math.Ceil(float64(radius))) - margin
	startZ := origin.z - int(math.Ceil(float64(radius))) - margin
	startY := origin.y - 2 - margin
	width := 2 * (int(math.Ceil(float64(radius))) + margin)
	baseX, baseZ := int(chunk.X)*16, int(chunk.Z)*16
	if !chunk.FeatureRegionEnabled() && (startX < baseX || startX+width >= baseX+16 || startZ < baseZ || startZ+width >= baseZ+16) {
		return true
	}
	outside := false
	for x := startX; x <= startX+width; x++ {
		for z := startZ; z <= startZ+width; z++ {
			if !chunk.FeatureRegionEnabled() && (x < baseX || x >= baseX+16 || z < baseZ || z >= baseZ+16) {
				outside = true
				continue
			}
			if height, _ := chunk.FeatureHeightmap(x, z, section.HeightmapOceanFloor); startY <= height {
				return true
			}
		}
	}
	return outside
}

func (o *configuredOre) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	return o.place(chunk, random, origin)
}

func (o *configuredOre) placeScattered(chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	count := int(random.NextIntBound(int32(o.size + 1)))
	for i := 0; i < count; i++ {
		distance := i
		if distance > 7 {
			distance = 7
		}
		position := origin
		position.x += int(math.Floor(float64((random.NextFloat()-random.NextFloat())*float32(distance)) + .5))
		position.y += int(math.Floor(float64((random.NextFloat()-random.NextFloat())*float32(distance)) + .5))
		position.z += int(math.Floor(float64((random.NextFloat()-random.NextFloat())*float32(distance)) + .5))
		o.tryPlace(chunk, random, position)
	}
	return true
}

func (o *configuredOre) tryPlace(chunk *section.Chunk, random worldrandom.Source, position blockPosition) bool {
	if !chunk.FeatureWriteAllowed(position.x, position.y, position.z) {
		return false
	}
	return o.tryPlaceValid(chunk, random, position)
}

// tryPlaceValid places one candidate whose write bounds were already checked by the sphere traversal.
func (o *configuredOre) tryPlaceValid(chunk *section.Chunk, random worldrandom.Source, position blockPosition) bool {
	state := getChunkBlock(chunk, position)
	if o.resolvedTargets != nil {
		index := int(state - o.targetBase)
		if index < 0 || index >= len(o.resolvedTargets) || o.resolvedTargets[index] < 0 {
			return false
		}
		if o.discard > 0 && (o.discard >= 1 || random.NextFloat() < o.discard) && adjacentAir(chunk, position) {
			return false
		}
		setChunkBlock(chunk, position, o.resolvedTargets[index])
		return true
	}
	for _, target := range o.targets {
		if _, ok := target.states[state]; !ok {
			continue
		}
		if o.discard > 0 && (o.discard >= 1 || random.NextFloat() < o.discard) && adjacentAir(chunk, position) {
			return false
		}
		setChunkBlock(chunk, position, target.result)
		return true
	}
	return false
}

func adjacentAir(chunk *section.Chunk, position blockPosition) bool {
	baseX, baseZ := int(chunk.X)*16, int(chunk.Z)*16
	for _, offset := range [][3]int{{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}} {
		x, y, z := position.x+offset[0], position.y+offset[1], position.z+offset[2]
		if !chunk.FeatureRegionEnabled() && (x < baseX || x >= baseX+16 || z < baseZ || z >= baseZ+16 || y < chunk.MinY() || y > chunk.MaxY()) {
			continue
		}
		if block.IsAir(getChunkBlock(chunk, blockPosition{x, y, z})) {
			return true
		}
	}
	return false
}
