package worldgen

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/vitismc/vitis/internal/block"
	blockfluid "github.com/vitismc/vitis/internal/block/fluid"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type rootSystemFeature struct {
	tree                                                                  placedFeatureReference
	allowedTreePosition                                                   blockPredicate
	rootReplaceable                                                       map[int32]struct{}
	rootState, hangingState                                               stateProvider
	columnHeight, requiredSpace, allowedWater                             int
	rootRadius, rootAttempts, hangingRadius, hangingSpan, hangingAttempts int
}

// decodeRootSystemFeature compiles the tree and root placement rules.
func decodeRootSystemFeature(snapshot *datapack.Snapshot, raw json.RawMessage) (configuredFeature, bool, error) {
	var value struct {
		Config struct {
			AllowedTreePosition json.RawMessage `json:"allowed_tree_position"`
			AllowedWater        int             `json:"allowed_vertical_water_for_tree"`
			Feature             json.RawMessage `json:"feature"`
			HangingAttempts     int             `json:"hanging_root_placement_attempts"`
			HangingRadius       int             `json:"hanging_root_radius"`
			HangingState        json.RawMessage `json:"hanging_root_state_provider"`
			HangingSpan         int             `json:"hanging_roots_vertical_span"`
			RequiredSpace       int             `json:"required_vertical_space_for_tree"`
			ColumnHeight        int             `json:"root_column_max_height"`
			RootAttempts        int             `json:"root_placement_attempts"`
			RootRadius          int             `json:"root_radius"`
			RootReplaceable     string          `json:"root_replaceable"`
			RootState           json.RawMessage `json:"root_state_provider"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, true, err
	}
	c := value.Config
	if c.ColumnHeight < 1 || c.ColumnHeight > 4096 || c.RequiredSpace < 1 || c.RequiredSpace > 64 || c.AllowedWater < 1 || c.AllowedWater > 64 || c.RootRadius < 1 || c.RootRadius > 64 || c.HangingRadius < 1 || c.HangingRadius > 64 || c.HangingSpan < 0 || c.HangingSpan > 16 || c.RootAttempts < 1 || c.RootAttempts > 256 || c.HangingAttempts < 1 || c.HangingAttempts > 256 {
		return nil, true, fmt.Errorf("invalid root system bounds")
	}
	feature := &rootSystemFeature{columnHeight: c.ColumnHeight, requiredSpace: c.RequiredSpace, allowedWater: c.AllowedWater, rootRadius: c.RootRadius, rootAttempts: c.RootAttempts, hangingRadius: c.HangingRadius, hangingSpan: c.HangingSpan, hangingAttempts: c.HangingAttempts}
	var err error
	feature.tree, err = decodePlacedFeatureReference(snapshot, c.Feature)
	if errors.Is(err, errUnsupportedPlacedFeature) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, fmt.Errorf("root tree: %w", err)
	}
	feature.allowedTreePosition, err = decodeBlockPredicateWithSnapshot(snapshot, c.AllowedTreePosition)
	if err != nil {
		return nil, true, fmt.Errorf("root tree position: %w", err)
	}
	feature.rootState, err = decodeStateProvider(c.RootState)
	if err != nil {
		return nil, true, fmt.Errorf("root state: %w", err)
	}
	feature.hangingState, err = decodeStateProvider(c.HangingState)
	if err != nil {
		return nil, true, fmt.Errorf("hanging root state: %w", err)
	}
	if !strings.HasPrefix(c.RootReplaceable, "#") {
		return nil, true, fmt.Errorf("root replaceable must be a tag")
	}
	names, err := snapshot.ResolveTag("block", normalizeResourceID(strings.TrimPrefix(c.RootReplaceable, "#")))
	if err != nil {
		return nil, true, fmt.Errorf("root replaceable: %w", err)
	}
	feature.rootReplaceable = make(map[int32]struct{})
	for _, name := range names {
		info := block.InfoByName(name)
		if info == nil {
			return nil, true, fmt.Errorf("unknown root replaceable block %s", name)
		}
		for state := info.MinStateID; state <= info.MaxStateID; state++ {
			feature.rootReplaceable[state] = struct{}{}
		}
	}
	return feature, true, nil
}

// placeFeature searches for a tree site above the root column, then grows roots.
func (f *rootSystemFeature) placeFeature(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	if !isAirAt(chunk, origin) {
		return false
	}
	position := origin
	for step := 0; step < f.columnHeight; step++ {
		position.y++
		if position.y > chunk.MaxY() {
			break
		}
		if !f.allowedTreePosition.test(system, chunk, position) || !f.spaceForTree(chunk, position) {
			continue
		}
		below := blockPosition{x: position.x, y: position.y - 1, z: position.z}
		belowState := getChunkBlock(chunk, below)
		if !block.IsSolid(belowState) || blockfluid.GetFluidType(belowState) == blockfluid.FluidTypeLava {
			return false
		}
		if f.tree.place(system, chunk, random, position) {
			f.placeDirt(system, chunk, random, origin, step)
			f.placeHangingRoots(system, chunk, random, origin)
		}
		return true
	}
	return true
}

// spaceForTree checks the vertical space required above a candidate trunk.
func (f *rootSystemFeature) spaceForTree(chunk *section.Chunk, position blockPosition) bool {
	for distance := 1; distance <= f.requiredSpace; distance++ {
		above := blockPosition{x: position.x, y: position.y + distance, z: position.z}
		if above.y > chunk.MaxY() {
			return false
		}
		state := getChunkBlock(chunk, above)
		if block.IsAir(state) {
			continue
		}
		if distance+1 > f.allowedWater || !blockfluid.IsWater(state) {
			return false
		}
	}
	return true
}

// placeDirt scatters rooted dirt along the column below the generated tree.
func (f *rootSystemFeature) placeDirt(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition, height int) {
	for y := origin.y; y < origin.y+height; y++ {
		for attempt := 0; attempt < f.rootAttempts; attempt++ {
			p := blockPosition{x: origin.x + int(random.NextIntBound(int32(f.rootRadius))) - int(random.NextIntBound(int32(f.rootRadius))), y: y, z: origin.z + int(random.NextIntBound(int32(f.rootRadius))) - int(random.NextIntBound(int32(f.rootRadius)))}
			if _, ok := f.rootReplaceable[getChunkBlock(chunk, p)]; ok {
				setChunkBlock(chunk, p, f.rootState.state(system, chunk, random, p))
			}
		}
	}
}

// placeHangingRoots scatters roots under solid blocks near the column base.
func (f *rootSystemFeature) placeHangingRoots(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) {
	for attempt := 0; attempt < f.hangingAttempts; attempt++ {
		xOffset := int(random.NextIntBound(int32(f.hangingRadius))) - int(random.NextIntBound(int32(f.hangingRadius)))
		yOffset := 0
		if f.hangingSpan > 0 {
			yOffset = int(random.NextIntBound(int32(f.hangingSpan))) - int(random.NextIntBound(int32(f.hangingSpan)))
		}
		p := blockPosition{
			x: origin.x + xOffset,
			y: origin.y + yOffset,
			z: origin.z + int(random.NextIntBound(int32(f.hangingRadius))) - int(random.NextIntBound(int32(f.hangingRadius))),
		}
		if isAirAt(chunk, p) && block.IsSolid(getChunkBlock(chunk, blockPosition{x: p.x, y: p.y + 1, z: p.z})) {
			setChunkBlock(chunk, p, f.hangingState.state(system, chunk, random, p))
		}
	}
}
