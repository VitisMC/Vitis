package worldgen

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vitismc/vitis/internal/block"
	genblock "github.com/vitismc/vitis/internal/data/generated/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// multifaceGrowthFeature owns compiled attachment targets and all face/water state combinations.
type multifaceGrowthFeature struct {
	base          int32
	masks         []uint8
	states        [128]int32
	targets, fire [genblock.BlockCount]bool
	directions    []featureFace
	searchRange   int
	spreadChance  float32
	vein          bool
}

// decodeMultifaceGrowthFeature validates configuration and resolves immutable state tables.
func decodeMultifaceGrowthFeature(snapshot *datapack.Snapshot, raw json.RawMessage) (*multifaceGrowthFeature, error) {
	var value struct {
		Config struct {
			Block   string          `json:"block"`
			Range   *int            `json:"search_range"`
			Floor   bool            `json:"can_place_on_floor"`
			Ceiling bool            `json:"can_place_on_ceiling"`
			Wall    bool            `json:"can_place_on_wall"`
			Chance  *float32        `json:"chance_of_spreading"`
			Targets json.RawMessage `json:"can_be_placed_on"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	c := value.Config
	f := &multifaceGrowthFeature{searchRange: 10, spreadChance: 0.5}
	if c.Range != nil {
		f.searchRange = *c.Range
	}
	if c.Chance != nil {
		f.spreadChance = *c.Chance
	}
	if f.searchRange < 1 || f.searchRange > 64 || f.spreadChance < 0 || f.spreadChance > 1 {
		return nil, fmt.Errorf("invalid multiface search range or spread chance")
	}
	if c.Block == "" {
		c.Block = "minecraft:glow_lichen"
	}
	c.Block = normalizeResourceID(c.Block)
	if c.Block != "minecraft:glow_lichen" && c.Block != "minecraft:sculk_vein" {
		return nil, fmt.Errorf("unsupported multiface block %s", c.Block)
	}
	f.vein = c.Block == "minecraft:sculk_vein"
	info := block.InfoByName(c.Block)
	if info == nil {
		return nil, fmt.Errorf("unknown multiface block %s", c.Block)
	}
	f.base = info.MinStateID
	f.masks = make([]uint8, info.MaxStateID-info.MinStateID+1)
	for i := range f.states {
		f.states[i] = -1
	}
	for id := info.MinStateID; id <= info.MaxStateID; id++ {
		state, err := block.BlockStateFromID(id)
		if err != nil {
			return nil, err
		}
		var mask uint8
		for face, name := range featureFaceNames {
			if enabled, _ := state.Property(name); enabled == "true" {
				mask |= 1 << face
			}
		}
		if wet, _ := state.Property("waterlogged"); wet == "true" {
			mask |= 64
		}
		f.masks[id-f.base] = mask
		f.states[mask] = id
	}
	for _, id := range f.states {
		if id < 0 {
			return nil, fmt.Errorf("incomplete multiface state table for %s", c.Block)
		}
	}
	if c.Ceiling {
		f.directions = append(f.directions, featureUp)
	}
	if c.Floor {
		f.directions = append(f.directions, featureDown)
	}
	if c.Wall {
		f.directions = append(f.directions, featureNorth, featureEast, featureSouth, featureWest)
	}
	if len(c.Targets) == 0 || string(c.Targets) == "null" {
		return nil, fmt.Errorf("missing multiface attachment targets")
	}
	var names []string
	var single string
	if err := json.Unmarshal(c.Targets, &single); err == nil {
		if strings.HasPrefix(single, "#") {
			if snapshot == nil {
				return nil, fmt.Errorf("multiface target tag requires resources")
			}
			var err error
			names, err = snapshot.ResolveTag("block", normalizeResourceID(strings.TrimPrefix(single, "#")))
			if err != nil {
				return nil, err
			}
		} else {
			names = []string{single}
		}
	} else if err := json.Unmarshal(c.Targets, &names); err != nil {
		return nil, err
	}
	for _, name := range names {
		info := block.InfoByName(normalizeResourceID(name))
		if info == nil {
			return nil, fmt.Errorf("unknown multiface attachment block %s", name)
		}
		f.targets[info.ID] = true
	}
	if f.vein {
		if snapshot == nil {
			return nil, fmt.Errorf("sculk vein requires fire tag resources")
		}
		fire, err := snapshot.ResolveTag("block", "minecraft:fire")
		if err != nil {
			return nil, err
		}
		for _, name := range fire {
			info := block.InfoByName(name)
			if info == nil {
				return nil, fmt.Errorf("unknown fire tag block %s", name)
			}
			f.fire[info.ID] = true
		}
	}
	return f, nil
}

// mask identifies existing growth states without property lookup or allocation.
func (f *multifaceGrowthFeature) mask(state int32) (uint8, bool) {
	index := int(state - f.base)
	if index < 0 || index >= len(f.masks) {
		return 0, false
	}
	return f.masks[index], true
}

// shuffleFeatureFaces preserves the backward Fisher-Yates order of RNG draws.
func shuffleFeatureFaces(directions []featureFace, random worldrandom.Source) {
	for i := len(directions) - 1; i > 0; i-- {
		j := int(random.NextIntBound(int32(i + 1)))
		directions[i], directions[j] = directions[j], directions[i]
	}
}

// placeFeature attaches growth at the origin or one adjacent search position, then spreads once.
func (f *multifaceGrowthFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	if !chunk.FeatureWriteAllowed(origin.x, origin.y, origin.z) {
		return false
	}
	state := getChunkBlock(chunk, origin)
	if !block.IsAir(state) && !block.IsWater(state) {
		return false
	}
	var initial [6]featureFace
	directions := initial[:copy(initial[:], f.directions)]
	shuffleFeatureFaces(directions, random)
	if f.attach(chunk, random, origin, state, directions) {
		return true
	}
	for _, search := range directions {
		var order [6]featureFace
		count := 0
		for _, face := range f.directions {
			if face != search.opposite() {
				order[count] = face
				count++
			}
		}
		candidates := order[:count]
		shuffleFeatureFaces(candidates, random)
		position := search.offset(origin)
		state := getChunkBlock(chunk, position)
		_, same := f.mask(state)
		if !block.IsAir(state) && !block.IsWater(state) && !same {
			continue
		}
		// Version 1.21.4 revisits this same cell: failed retries neither write nor draw RNG.
		if f.attach(chunk, random, position, state, candidates) {
			return true
		}
	}
	return false
}

// stateForFace adds one supported face while retaining existing faces and source water.
func (f *multifaceGrowthFeature) stateForFace(chunk *section.Chunk, position blockPosition, state int32, face featureFace) (int32, bool) {
	if !chunk.FeatureWriteAllowed(position.x, position.y, position.z) {
		return 0, false
	}
	mask, same := f.mask(state)
	if same && mask&(1<<face) != 0 {
		return 0, false
	}
	if !featureFullFace(getChunkBlock(chunk, face.offset(position)), face.opposite(), true) {
		return 0, false
	}
	if !same {
		mask = 0
		if generationBlockTraits(state)&featureWaterSourceTrait != 0 {
			mask = 64
		}
	}
	return f.states[mask|1<<face], true
}

// attach stops at the first configured substrate, preserving placement and spread RNG order.
func (f *multifaceGrowthFeature) attach(chunk *section.Chunk, random worldrandom.Source, position blockPosition, state int32, directions []featureFace) bool {
	for _, face := range directions {
		support := getChunkBlock(chunk, face.offset(position))
		id := block.BlockIDFromState(support)
		if id < 0 || !f.targets[id] {
			continue
		}
		growth, ok := f.stateForFace(chunk, position, state, face)
		if !ok {
			return false
		}
		setChunkBlock(chunk, position, growth)
		if random.NextFloat() < f.spreadChance {
			f.spreadRandom(chunk, random, position, growth, face)
		}
		return true
	}
	return false
}

// spreadRandom shuffles all six directions and accepts the first supported spread candidate.
func (f *multifaceGrowthFeature) spreadRandom(chunk *section.Chunk, random worldrandom.Source, origin blockPosition, state int32, face featureFace) bool {
	directions := [6]featureFace{featureDown, featureUp, featureNorth, featureSouth, featureWest, featureEast}
	shuffleFeatureFaces(directions[:], random)
	for _, toward := range directions {
		if f.spreadDirection(chunk, origin, state, face, toward) {
			return true
		}
	}
	return false
}

// spreadDirection tries another face, a coplanar neighbor, then the outer corner in that order.
func (f *multifaceGrowthFeature) spreadDirection(chunk *section.Chunk, origin blockPosition, state int32, face, toward featureFace) bool {
	if face/2 == toward/2 {
		return false
	}
	mask, same := f.mask(state)
	if same || !f.vein {
		if mask&(1<<face) == 0 || mask&(1<<toward) != 0 {
			return false
		}
	}
	candidates := [3]struct {
		position blockPosition
		face     featureFace
	}{
		{origin, toward}, {toward.offset(origin), face}, {face.offset(toward.offset(origin)), toward.opposite()},
	}
	for _, candidate := range candidates {
		old := getChunkBlock(chunk, candidate.position)
		if !f.canSpreadInto(chunk, origin, candidate.position, candidate.face, old) {
			continue
		}
		growth, ok := f.stateForFace(chunk, candidate.position, old, candidate.face)
		if !ok {
			continue
		}
		setChunkBlock(chunk, candidate.position, growth)
		return true
	}
	return false
}

// canSpreadInto applies source-water replacement and the additional sculk corner/fire exclusions.
func (f *multifaceGrowthFeature) canSpreadInto(chunk *section.Chunk, origin, position blockPosition, face featureFace, state int32) bool {
	_, same := f.mask(state)
	traits := generationBlockTraits(state)
	replace := block.IsAir(state) || same || block.IsWater(state) && traits&featureWaterSourceTrait != 0
	if !f.vein {
		return replace
	}
	support := getChunkBlock(chunk, face.offset(position))
	switch block.NameFromState(support) {
	case "minecraft:sculk", "minecraft:sculk_catalyst", "minecraft:moving_piston":
		return false
	}
	if absInt(origin.x-position.x)+absInt(origin.y-position.y)+absInt(origin.z-position.z) == 2 {
		if featureFullFace(getChunkBlock(chunk, face.opposite().offset(origin)), face, false) {
			return false
		}
	}
	if traits&featureFluidTrait != 0 && traits&featureWaterSourceTrait == 0 {
		return false
	}
	if id := block.BlockIDFromState(state); id >= 0 && f.fire[id] {
		return false
	}
	return replace || traits&featureReplaceableTrait != 0
}
