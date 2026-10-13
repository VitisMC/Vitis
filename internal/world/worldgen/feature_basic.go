package worldgen

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/vitismc/vitis/internal/block"
	blockfluid "github.com/vitismc/vitis/internal/block/fluid"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type stateProvider interface {
	state(*featureSystem, *section.Chunk, worldrandom.Source, blockPosition) int32
}

type simpleStateProvider int32

func (p simpleStateProvider) state(*featureSystem, *section.Chunk, worldrandom.Source, blockPosition) int32 {
	return int32(p)
}

type weightedState struct {
	stateID int32
	weight  int
}

type weightedStateProvider struct {
	entries []weightedState
	total   int
}

func (p *weightedStateProvider) state(_ *featureSystem, _ *section.Chunk, random worldrandom.Source, _ blockPosition) int32 {
	value := int(random.NextIntBound(int32(p.total)))
	for _, entry := range p.entries {
		value -= entry.weight
		if value < 0 {
			return entry.stateID
		}
	}
	return p.entries[len(p.entries)-1].stateID
}

type rotatedStateProvider struct{ name string }

func (p rotatedStateProvider) state(_ *featureSystem, _ *section.Chunk, random worldrandom.Source, _ blockPosition) int32 {
	axis := [...]string{"x", "y", "z"}[random.NextIntBound(3)]
	state, err := block.BlockStateFromProperties(p.name, map[string]string{"axis": axis})
	if err != nil {
		return block.DefaultStateID(p.name)
	}
	return state.ID()
}

type randomizedIntStateProvider struct {
	source   stateProvider
	property string
	values   intProvider
}

func (p *randomizedIntStateProvider) state(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, position blockPosition) int32 {
	base, err := block.BlockStateFromID(p.source.state(system, chunk, random, position))
	if err != nil {
		return 0
	}
	result, err := base.WithProperty(p.property, strconv.Itoa(p.values.sample(random)))
	if err != nil {
		return base.ID()
	}
	return result.ID()
}

type simpleBlockFeature struct {
	provider stateProvider
}

func (f *simpleBlockFeature) placeFeature(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, position blockPosition) bool {
	state := f.provider.state(system, chunk, random, position)
	if !canStateSurvive(chunk, position, state) {
		return false
	}
	name := block.NameFromState(state)
	if name == "minecraft:tall_grass" || name == "minecraft:large_fern" || name == "minecraft:sunflower" || name == "minecraft:lilac" || name == "minecraft:rose_bush" || name == "minecraft:peony" {
		if !isAirAt(chunk, blockPosition{x: position.x, y: position.y + 1, z: position.z}) {
			return false
		}
		upper, err := block.BlockStateFromProperties(name, map[string]string{"half": "upper"})
		if err != nil {
			return false
		}
		setChunkBlock(chunk, position, state)
		setChunkBlock(chunk, blockPosition{x: position.x, y: position.y + 1, z: position.z}, upper.ID())
		return true
	}
	setChunkBlock(chunk, position, state)
	return true
}

type blockPileFeature struct{ provider stateProvider }

func (f *blockPileFeature) placeFeature(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	if origin.y < chunk.MinY()+5 {
		return false
	}
	xRadius, zRadius := 2+int(random.NextIntBound(2)), 2+int(random.NextIntBound(2))
	for z := origin.z - zRadius; z <= origin.z+zRadius; z++ {
		for y := origin.y; y <= origin.y+1; y++ {
			for x := origin.x - xRadius; x <= origin.x+xRadius; x++ {
				dx, dz := origin.x-x, origin.z-z
				if float32(dx*dx+dz*dz) <= random.NextFloat()*10-random.NextFloat()*6 || float64(random.NextFloat()) < 0.031 {
					position := blockPosition{x: x, y: y, z: z}
					below := getChunkBlock(chunk, blockPosition{x: x, y: y - 1, z: z})
					mayPlace := block.IsSolid(below)
					if block.NameFromState(below) == "minecraft:dirt_path" {
						mayPlace = random.NextBoolean()
					}
					if isAirAt(chunk, position) && mayPlace {
						setChunkBlock(chunk, position, f.provider.state(system, chunk, random, position))
					}
				}
			}
		}
	}
	return true
}

