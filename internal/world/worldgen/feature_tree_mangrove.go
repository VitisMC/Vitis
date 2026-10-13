package worldgen

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type mangroveTreeConfig struct {
	trunkOffset, branchSteps, branchLength         intProvider
	branchProbability, skewChance, aboveChance     float32
	maxRootLength, maxRootWidth                    int
	root, muddyRoot, aboveRoot                     stateProvider
	trunkGrowThrough, rootGrowThrough, muddyStates map[int32]struct{}
}

// decodeMangroveTreeConfig compiles root and branching rules from the tree definition.
func decodeMangroveTreeConfig(snapshot *datapack.Snapshot, raw, steps, length json.RawMessage, growThrough string, probability float32) (*mangroveTreeConfig, error) {
	var data struct {
		Type        string          `json:"type"`
		TrunkOffset json.RawMessage `json:"trunk_offset_y"`
		Root        json.RawMessage `json:"root_provider"`
		Above       struct {
			Provider json.RawMessage `json:"above_root_provider"`
			Chance   float32         `json:"above_root_placement_chance"`
		} `json:"above_root_placement"`
		Placement struct {
			GrowThrough   string          `json:"can_grow_through"`
			MaxLength     int             `json:"max_root_length"`
			MaxWidth      int             `json:"max_root_width"`
			Muddy         []string        `json:"muddy_roots_in"`
			MuddyProvider json.RawMessage `json:"muddy_roots_provider"`
			Skew          float32         `json:"random_skew_chance"`
		} `json:"mangrove_root_placement"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	if data.Type != "minecraft:mangrove_root_placer" || probability < 0 || probability > 1 || data.Placement.MaxLength < 1 || data.Placement.MaxLength > 15 || data.Placement.MaxWidth < 1 || data.Placement.MaxWidth > 8 || data.Placement.Skew < 0 || data.Placement.Skew > 1 || data.Above.Chance < 0 || data.Above.Chance > 1 {
		return nil, fmt.Errorf("invalid mangrove placement")
	}
	c := &mangroveTreeConfig{branchProbability: probability, skewChance: data.Placement.Skew, aboveChance: data.Above.Chance, maxRootLength: data.Placement.MaxLength, maxRootWidth: data.Placement.MaxWidth, muddyStates: make(map[int32]struct{})}
	var err error
	if c.trunkOffset, err = decodeIntProviderRange(data.TrunkOffset, 0, 16); err != nil {
		return nil, err
	}
	if c.branchSteps, err = decodeIntProviderRange(steps, 1, 16); err != nil {
		return nil, err
	}
	if c.branchLength, err = decodeIntProviderRange(length, 0, 16); err != nil {
		return nil, err
	}
	if c.root, err = decodeStateProvider(data.Root); err != nil {
		return nil, err
	}
	if c.muddyRoot, err = decodeStateProvider(data.Placement.MuddyProvider); err != nil {
		return nil, err
	}
	if len(data.Above.Provider) > 0 {
		if c.aboveRoot, err = decodeStateProvider(data.Above.Provider); err != nil {
			return nil, err
		}
	}
	resolve := func(name string) (map[int32]struct{}, error) {
		if !strings.HasPrefix(name, "#") {
			return nil, fmt.Errorf("expected block tag: %s", name)
		}
		names, err := snapshot.ResolveTag("block", strings.TrimPrefix(name, "#"))
		if err != nil {
			return nil, err
		}
		states := make(map[int32]struct{})
		for _, name := range names {
			info := block.InfoByName(name)
			if info == nil {
				return nil, fmt.Errorf("unknown block %s", name)
			}
			for id := info.MinStateID; id <= info.MaxStateID; id++ {
				states[id] = struct{}{}
			}
		}
		return states, nil
	}
	if c.trunkGrowThrough, err = resolve(growThrough); err != nil {
		return nil, err
	}
	if c.rootGrowThrough, err = resolve(data.Placement.GrowThrough); err != nil {
		return nil, err
	}
	for _, name := range data.Placement.Muddy {
		info := block.InfoByName(name)
		if info == nil {
			return nil, fmt.Errorf("unknown muddy root block %s", name)
		}
		for id := info.MinStateID; id <= info.MaxStateID; id++ {
			c.muddyStates[id] = struct{}{}
		}
	}
	return c, nil
}

// placeMangroveRoots validates each root path before writing any blocks.
func (f *treeFeature) placeMangroveRoots(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, ground, trunk blockPosition) bool {
	c := f.mangrove
	canRoot := func(p blockPosition) bool {
		if p.y < chunk.MinY() || p.y > chunk.MaxY() {
			return false
		}
		if p.x < int(chunk.X)*16-16 || p.x >= int(chunk.X)*16+32 || p.z < int(chunk.Z)*16-16 || p.z >= int(chunk.Z)*16+32 {
			return false
		}
		state := getTreeBlock(chunk, p)
		if block.IsAir(state) {
			return true
		}
		if _, ok := f.replaceable[state]; ok {
			return true
		}
		_, ok := c.rootGrowThrough[state]
		return ok
	}
	for p := ground; p.y < trunk.y; p.y++ {
		if !canRoot(p) {
			return false
		}
	}
	roots := []blockPosition{{trunk.x, trunk.y - 1, trunk.z}}
	for _, direction := range treeHorizontalDirections {
		first := blockPosition{trunk.x + direction[0], trunk.y, trunk.z + direction[1]}
		path := make([]blockPosition, 0, c.maxRootLength)
		var visit func(blockPosition, int) bool
		visit = func(p blockPosition, depth int) bool {
			if depth == c.maxRootLength || len(path) > c.maxRootLength {
				return false
			}
			below := blockPosition{p.x, p.y - 1, p.z}
			forward := blockPosition{p.x + direction[0], p.y, p.z + direction[1]}
			width := abs(p.x-trunk.x) + abs(p.y-trunk.y) + abs(p.z-trunk.z)
			choices := []blockPosition{below}
			if width > c.maxRootWidth-3 && width <= c.maxRootWidth && random.NextFloat() < c.skewChance {
				choices = append(choices, blockPosition{forward.x, forward.y - 1, forward.z})
			} else if width <= c.maxRootWidth && width <= c.maxRootWidth-3 && random.NextFloat() >= c.skewChance && random.NextBoolean() {
				choices[0] = forward
			}
			for _, next := range choices {
				if canRoot(next) {
					path = append(path, next)
					if !visit(next, depth+1) {
						return false
					}
				}
			}
			return true
		}
		if !visit(first, 0) {
			return false
		}
		roots = append(roots, path...)
		roots = append(roots, first)
	}
	for _, p := range roots {
		if !canRoot(p) {
			continue
		}
		state := getTreeBlock(chunk, p)
		provider := c.root
		if _, ok := c.muddyStates[state]; ok {
			provider = c.muddyRoot
		}
		setTreeBlock(chunk, p, waterlogTreeState(provider.state(system, chunk, random, p), state))
		above := blockPosition{p.x, p.y + 1, p.z}
		if c.aboveRoot != nil && random.NextFloat() < c.aboveChance && block.IsAir(getTreeBlock(chunk, above)) {
			setTreeBlock(chunk, above, c.aboveRoot.state(system, chunk, random, above))
		}
	}
	return true
}

// waterlogTreeState preserves fluid when a root replaces water.
func waterlogTreeState(state, old int32) int32 {
	if block.NameFromState(old) != "minecraft:water" {
		return state
	}
	value, err := block.BlockStateFromID(state)
	if err != nil {
		return state
	}
	if waterlogged, err := value.WithProperty("waterlogged", "true"); err == nil {
		return waterlogged.ID()
	}
	return state
}

// placeMangroveTrunk grows a vertical stem and upward side branches.
func (f *treeFeature) placeMangroveTrunk(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition, height int) ([]treeAttachment, []blockPosition) {
	c := f.mangrove
	var attachments []treeAttachment
	var logs []blockPosition
	for layer := 0; layer < height; layer++ {
		p := blockPosition{origin.x, origin.y + layer, origin.z}
		if f.placeLog(system, chunk, random, p, "y") {
			logs = append(logs, p)
			if layer < height-1 && random.NextFloat() < c.branchProbability {
				d := treeHorizontalDirections[int(random.NextIntBound(4))]
				branchLength := c.branchLength.sample(random)
				start := max(0, branchLength-c.branchLength.sample(random)-1)
				steps := c.branchSteps.sample(random)
				bx, bz := p.x, p.z
				lastY := p.y + start
				for step := start; step < height && steps > 0; step, steps = step+1, steps-1 {
					if step < 1 {
						continue
					}
					bx += d[0]
					bz += d[1]
					bp := blockPosition{bx, p.y + step, bz}
					if f.placeLog(system, chunk, random, bp, "y") {
						logs = append(logs, bp)
						lastY = bp.y + 1
					} else {
						lastY = bp.y
					}
					attachments = append(attachments, treeAttachment{position: bp})
				}
				if lastY-p.y > 1 {
					attachments = append(attachments, treeAttachment{position: blockPosition{bx, lastY, bz}}, treeAttachment{position: blockPosition{bx, lastY - 2, bz}})
				}
			}
		}
		if layer == height-1 {
			attachments = append(attachments, treeAttachment{position: blockPosition{origin.x, p.y + 1, origin.z}})
		}
	}
	return attachments, logs
}
