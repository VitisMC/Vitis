package worldgen

import (
	"encoding/json"
	"fmt"

	"github.com/vitismc/vitis/internal/block"
	blockfluid "github.com/vitismc/vitis/internal/block/fluid"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
)

type blockPredicate func(*featureSystem, *section.Chunk, blockPosition) bool

var alwaysBlockPredicate blockPredicate = func(*featureSystem, *section.Chunk, blockPosition) bool { return true }

func (p blockPredicate) test(system *featureSystem, chunk *section.Chunk, position blockPosition) bool {
	return p != nil && p(system, chunk, position)
}

func decodeBlockPredicate(raw json.RawMessage) (blockPredicate, error) {
	return decodeBlockPredicateWithSnapshot(nil, raw)
}

// decodeBlockPredicateWithSnapshot resolves tag predicates against the active resources.
func decodeBlockPredicateWithSnapshot(snapshot *datapack.Snapshot, raw json.RawMessage) (blockPredicate, error) {
	var value struct {
		Type       string            `json:"type"`
		Offset     []int             `json:"offset"`
		Blocks     json.RawMessage   `json:"blocks"`
		Fluids     json.RawMessage   `json:"fluids"`
		State      BlockState        `json:"state"`
		Predicate  json.RawMessage   `json:"predicate"`
		Predicates []json.RawMessage `json:"predicates"`
		Tag        string            `json:"tag"`
		Direction  string            `json:"direction"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	offset := [3]int{}
	if len(value.Offset) != 0 {
		if len(value.Offset) != 3 {
			return nil, fmt.Errorf("invalid block predicate offset")
		}
		copy(offset[:], value.Offset)
	}
	at := func(position blockPosition) blockPosition {
		return blockPosition{x: position.x + offset[0], y: position.y + offset[1], z: position.z + offset[2]}
	}
	switch value.Type {
	case "minecraft:matching_block_tag":
		if snapshot == nil || value.Tag == "" {
			return nil, fmt.Errorf("missing block tag resources")
		}
		names, err := snapshot.ResolveTag("block", normalizeResourceID(value.Tag))
		if err != nil {
			return nil, err
		}
		states := make(map[int32]struct{})
		for _, name := range names {
			info := block.InfoByName(name)
			if info == nil {
				return nil, fmt.Errorf("unknown predicate tag block %s", name)
			}
			for id := info.MinStateID; id <= info.MaxStateID; id++ {
				states[id] = struct{}{}
			}
		}
		return func(_ *featureSystem, chunk *section.Chunk, position blockPosition) bool {
			_, ok := states[getChunkBlock(chunk, at(position))]
			return ok
		}, nil
	case "minecraft:matching_blocks":
		names, err := decodeStringList(value.Blocks)
		if err != nil {
			return nil, err
		}
		states := make(map[int32]struct{})
		for _, name := range names {
			info := block.InfoByName(normalizeResourceID(name))
			if info == nil {
				return nil, fmt.Errorf("unknown predicate block %s", name)
			}
			for id := info.MinStateID; id <= info.MaxStateID; id++ {
				states[id] = struct{}{}
			}
		}
		return func(_ *featureSystem, chunk *section.Chunk, position blockPosition) bool {
			_, ok := states[getChunkBlock(chunk, at(position))]
			return ok
		}, nil
	case "minecraft:matching_fluids":
		names, err := decodeStringList(value.Fluids)
		if err != nil {
			return nil, err
		}
		water, lava, empty := false, false, false
		for _, name := range names {
			switch normalizeResourceID(name) {
			case "minecraft:water", "minecraft:flowing_water":
				water = true
			case "minecraft:lava", "minecraft:flowing_lava":
				lava = true
			case "minecraft:empty":
				empty = true
			default:
				return nil, fmt.Errorf("unknown predicate fluid %s", name)
			}
		}
		return func(_ *featureSystem, chunk *section.Chunk, position blockPosition) bool {
			typeID := blockfluid.GetFluidType(getChunkBlock(chunk, at(position)))
			return water && typeID == blockfluid.FluidTypeWater || lava && typeID == blockfluid.FluidTypeLava || empty && typeID == blockfluid.FluidTypeNone
		}, nil
	case "minecraft:replaceable":
		return func(_ *featureSystem, chunk *section.Chunk, position blockPosition) bool {
			return generationBlockTraits(getChunkBlock(chunk, at(position)))&featureReplaceableTrait != 0
		}, nil
	case "minecraft:solid":
		return func(_ *featureSystem, chunk *section.Chunk, position blockPosition) bool {
			return generationBlockTraits(getChunkBlock(chunk, at(position)))&featureSolidTrait != 0
		}, nil
	case "minecraft:has_sturdy_face":
		face, ok := parseFeatureFace(value.Direction)
		if !ok {
			return nil, fmt.Errorf("invalid sturdy face direction %q", value.Direction)
		}
		return func(_ *featureSystem, chunk *section.Chunk, position blockPosition) bool {
			return featureFullFace(getChunkBlock(chunk, at(position)), face, false)
		}, nil
	case "minecraft:inside_world_bounds":
		return func(_ *featureSystem, chunk *section.Chunk, position blockPosition) bool {
			position = at(position)
			return position.y >= chunk.MinY() && position.y <= chunk.MaxY()
		}, nil
	case "minecraft:would_survive":
		state, err := block.BlockStateFromProperties(value.State.Name, value.State.Properties)
		if err != nil {
			return nil, err
		}
		return func(_ *featureSystem, chunk *section.Chunk, position blockPosition) bool {
			return canStateSurvive(chunk, at(position), state.ID())
		}, nil
	case "minecraft:not":
		child, err := decodeBlockPredicateWithSnapshot(snapshot, value.Predicate)
		if err != nil {
			return nil, err
		}
		return func(system *featureSystem, chunk *section.Chunk, position blockPosition) bool {
			return !child.test(system, chunk, at(position))
		}, nil
	case "minecraft:all_of", "minecraft:any_of":
		children := make([]blockPredicate, len(value.Predicates))
		for i, child := range value.Predicates {
			decoded, err := decodeBlockPredicateWithSnapshot(snapshot, child)
			if err != nil {
				return nil, err
			}
			children[i] = decoded
		}
		all := value.Type == "minecraft:all_of"
		return func(system *featureSystem, chunk *section.Chunk, position blockPosition) bool {
			position = at(position)
			for _, child := range children {
				matched := child.test(system, chunk, position)
				if all && !matched {
					return false
				}
				if !all && matched {
					return true
				}
			}
			return all
		}, nil
	default:
		return nil, fmt.Errorf("unsupported block predicate %q", value.Type)
	}
}

func decodeStringList(raw json.RawMessage) ([]string, error) {
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}, nil
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("empty identifier list")
	}
	return list, nil
}