type springFeature struct {
	state         int32
	requiresBelow bool
	rockCount     int
	holeCount     int
	valid         map[int32]struct{}
}

func (f *springFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, _ worldrandom.Source, position blockPosition) bool {
	valid := func(position blockPosition) bool {
		_, ok := f.valid[getChunkBlock(chunk, position)]
		return ok
	}
	if !valid(blockPosition{x: position.x, y: position.y + 1, z: position.z}) || f.requiresBelow && !valid(blockPosition{x: position.x, y: position.y - 1, z: position.z}) {
		return false
	}
	current := getChunkBlock(chunk, position)
	if !block.IsAir(current) {
		if _, ok := f.valid[current]; !ok {
			return false
		}
	}
	rock, holes := 0, 0
	for _, offset := range [][3]int{{-1, 0, 0}, {1, 0, 0}, {0, 0, -1}, {0, 0, 1}, {0, -1, 0}} {
		neighbor := blockPosition{x: position.x + offset[0], y: position.y + offset[1], z: position.z + offset[2]}
		if valid(neighbor) {
			rock++
		}
		if isAirAt(chunk, neighbor) {
			holes++
		}
	}
	if rock != f.rockCount || holes != f.holeCount {
		return false
	}
	setChunkBlock(chunk, position, f.state)
	return true
}

type randomPatchFeature struct {
	tries, xzSpread, ySpread int
	feature                  *placedFeature
}

func (f *randomPatchFeature) placeFeature(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	placed := false
	for range f.tries {
		position := blockPosition{
			x: origin.x + int(random.NextIntBound(int32(f.xzSpread+1))) - int(random.NextIntBound(int32(f.xzSpread+1))),
			y: origin.y + int(random.NextIntBound(int32(f.ySpread+1))) - int(random.NextIntBound(int32(f.ySpread+1))),
			z: origin.z + int(random.NextIntBound(int32(f.xzSpread+1))) - int(random.NextIntBound(int32(f.xzSpread+1))),
		}
		placed = f.feature.place(system, chunk, "", random, position, nil, nil) || placed
	}
	return placed
}

