package worldgen

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type treeFeature struct {
	trunk, foliage, dirt                                stateProvider
	trunkKind, foliageKind                              string
	baseHeight, heightRandA, heightRandB                int
	foliageHeight                                       intProvider
	radius, offset                                      intProvider
	trunkHeight                                         intProvider
	sizeKind                                            string
	limit, lowerSize, middleSize, upperSize, upperLimit int
	minClippedHeight                                    *int
	forceDirt, ignoreVines                              bool
	replaceable, logs, leaves, dirtStates               map[int32]struct{}
	cherry                                              *cherryTreeConfig
	bending                                             *bendingTreeConfig
	mangrove                                            *mangroveTreeConfig
	leafAttempts                                        int
	decorators                                          []treeDecorator
}

type treeAttachment struct {
	position     blockPosition
	branchBase   int
	radiusOffset int
	doubleTrunk  bool
}

// decodeTreeFeature compiles tree configurations with supported trunk and foliage placers.
func decodeTreeFeature(snapshot *datapack.Snapshot, raw json.RawMessage) (configuredFeature, bool, error) {
	var data struct {
		Config struct {
			TrunkProvider   json.RawMessage `json:"trunk_provider"`
			FoliageProvider json.RawMessage `json:"foliage_provider"`
			DirtProvider    json.RawMessage `json:"dirt_provider"`
			TrunkPlacer     struct {
				Type                   string          `json:"type"`
				BaseHeight             int             `json:"base_height"`
				HeightRandA            int             `json:"height_rand_a"`
				HeightRandB            int             `json:"height_rand_b"`
				BranchCount            json.RawMessage `json:"branch_count"`
				BranchHorizontalLength json.RawMessage `json:"branch_horizontal_length"`
				BranchStartOffset      json.RawMessage `json:"branch_start_offset_from_top"`
				BranchEndOffset        json.RawMessage `json:"branch_end_offset_from_top"`
				MinHeightForLeaves     int             `json:"min_height_for_leaves"`
				BendLength             json.RawMessage `json:"bend_length"`
				ExtraBranchSteps       json.RawMessage `json:"extra_branch_steps"`
				ExtraBranchLength      json.RawMessage `json:"extra_branch_length"`
				BranchProbability      float32         `json:"place_branch_per_log_probability"`
				CanGrowThrough         string          `json:"can_grow_through"`
			} `json:"trunk_placer"`
			FoliagePlacer struct {
				Type                         string          `json:"type"`
				Height                       json.RawMessage `json:"height"`
				CrownHeight                  json.RawMessage `json:"crown_height"`
				TrunkHeight                  json.RawMessage `json:"trunk_height"`
				Radius                       json.RawMessage `json:"radius"`
				Offset                       json.RawMessage `json:"offset"`
				WideBottomLayerHoleChance    float32         `json:"wide_bottom_layer_hole_chance"`
				CornerHoleChance             float32         `json:"corner_hole_chance"`
				HangingLeavesChance          float32         `json:"hanging_leaves_chance"`
				HangingLeavesExtensionChance float32         `json:"hanging_leaves_extension_chance"`
				RandomSpreadHeight           json.RawMessage `json:"foliage_height"`
				LeafPlacementAttempts        int             `json:"leaf_placement_attempts"`
			} `json:"foliage_placer"`
			MinimumSize struct {
				Type             string `json:"type"`
				Limit            int    `json:"limit"`
				UpperLimit       int    `json:"upper_limit"`
				LowerSize        int    `json:"lower_size"`
				MiddleSize       int    `json:"middle_size"`
				UpperSize        int    `json:"upper_size"`
				MinClippedHeight *int   `json:"min_clipped_height"`
			} `json:"minimum_size"`
			ForceDirt   bool              `json:"force_dirt"`
			RootPlacer  json.RawMessage   `json:"root_placer"`
			IgnoreVines bool              `json:"ignore_vines"`
			Decorators  []json.RawMessage `json:"decorators"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, true, err
	}
	c := data.Config
	if c.TrunkPlacer.Type != "minecraft:straight_trunk_placer" && c.TrunkPlacer.Type != "minecraft:fancy_trunk_placer" && c.TrunkPlacer.Type != "minecraft:forking_trunk_placer" && c.TrunkPlacer.Type != "minecraft:dark_oak_trunk_placer" && c.TrunkPlacer.Type != "minecraft:cherry_trunk_placer" && c.TrunkPlacer.Type != "minecraft:bending_trunk_placer" && c.TrunkPlacer.Type != "minecraft:giant_trunk_placer" && c.TrunkPlacer.Type != "minecraft:mega_jungle_trunk_placer" && c.TrunkPlacer.Type != "minecraft:upwards_branching_trunk_placer" {
		return nil, false, nil
	}
	switch c.FoliagePlacer.Type {
	case "minecraft:blob_foliage_placer", "minecraft:bush_foliage_placer", "minecraft:fancy_foliage_placer", "minecraft:spruce_foliage_placer", "minecraft:pine_foliage_placer", "minecraft:mega_pine_foliage_placer", "minecraft:jungle_foliage_placer", "minecraft:acacia_foliage_placer", "minecraft:dark_oak_foliage_placer", "minecraft:cherry_foliage_placer", "minecraft:random_spread_foliage_placer":
	default:
		return nil, false, nil
	}
	if (c.TrunkPlacer.Type == "minecraft:bending_trunk_placer" || c.TrunkPlacer.Type == "minecraft:upwards_branching_trunk_placer") != (c.FoliagePlacer.Type == "minecraft:random_spread_foliage_placer") {
		return nil, false, nil
	}
	if c.MinimumSize.Type != "minecraft:two_layers_feature_size" && c.MinimumSize.Type != "minecraft:three_layers_feature_size" {
		return nil, false, nil
	}
	if c.TrunkPlacer.BaseHeight < 0 || c.TrunkPlacer.BaseHeight > 32 || c.TrunkPlacer.HeightRandA < 0 || c.TrunkPlacer.HeightRandA > 24 || c.TrunkPlacer.HeightRandB < 0 || c.TrunkPlacer.HeightRandB > 24 || c.MinimumSize.Limit < 0 || c.MinimumSize.Limit > 81 || c.MinimumSize.UpperLimit < 0 || c.MinimumSize.UpperLimit > 80 || c.MinimumSize.LowerSize < 0 || c.MinimumSize.LowerSize > 16 || c.MinimumSize.MiddleSize < 0 || c.MinimumSize.MiddleSize > 16 || c.MinimumSize.UpperSize < 0 || c.MinimumSize.UpperSize > 16 || c.MinimumSize.MinClippedHeight != nil && (*c.MinimumSize.MinClippedHeight < 0 || *c.MinimumSize.MinClippedHeight > 80) {
		return nil, true, fmt.Errorf("invalid tree size")
	}
	feature := &treeFeature{
		trunkKind: c.TrunkPlacer.Type, foliageKind: c.FoliagePlacer.Type,
		baseHeight: c.TrunkPlacer.BaseHeight, heightRandA: c.TrunkPlacer.HeightRandA, heightRandB: c.TrunkPlacer.HeightRandB,
		sizeKind: c.MinimumSize.Type, limit: c.MinimumSize.Limit, lowerSize: c.MinimumSize.LowerSize, middleSize: c.MinimumSize.MiddleSize, upperSize: c.MinimumSize.UpperSize, upperLimit: c.MinimumSize.UpperLimit,
		minClippedHeight: c.MinimumSize.MinClippedHeight, forceDirt: c.ForceDirt, ignoreVines: c.IgnoreVines, leafAttempts: c.FoliagePlacer.LeafPlacementAttempts,
	}
	var err error
	if feature.trunk, err = decodeStateProvider(c.TrunkProvider); err != nil {
		return nil, true, fmt.Errorf("trunk provider: %w", err)
	}
	if feature.foliage, err = decodeStateProvider(c.FoliageProvider); err != nil {
		return nil, true, fmt.Errorf("foliage provider: %w", err)
	}
	if feature.dirt, err = decodeStateProvider(c.DirtProvider); err != nil {
		return nil, true, fmt.Errorf("dirt provider: %w", err)
	}
	if feature.radius, err = decodeIntProviderRange(c.FoliagePlacer.Radius, 0, 16); err != nil {
		return nil, true, fmt.Errorf("foliage radius: %w", err)
	}
	if feature.offset, err = decodeIntProviderRange(c.FoliagePlacer.Offset, 0, 16); err != nil {
		return nil, true, fmt.Errorf("foliage offset: %w", err)
	}
	if feature.foliageKind == "minecraft:acacia_foliage_placer" {
		feature.foliageHeight = constantInt(0)
	} else if feature.foliageKind == "minecraft:dark_oak_foliage_placer" {
		feature.foliageHeight = constantInt(4)
	} else if feature.foliageKind == "minecraft:spruce_foliage_placer" {
		feature.trunkHeight, err = decodeIntProviderRange(c.FoliagePlacer.TrunkHeight, 0, 24)
	} else if feature.foliageKind == "minecraft:random_spread_foliage_placer" {
		feature.foliageHeight, err = decodeIntProviderRange(c.FoliagePlacer.RandomSpreadHeight, 1, 512)
	} else if feature.foliageKind == "minecraft:pine_foliage_placer" {
		feature.foliageHeight, err = decodeIntProviderRange(c.FoliagePlacer.Height, 0, 24)
	} else if feature.foliageKind == "minecraft:mega_pine_foliage_placer" {
		feature.foliageHeight, err = decodeIntProviderRange(c.FoliagePlacer.CrownHeight, 0, 24)
	} else {
		feature.foliageHeight, err = decodeIntProviderRange(c.FoliagePlacer.Height, 0, 16)
	}
	if err != nil {
		return nil, true, fmt.Errorf("foliage height: %w", err)
	}
	if c.TrunkPlacer.Type == "minecraft:cherry_trunk_placer" {
		feature.cherry, err = decodeCherryTreeConfig(c.TrunkPlacer.BranchCount, c.TrunkPlacer.BranchHorizontalLength, c.TrunkPlacer.BranchStartOffset, c.TrunkPlacer.BranchEndOffset, c.FoliagePlacer.WideBottomLayerHoleChance, c.FoliagePlacer.CornerHoleChance, c.FoliagePlacer.HangingLeavesChance, c.FoliagePlacer.HangingLeavesExtensionChance)
		if err != nil {
			return nil, true, err
		}
	}
	if c.TrunkPlacer.Type == "minecraft:bending_trunk_placer" {
		if c.TrunkPlacer.MinHeightForLeaves < 1 || c.TrunkPlacer.MinHeightForLeaves > 64 || c.FoliagePlacer.LeafPlacementAttempts < 0 || c.FoliagePlacer.LeafPlacementAttempts > 256 {
			return nil, true, fmt.Errorf("invalid bending tree settings")
		}
		bendLength, err := decodeIntProviderRange(c.TrunkPlacer.BendLength, 1, 64)
		if err != nil {
			return nil, true, fmt.Errorf("bend length: %w", err)
		}
		feature.bending = &bendingTreeConfig{bendLength: bendLength, minHeightForLeaves: c.TrunkPlacer.MinHeightForLeaves}
	}
	if c.TrunkPlacer.Type == "minecraft:upwards_branching_trunk_placer" {
		feature.mangrove, err = decodeMangroveTreeConfig(snapshot, c.RootPlacer, c.TrunkPlacer.ExtraBranchSteps, c.TrunkPlacer.ExtraBranchLength, c.TrunkPlacer.CanGrowThrough, c.TrunkPlacer.BranchProbability)
		if err != nil {
			return nil, true, err
		}
	}
	for _, rawDecorator := range c.Decorators {
		decorator, supported, err := decodeTreeDecorator(snapshot, rawDecorator)
		if err != nil {
			return nil, true, fmt.Errorf("tree decorator: %w", err)
		}
		if !supported {
			return nil, false, nil
		}
		feature.decorators = append(feature.decorators, decorator)
	}
	resolve := func(tag string) (map[int32]struct{}, error) {
		names, err := snapshot.ResolveTag("block", tag)
		if err != nil {
			return nil, err
		}
		states := make(map[int32]struct{})
		for _, name := range names {
			info := block.InfoByName(name)
			if info == nil {
				return nil, fmt.Errorf("unknown tree block %s", name)
			}
			for id := info.MinStateID; id <= info.MaxStateID; id++ {
				states[id] = struct{}{}
			}
		}
		return states, nil
	}
	if feature.replaceable, err = resolve("minecraft:replaceable_by_trees"); err != nil {
		return nil, true, err
	}
	if feature.logs, err = resolve("minecraft:logs"); err != nil {
		return nil, true, err
	}
	if feature.leaves, err = resolve("minecraft:leaves"); err != nil {
		return nil, true, err
	}
	if feature.dirtStates, err = resolve("minecraft:dirt"); err != nil {
		return nil, true, err
	}
	return feature, true, nil
}

// validPosition checks tree replacement rules within and beside the source chunk.
func (f *treeFeature) validPosition(chunk *section.Chunk, position blockPosition) bool {
	if position.y < chunk.MinY() || position.y > chunk.MaxY() {
		return false
	}
	baseX, baseZ := int(chunk.X)*16, int(chunk.Z)*16
	if position.x < baseX || position.x >= baseX+16 || position.z < baseZ || position.z >= baseZ+16 {
		if !chunk.BorderBlocksEnabled() || position.x < baseX-16 || position.x >= baseX+32 || position.z < baseZ-16 || position.z >= baseZ+32 {
			return false
		}
	}
	state := getTreeBlock(chunk, position)
	if block.IsAir(state) {
		return true
	}
	_, ok := f.replaceable[state]
	return ok
}

// validTrunkPosition also accepts blocks listed by a mangrove trunk placer.
func (f *treeFeature) validTrunkPosition(chunk *section.Chunk, position blockPosition) bool {
	if f.validPosition(chunk, position) {
		return true
	}
	if f.mangrove == nil || position.y < chunk.MinY() || position.y > chunk.MaxY() {
		return false
	}
	state := getTreeBlock(chunk, position)
	_, ok := f.mangrove.trunkGrowThrough[state]
	return ok
}

// getTreeBlock reads a tree's own writes beyond its source chunk.
func getTreeBlock(chunk *section.Chunk, position blockPosition) int32 {
	if chunk == nil || position.y < chunk.MinY() || position.y > chunk.MaxY() {
		return 0
	}
	if position.x < int(chunk.X)*16 || position.x >= int(chunk.X)*16+16 || position.z < int(chunk.Z)*16 || position.z >= int(chunk.Z)*16+16 {
		state, _ := chunk.GetBorderBlock(position.x, position.y, position.z)
		return state
	}
	return chunk.GetBlock(position.x, position.y, position.z)
}

// setTreeBlock writes within the source chunk or records an adjacent block.
func setTreeBlock(chunk *section.Chunk, position blockPosition, state int32) {
	if chunk.FeatureRegionEnabled() {
		chunk.SetFeatureBlock(position.x, position.y, position.z, state)
		return
	}
	if position.x < int(chunk.X)*16 || position.x >= int(chunk.X)*16+16 || position.z < int(chunk.Z)*16 || position.z >= int(chunk.Z)*16+16 {
		chunk.SetBorderBlock(position.x, position.y, position.z, state)
		return
	}
	chunk.SetBlock(position.x, position.y, position.z, state)
}

// freePosition checks the space required for the trunk and foliage.
func (f *treeFeature) freePosition(chunk *section.Chunk, position blockPosition) bool {
	if f.validTrunkPosition(chunk, position) {
		return true
	}
	_, ok := f.logs[getTreeBlock(chunk, position)]
	return ok
}

// placeFeature grows the trunk, canopy, and leaf support distances.
func (f *treeFeature) placeFeature(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	if !chunk.FeatureWriteAllowed(origin.x, origin.y, origin.z) {
		return false
	}
	height := f.baseHeight + int(random.NextIntBound(int32(f.heightRandA+1))) + int(random.NextIntBound(int32(f.heightRandB+1)))
	foliageHeight := 0
	if f.foliageKind == "minecraft:spruce_foliage_placer" {
		foliageHeight = max(4, height-f.trunkHeight.sample(random))
	} else {
		foliageHeight = f.foliageHeight.sample(random)
	}
	radius := f.radius.sample(random)
	if f.foliageKind == "minecraft:pine_foliage_placer" {
		radius += int(random.NextIntBound(int32(max(height-foliageHeight+1, 1))))
	}
	rootOrigin := origin
	if f.mangrove != nil {
		origin.y += f.mangrove.trunkOffset.sample(random)
	}
	if origin.y < chunk.MinY()+1 || origin.y+height+1 > chunk.MaxY()+1 {
		return false
	}
	baseWidth := 1
	if f.trunkKind == "minecraft:dark_oak_trunk_placer" || f.trunkKind == "minecraft:giant_trunk_placer" || f.trunkKind == "minecraft:mega_jungle_trunk_placer" {
		baseWidth = 2
	}
	for dx := 0; dx < baseWidth; dx++ {
		for dz := 0; dz < baseWidth; dz++ {
			for layer := -1; layer < height; layer++ {
				position := blockPosition{x: origin.x + dx, y: origin.y + layer, z: origin.z + dz}
				if f.isTreeBlock(getTreeBlock(chunk, position)) {
					return false
				}
			}
		}
	}
	freeHeight := height
	for layer := 0; layer <= height+1; layer++ {
		size := f.upperSize
		if layer < f.limit {
			size = f.lowerSize
		} else if f.sizeKind == "minecraft:three_layers_feature_size" && layer < height-f.upperLimit {
			size = f.middleSize
		}
		blocked := false
		for dx := -size; dx <= size && !blocked; dx++ {
			for dz := -size; dz <= size; dz++ {
				p := blockPosition{x: origin.x + dx, y: origin.y + layer, z: origin.z + dz}
				if !f.freePosition(chunk, p) || !f.ignoreVines && block.NameFromState(getTreeBlock(chunk, p)) == "minecraft:vine" {
					freeHeight = layer - 2
					blocked = true
					break
				}
			}
		}
		if blocked {
			break
		}
	}
	if freeHeight < height && (f.minClippedHeight == nil || freeHeight < *f.minClippedHeight) {
		return false
	}
	if freeHeight <= 0 {
		return false
	}
	if f.mangrove != nil && !f.placeMangroveRoots(system, chunk, random, rootOrigin, origin) {
		return false
	}
	below := blockPosition{x: origin.x, y: origin.y - 1, z: origin.z}
	belowState := getTreeBlock(chunk, below)
	_, isDirt := f.dirtStates[belowState]
	if f.forceDirt || !isDirt || block.NameFromState(belowState) == "minecraft:grass_block" || block.NameFromState(belowState) == "minecraft:mycelium" {
		setTreeBlock(chunk, below, f.dirt.state(system, chunk, random, below))
	}
	var attachments []treeAttachment
	var logs []blockPosition
	if f.trunkKind == "minecraft:fancy_trunk_placer" {
		attachments, logs = f.placeFancyTrunk(system, chunk, random, origin, freeHeight)
	} else if f.trunkKind == "minecraft:forking_trunk_placer" {
		attachments, logs = f.placeForkingTrunk(system, chunk, random, origin, freeHeight)
	} else if f.trunkKind == "minecraft:dark_oak_trunk_placer" {
		attachments, logs = f.placeDarkOakTrunk(system, chunk, random, origin, freeHeight)
	} else if f.trunkKind == "minecraft:giant_trunk_placer" || f.trunkKind == "minecraft:mega_jungle_trunk_placer" {
		attachments, logs = f.placeGiantTrunk(system, chunk, random, origin, freeHeight)
	} else if f.trunkKind == "minecraft:cherry_trunk_placer" {
		attachments, logs = f.placeCherryTrunk(system, chunk, random, origin, freeHeight)
	} else if f.trunkKind == "minecraft:bending_trunk_placer" {
		attachments, logs = f.placeBendingTrunk(system, chunk, random, origin, freeHeight)
	} else if f.trunkKind == "minecraft:upwards_branching_trunk_placer" {
		attachments, logs = f.placeMangroveTrunk(system, chunk, random, origin, freeHeight)
	} else {
		for layer := 0; layer < freeHeight; layer++ {
			p := blockPosition{x: origin.x, y: origin.y + layer, z: origin.z}
			if f.placeLog(system, chunk, random, p, "y") {
				logs = append(logs, p)
			}
		}
		attachments = []treeAttachment{{position: blockPosition{x: origin.x, y: origin.y + freeHeight, z: origin.z}}}
	}
	var leaves []blockPosition
	for _, attachment := range attachments {
		if f.foliageKind == "minecraft:cherry_foliage_placer" {
			leaves = append(leaves, f.placeCherryFoliage(system, chunk, random, attachment, foliageHeight, radius)...)
		} else if f.foliageKind == "minecraft:random_spread_foliage_placer" {
			leaves = append(leaves, f.placeRandomSpreadFoliage(system, chunk, random, attachment, foliageHeight, radius)...)
		} else if f.foliageKind == "minecraft:jungle_foliage_placer" || f.foliageKind == "minecraft:mega_pine_foliage_placer" {
			leaves = append(leaves, f.placeGiantFoliage(system, chunk, random, attachment, foliageHeight, radius)...)
		} else {
			leaves = append(leaves, f.placeFoliage(system, chunk, random, attachment, foliageHeight, radius)...)
		}
	}
	if len(logs) == 0 && len(leaves) == 0 {
		return false
	}
	f.placeDecorators(system, chunk, random, logs, leaves)
	f.updateLeafDistances(chunk, logs, leaves)
	return true
}

// isTreeBlock reports whether an existing trunk or canopy occupies a tree base.
func (f *treeFeature) isTreeBlock(state int32) bool {
	if _, ok := f.logs[state]; ok {
		return true
	}
	_, ok := f.leaves[state]
	return ok
}

// placeLog places a trunk state when the target is replaceable.
func (f *treeFeature) placeLog(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, position blockPosition, axis string) bool {
	if !f.validTrunkPosition(chunk, position) {
		return false
	}
	state := f.trunk.state(system, chunk, random, position)
	if axis != "y" {
		if value, err := block.BlockStateFromID(state); err == nil {
			if rotated, err := value.WithProperty("axis", axis); err == nil {
				state = rotated.ID()
			}
		}
	}
	setTreeBlock(chunk, position, state)
	return true
}

// placeFoliage places a canopy around one trunk attachment.
func (f *treeFeature) placeFoliage(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, attachment treeAttachment, height, radius int) []blockPosition {
	offset := f.offset.sample(random)
	center := attachment.position
	var leaves []blockPosition
	row := func(yOffset, rowRadius int) {
		endRadius := rowRadius
		if attachment.doubleTrunk {
			endRadius++
		}
		for dx := -rowRadius; dx <= endRadius; dx++ {
			for dz := -rowRadius; dz <= endRadius; dz++ {
				skip := false
				switch f.foliageKind {
				case "minecraft:blob_foliage_placer":
					if abs(dx) == rowRadius && abs(dz) == rowRadius {
						skip = random.NextIntBound(2) == 0 || yOffset == 0
					}
				case "minecraft:bush_foliage_placer":
					if abs(dx) == rowRadius && abs(dz) == rowRadius {
						skip = random.NextIntBound(2) == 0
					}
				case "minecraft:fancy_foliage_placer":
					x, z := float32(abs(dx))+.5, float32(abs(dz))+.5
					skip = x*x+z*z > float32(rowRadius*rowRadius)
				case "minecraft:acacia_foliage_placer":
					if yOffset == offset {
						skip = (abs(dx) > 1 || abs(dz) > 1) && dx != 0 && dz != 0
					} else {
						skip = rowRadius > 0 && abs(dx) == rowRadius && abs(dz) == rowRadius
					}
				case "minecraft:dark_oak_foliage_placer":
					if attachment.doubleTrunk && yOffset == offset && (dx == -rowRadius || dx == endRadius) && (dz == -rowRadius || dz == endRadius) {
						skip = true
					} else if yOffset == offset-1 && !attachment.doubleTrunk {
						skip = abs(dx) == rowRadius && abs(dz) == rowRadius
					} else if yOffset == offset+1 {
						skip = abs(dx)+abs(dz) > rowRadius*2-2
					}
				default:
					skip = rowRadius > 0 && abs(dx) == rowRadius && abs(dz) == rowRadius
				}
				if skip {
					continue
				}
				p := blockPosition{x: center.x + dx, y: center.y + yOffset, z: center.z + dz}
				if !f.validPosition(chunk, p) {
					continue
				}
				state := f.foliage.state(system, chunk, random, p)
				setTreeBlock(chunk, p, state)
				leaves = append(leaves, p)
			}
		}
	}
	switch f.foliageKind {
	case "minecraft:dark_oak_foliage_placer":
		if attachment.doubleTrunk {
			row(offset-1, radius+2)
			row(offset, radius+3)
			row(offset+1, radius+2)
			if random.NextBoolean() {
				row(offset+2, radius)
			}
		} else {
			row(offset-1, radius+2)
			row(offset, radius+1)
		}
	case "minecraft:acacia_foliage_placer":
		row(offset-1-height, radius+attachment.radiusOffset)
		row(offset-height, radius-1)
		row(offset, radius+attachment.radiusOffset-1)
	case "minecraft:blob_foliage_placer", "minecraft:bush_foliage_placer", "minecraft:fancy_foliage_placer":
		for y := offset; y >= offset-height; y-- {
			r := radius
			if f.foliageKind == "minecraft:blob_foliage_placer" {
				r = max(radius-1-y/2, 0)
			} else if f.foliageKind == "minecraft:bush_foliage_placer" {
				r = radius - 1 - y
			} else if y != offset && y != offset-height {
				r++
			}
			row(y, r)
		}
	case "minecraft:spruce_foliage_placer":
		r, limit, reset := int(random.NextIntBound(2)), 1, 0
		for y := offset; y >= -height; y-- {
			row(y, r)
			if r >= limit {
				r, reset, limit = reset, 1, min(limit+1, radius)
			} else {
				r++
			}
		}
	case "minecraft:pine_foliage_placer":
		r := 0
		for y := offset; y >= offset-height; y-- {
			row(y, r)
			if r >= 1 && y == offset-height+1 {
				r--
			} else if r < radius {
				r++
			}
		}
	}
	return leaves
}

// leafDistanceScratch retains worker-local traversal storage between trees.
type leafDistanceScratch struct {
	leaves    map[blockPosition]struct{}
	seen      map[blockPosition]struct{}
	frontiers [2][]blockPosition
}

var leafDistancePool = sync.Pool{New: func() any {
	return &leafDistanceScratch{
		leaves: make(map[blockPosition]struct{}, 256),
		seen:   make(map[blockPosition]struct{}, 256),
	}
}}

// updateLeafDistances connects generated leaves to nearby trunk blocks.
func (f *treeFeature) updateLeafDistances(chunk *section.Chunk, logs, leaves []blockPosition) {
	scratch := leafDistancePool.Get().(*leafDistanceScratch)
	defer leafDistancePool.Put(scratch)
	clear(scratch.leaves)
	clear(scratch.seen)
	for _, p := range leaves {
		scratch.leaves[p] = struct{}{}
	}
	current := logs
	for _, p := range logs {
		scratch.seen[p] = struct{}{}
	}
	for distance := 1; distance < 7 && len(current) > 0; distance++ {
		next := scratch.frontiers[distance&1][:0]
		for _, p := range current {
			for _, direction := range adjacentDirections {
				q := blockPosition{x: p.x + direction[0], y: p.y + direction[1], z: p.z + direction[2]}
				if _, ok := scratch.leaves[q]; !ok {
					continue
				}
				if _, ok := scratch.seen[q]; ok {
					continue
				}
				scratch.seen[q] = struct{}{}
				state := getTreeBlock(chunk, q)
				if state < 0 || int(state) >= len(leafDistanceStates) {
					continue
				}
				withDistance := leafDistanceStates[state][distance]
				if withDistance < 0 {
					continue
				}
				setTreeBlock(chunk, q, withDistance)
				next = append(next, q)
			}
		}
		scratch.frontiers[distance&1] = next
		current = next
	}
}

// placeFancyTrunk builds a branching trunk and returns its foliage points.
func (f *treeFeature) placeFancyTrunk(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition, height int) ([]treeAttachment, []blockPosition) {
	treeHeight := height + 2
	trunkHeight := int(math.Floor(float64(treeHeight) * .618))
	clusters := min(1, int(math.Floor(1.382+math.Pow(float64(treeHeight)/13, 2))))
	branchTop := origin.y + trunkHeight
	attachments := []treeAttachment{{position: blockPosition{x: origin.x, y: origin.y + treeHeight - 5, z: origin.z}, branchBase: branchTop}}
	for layer := treeHeight - 5; layer >= 0; layer-- {
		shape := fancyTreeShape(treeHeight, layer)
		if shape < 0 {
			continue
		}
		for cluster := 0; cluster < clusters; cluster++ {
			length := float64(shape) * (float64(random.NextFloat()) + .328)
			angle := float64(random.NextFloat()*2) * math.Pi
			p := blockPosition{x: origin.x + int(math.Floor(length*math.Sin(angle)+.5)), y: origin.y + layer - 1, z: origin.z + int(math.Floor(length*math.Cos(angle)+.5))}
			q := blockPosition{x: p.x, y: p.y + 5, z: p.z}
			if !f.fancyLimb(system, chunk, random, p, q, false, nil) {
				continue
			}
			dx, dz := origin.x-p.x, origin.z-p.z
			base := float64(p.y) - math.Sqrt(float64(dx*dx+dz*dz))*.381
			baseY := min(branchTop, int(base))
			start := blockPosition{x: origin.x, y: baseY, z: origin.z}
			if f.fancyLimb(system, chunk, random, start, p, false, nil) {
				attachments = append(attachments, treeAttachment{position: p, branchBase: baseY})
			}
		}
	}
	var logs []blockPosition
	f.fancyLimb(system, chunk, random, origin, blockPosition{x: origin.x, y: origin.y + trunkHeight, z: origin.z}, true, &logs)
	for _, attachment := range attachments {
		if float64(attachment.branchBase-origin.y) < float64(treeHeight)*.2 {
			continue
		}
		start := blockPosition{x: origin.x, y: attachment.branchBase, z: origin.z}
		if start != attachment.position {
			f.fancyLimb(system, chunk, random, start, attachment.position, true, &logs)
		}
	}
	kept := attachments[:0]
	for _, attachment := range attachments {
		if float64(attachment.branchBase-origin.y) >= float64(treeHeight)*.2 {
			kept = append(kept, attachment)
		}
	}
	return kept, logs
}

// fancyTreeShape calculates the branch spread at a trunk layer.
func fancyTreeShape(height, layer int) float32 {
	if float32(layer) < float32(height)*.3 {
		return -1
	}
	half := float32(height) / 2
	distance := half - float32(layer)
	if absFloat32(distance) >= half {
		return 0
	}
	return float32(math.Sqrt(float64(half*half-distance*distance))) * .5
}

// fancyLimb checks or places a line of trunk blocks.
func (f *treeFeature) fancyLimb(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, start, end blockPosition, place bool, logs *[]blockPosition) bool {
	dx, dy, dz := end.x-start.x, end.y-start.y, end.z-start.z
	steps := max(abs(dx), abs(dy), abs(dz))
	if steps == 0 && !place {
		return true
	}
	if steps == 0 {
		steps = 1
	}
	for step := 0; step <= steps; step++ {
		p := blockPosition{x: start.x + int(math.Floor(float64(float32(.5)+float32(step)*float32(dx)/float32(steps)))), y: start.y + int(math.Floor(float64(float32(.5)+float32(step)*float32(dy)/float32(steps)))), z: start.z + int(math.Floor(float64(float32(.5)+float32(step)*float32(dz)/float32(steps))))}
		if !place {
			if !f.freePosition(chunk, p) {
				return false
			}
			continue
		}
		axis := "y"
		if abs(p.x-start.x) > 0 || abs(p.z-start.z) > 0 {
			if abs(p.x-start.x) >= abs(p.z-start.z) {
				axis = "x"
			} else {
				axis = "z"
			}
		}
		if f.placeLog(system, chunk, random, p, axis) {
			*logs = append(*logs, p)
		}
	}
	return true
}