func decodeBasicConfiguredFeature(snapshot *datapack.Snapshot, kind string, raw json.RawMessage) (configuredFeature, bool, error) {
	if kind == "minecraft:bonus_chest" {
		feature, err := decodeBonusChestFeature(raw)
		return feature, true, err
	}
	if kind == "minecraft:lake" {
		feature, err := decodeLakeFeature(snapshot, raw)
		return feature, true, err
	}
	if kind == "minecraft:pointed_dripstone" {
		feature, err := decodePointedDripstoneFeature(snapshot, raw)
		return feature, true, err
	}
	if kind == "minecraft:dripstone_cluster" {
		feature, err := decodeDripstoneClusterFeature(snapshot, raw)
		return feature, true, err
	}
	if kind == "minecraft:large_dripstone" {
		feature, err := decodeLargeDripstoneFeature(snapshot, raw)
		return feature, true, err
	}
	if kind == "minecraft:geode" {
		feature, err := decodeGeodeFeature(snapshot, raw)
		return feature, true, err
	}
	if kind == "minecraft:iceberg" {
		feature, err := decodeIcebergFeature(raw)
		return feature, true, err
	}
	if kind == "minecraft:multiface_growth" {
		feature, err := decodeMultifaceGrowthFeature(snapshot, raw)
		return feature, true, err
	}
	if kind == "minecraft:tree" {
		return decodeTreeFeature(snapshot, raw)
	}
	if kind == "minecraft:root_system" {
		return decodeRootSystemFeature(snapshot, raw)
	}
	if kind == "minecraft:huge_brown_mushroom" || kind == "minecraft:huge_red_mushroom" {
		return decodeHugeMushroomFeature(kind, raw)
	}
	if kind == "minecraft:vegetation_patch" || kind == "minecraft:waterlogged_vegetation_patch" {
		return decodeVegetationPatchFeature(snapshot, raw)
	}
	if kind == "minecraft:coral_tree" || kind == "minecraft:coral_claw" || kind == "minecraft:coral_mushroom" {
		return decodeCoralFeature(snapshot, kind)
	}
	if feature, ok, err := decodeSelectorFeature(snapshot, kind, raw); ok || err != nil {
		return feature, ok, err
	}
	if feature, ok, err := decodeBlobFeature(snapshot, kind, raw); ok || err != nil {
		return feature, ok, err
	}
	if feature, ok, err := decodeAquaticFeature(snapshot, kind, raw); ok || err != nil {
		return feature, ok, err
	}
	if feature, ok, err := decodeSpecialFeature(snapshot, kind, raw); ok || err != nil {
		return feature, ok, err
	}
	if feature, ok, err := decodeEnvironmentFeature(kind, raw); ok || err != nil {
		return feature, ok, err
	}
	if feature, ok, err := decodeNetherFeature(kind, raw); ok || err != nil {
		return feature, ok, err
	}
	if feature, ok, err := decodeEndFeature(kind, raw); ok || err != nil {
		return feature, ok, err
	}
	if feature, ok, err := decodeSmallStructureFeature(snapshot, kind); ok || err != nil {
		return feature, ok, err
	}
	if kind == "minecraft:huge_fungus" {
		return decodeHugeFungusFeature(snapshot, raw)
	}
	switch kind {
	case "minecraft:simple_block":
		var value struct {
			Config struct {
				ToPlace json.RawMessage `json:"to_place"`
			} `json:"config"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, true, err
		}
		provider, err := decodeStateProvider(value.Config.ToPlace)
		return &simpleBlockFeature{provider: provider}, true, err
	case "minecraft:block_pile":
		var value struct {
			Config struct {
				Provider json.RawMessage `json:"state_provider"`
			} `json:"config"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, true, err
		}
		provider, err := decodeStateProvider(value.Config.Provider)
		return &blockPileFeature{provider: provider}, true, err
	case "minecraft:spring_feature":
		return decodeSpringFeature(raw)
	case "minecraft:random_patch", "minecraft:flower", "minecraft:no_bonemeal_flower":
		return decodeRandomPatch(snapshot, raw)
	case "minecraft:disk":
		return decodeDiskFeature(snapshot, raw)
	case "minecraft:block_column":
		return decodeBlockColumnFeature(snapshot, raw)
	default:
		return nil, false, nil
	}
}

func decodeStateProvider(raw json.RawMessage) (stateProvider, error) {
	var header struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return nil, err
	}
	switch header.Type {
	case "minecraft:noise_provider", "minecraft:dual_noise_provider":
		return decodeNoiseStateProvider(raw, header.Type == "minecraft:dual_noise_provider")
	case "minecraft:noise_threshold_provider":
		return decodeThresholdNoiseStateProvider(raw)
	case "minecraft:simple_state_provider":
		var value struct {
			State BlockState `json:"state"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		state, err := block.BlockStateFromProperties(value.State.Name, value.State.Properties)
		if err != nil {
			return nil, err
		}
		return simpleStateProvider(state.ID()), nil
	case "minecraft:rotated_block_provider":
		var value struct {
			State BlockState `json:"state"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		if _, err := block.BlockStateFromProperties(value.State.Name, value.State.Properties); err != nil {
			return nil, err
		}
		return rotatedStateProvider{name: value.State.Name}, nil
	case "minecraft:weighted_state_provider":
		var value struct {
			Entries []struct {
				Data   BlockState `json:"data"`
				Weight int        `json:"weight"`
			} `json:"entries"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		provider := &weightedStateProvider{}
		for _, entry := range value.Entries {
			state, err := block.BlockStateFromProperties(entry.Data.Name, entry.Data.Properties)
			if err != nil {
				return nil, err
			}
			if entry.Weight <= 0 {
				return nil, fmt.Errorf("invalid state weight")
			}
			provider.entries = append(provider.entries, weightedState{stateID: state.ID(), weight: entry.Weight})
			provider.total += entry.Weight
		}
		if provider.total == 0 {
			return nil, fmt.Errorf("empty weighted state provider")
		}
		return provider, nil
	case "minecraft:randomized_int_state_provider":
		var value struct {
			Source   json.RawMessage `json:"source"`
			Property string          `json:"property"`
			Values   json.RawMessage `json:"values"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		source, err := decodeStateProvider(value.Source)
		if err != nil {
			return nil, err
		}
		values, err := decodeIntProviderRange(value.Values, -1000000, 1000000)
		if err != nil {
			return nil, err
		}
		return &randomizedIntStateProvider{source: source, property: value.Property, values: values}, nil
	default:
		return nil, fmt.Errorf("unsupported state provider %q", header.Type)
	}
}

func decodeSpringFeature(raw json.RawMessage) (configuredFeature, bool, error) {
	var value struct {
		Config struct {
			State         BlockState      `json:"state"`
			RequiresBelow bool            `json:"requires_block_below"`
			RockCount     int             `json:"rock_count"`
			HoleCount     int             `json:"hole_count"`
			Valid         json.RawMessage `json:"valid_blocks"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, true, err
	}
	var state int32
	switch value.Config.State.Name {
	case "minecraft:water":
		state = blockfluid.FlowingStateID(blockfluid.FluidTypeWater, 8, true)
	case "minecraft:lava":
		state = blockfluid.FlowingStateID(blockfluid.FluidTypeLava, 8, true)
	default:
		return nil, true, fmt.Errorf("invalid spring fluid %s", value.Config.State.Name)
	}
	feature := &springFeature{state: state, requiresBelow: value.Config.RequiresBelow, rockCount: value.Config.RockCount, holeCount: value.Config.HoleCount, valid: make(map[int32]struct{})}
	names, err := decodeStringList(value.Config.Valid)
	if err != nil {
		return nil, true, err
	}
	for _, name := range names {
		info := block.InfoByName(normalizeResourceID(name))
		if info == nil {
			return nil, true, fmt.Errorf("unknown valid block %s", name)
		}
		for id := info.MinStateID; id <= info.MaxStateID; id++ {
			feature.valid[id] = struct{}{}
		}
	}
	return feature, true, nil
}

func decodeRandomPatch(snapshot *datapack.Snapshot, raw json.RawMessage) (configuredFeature, bool, error) {
	var value struct {
		Config struct {
			Tries    int             `json:"tries"`
			XZSpread int             `json:"xz_spread"`
			YSpread  int             `json:"y_spread"`
			Feature  json.RawMessage `json:"feature"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, true, err
	}
	placed, ok, err := decodeInlinePlacedFeature(snapshot, value.Config.Feature)
	if err != nil || !ok {
		return nil, ok, err
	}
	if value.Config.Tries <= 0 || value.Config.XZSpread < 0 || value.Config.YSpread < 0 {
		return nil, true, fmt.Errorf("invalid random patch bounds")
	}
	return &randomPatchFeature{tries: value.Config.Tries, xzSpread: value.Config.XZSpread, ySpread: value.Config.YSpread, feature: placed}, true, nil
}

func decodeInlinePlacedFeature(snapshot *datapack.Snapshot, raw json.RawMessage) (*placedFeature, bool, error) {
	var value struct {
		Feature   json.RawMessage   `json:"feature"`
		Placement []json.RawMessage `json:"placement"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, false, err
	}
	configuredRaw := value.Feature
	var configuredID string
	if err := json.Unmarshal(value.Feature, &configuredID); err == nil {
		configuredID = normalizeResourceID(configuredID)
		namespace, path, ok := strings.Cut(configuredID, ":")
		if !ok {
			return nil, false, fmt.Errorf("invalid configured feature reference %s", configuredID)
		}
		data, exists := snapshot.Get("data/" + namespace + "/worldgen/configured_feature/" + path + ".json")
		if !exists {
			return nil, false, fmt.Errorf("unknown configured feature %s", configuredID)
		}
		configuredRaw = data
	}
	feature, ok, err := decodeConfiguredFeature(snapshot, configuredRaw)
	if err != nil || !ok {
		return nil, ok, err
	}
	encoded, err := json.Marshal(struct {
		Placement []json.RawMessage `json:"placement"`
	}{Placement: value.Placement})
	if err != nil {
		return nil, false, err
	}
	placed, err := decodePlacedFeature(encoded, feature)
	return placed, err == nil, err
}

func getChunkBlock(chunk *section.Chunk, position blockPosition) int32 {
	if chunk == nil || position.y < chunk.MinY() || position.y > chunk.MaxY() {
		return 0
	}
	if position.x < int(chunk.X)*16 || position.x >= int(chunk.X)*16+16 || position.z < int(chunk.Z)*16 || position.z >= int(chunk.Z)*16+16 {
		state, _ := chunk.GetBorderBlock(position.x, position.y, position.z)
		return state
	}
	return chunk.GetBlock(position.x, position.y, position.z)
}

func setChunkBlock(chunk *section.Chunk, position blockPosition, state int32) {
	chunk.SetFeatureBlock(position.x, position.y, position.z, state)
}

func isAirAt(chunk *section.Chunk, position blockPosition) bool {
	return block.IsAir(getChunkBlock(chunk, position))
}

func canStateSurvive(chunk *section.Chunk, position blockPosition, state int32) bool {
	name := block.NameFromState(state)
	if name == "minecraft:spore_blossom" {
		return block.IsSolid(getChunkBlock(chunk, blockPosition{x: position.x, y: position.y + 1, z: position.z}))
	}
	if name == "minecraft:lily_pad" {
		return blockfluid.IsWater(getChunkBlock(chunk, blockPosition{x: position.x, y: position.y - 1, z: position.z}))
	}
	if name == "minecraft:fire" || name == "minecraft:soul_fire" {
		return block.IsSolid(getChunkBlock(chunk, blockPosition{x: position.x, y: position.y - 1, z: position.z}))
	}
	return block.IsSolid(getChunkBlock(chunk, blockPosition{x: position.x, y: position.y - 1, z: position.z}))
}

func featureHeightmap(chunk *section.Chunk, x, z int, kind string) (int, bool) {
	var heightmap section.HeightmapKind
	switch kind {
	case "WORLD_SURFACE_WG":
		heightmap = section.HeightmapWorldSurface
	case "OCEAN_FLOOR", "OCEAN_FLOOR_WG":
		heightmap = section.HeightmapOceanFloor
	case "MOTION_BLOCKING":
		heightmap = section.HeightmapMotionBlocking
	case "MOTION_BLOCKING_NO_LEAVES":
		heightmap = section.HeightmapMotionBlockingNoLeaves
	default:
		return 0, false
	}
	return chunk.FeatureHeightmap(x, z, heightmap)
}

func findGroundLayer(chunk *section.Chunk, x, z, targetLayer int) (int, bool) {
	height, ok := featureHeightmap(chunk, x, z, "MOTION_BLOCKING")
	if !ok {
		return 0, false
	}
	layers := 0
	current := getChunkBlock(chunk, blockPosition{x: x, y: height, z: z})
	for y := height; y >= chunk.MinY()+1; y-- {
		below := getChunkBlock(chunk, blockPosition{x: x, y: y - 1, z: z})
		emptyCurrent := block.IsAir(current) || blockfluid.IsFluid(current)
		emptyBelow := block.IsAir(below) || blockfluid.IsFluid(below)
		if !emptyBelow && emptyCurrent && block.NameFromState(below) != "minecraft:bedrock" {
			if layers == targetLayer {
				return y, true
			}
			layers++
		}
		current = below
	}
	return 0, false
}
