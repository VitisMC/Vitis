package worldgen

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// hugeFungusFeature retains immutable blocks and replacement predicates for Nether fungi.
type hugeFungusFeature struct {
	base, stem, hat, decor int32
	planted, crimson       bool
	replaceable            map[int32]struct{}
	stemReplaceable        blockPredicate
	vineHeads              [3]int32
	vineStem               int32
}

// decodeHugeFungusFeature resolves all state variants and the current resource replacement tag.
func decodeHugeFungusFeature(snapshot *datapack.Snapshot, raw json.RawMessage) (configuredFeature, bool, error) {
	var data struct {
		Config struct {
			Base        BlockState      `json:"valid_base_block"`
			Stem        BlockState      `json:"stem_state"`
			Hat         BlockState      `json:"hat_state"`
			Decor       BlockState      `json:"decor_state"`
			Planted     bool            `json:"planted"`
			Replaceable json.RawMessage `json:"replaceable_blocks"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, true, err
	}
	f := &hugeFungusFeature{planted: data.Config.Planted, replaceable: make(map[int32]struct{})}
	for _, pair := range []struct {
		data   BlockState
		result *int32
	}{{data.Config.Base, &f.base}, {data.Config.Stem, &f.stem}, {data.Config.Hat, &f.hat}, {data.Config.Decor, &f.decor}} {
		state, err := block.BlockStateFromProperties(pair.data.Name, pair.data.Properties)
		if err != nil {
			return nil, true, err
		}
		*pair.result = state.ID()
	}
	f.crimson = block.NameFromState(f.hat) == "minecraft:nether_wart_block"
	var err error
	f.stemReplaceable, err = decodeBlockPredicateWithSnapshot(snapshot, data.Config.Replaceable)
	if err != nil {
		return nil, true, err
	}
	if snapshot == nil {
		return nil, true, fmt.Errorf("huge fungus requires block tags")
	}
	names, err := snapshot.ResolveTag("block", "minecraft:replaceable")
	if err != nil {
		return nil, true, err
	}
	for _, name := range names {
		info := block.InfoByName(name)
		if info == nil {
			return nil, true, fmt.Errorf("unknown replaceable fungus block %s", name)
		}
		for state := info.MinStateID; state <= info.MaxStateID; state++ {
			f.replaceable[state] = struct{}{}
		}
	}
	f.vineStem = block.DefaultStateID("minecraft:weeping_vines_plant")
	for index := range f.vineHeads {
		f.vineHeads[index] = block.StateID("minecraft:weeping_vines", map[string]string{"age": strconv.Itoa(23 + index)})
	}
	return f, true, nil
}

// placeFeature builds a fungus with the stem, cap, and vine RNG sequence intact.
func (f *hugeFungusFeature) placeFeature(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	if block.BlockIDFromState(getTreeBlock(chunk, blockPosition{origin.x, origin.y - 1, origin.z})) != block.BlockIDFromState(f.base) {
		return false
	}
	height := 4 + int(random.NextIntBound(10))
	if random.NextIntBound(12) == 0 {
		height *= 2
	}
	depth := chunk.MaxY() - chunk.MinY() + 1
	if system != nil {
		depth = system.height
	}
	if !f.planted && origin.y+height+1 >= depth {
		return false
	}
	wide := !f.planted && random.NextFloat() < .06
	setTreeBlock(chunk, origin, 0)
	f.placeFungusStem(system, chunk, random, origin, height, wide)
	f.placeFungusCap(chunk, random, origin, height, wide)
	return true
}

// canReplace distinguishes replaceable blocks from the additional stem-only plant predicate.
func (f *hugeFungusFeature) canReplace(system *featureSystem, chunk *section.Chunk, p blockPosition, stem bool) bool {
	if p.y < chunk.MinY() || p.y > chunk.MaxY() {
		return false
	}
	if _, ok := f.replaceable[getTreeBlock(chunk, p)]; ok {
		return true
	}
	return stem && f.stemReplaceable.test(system, chunk, p)
}

// placeFungusStem leaves sparse corners on the rare wider natural stems.
func (f *hugeFungusFeature) placeFungusStem(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition, height int, wide bool) {
	radius := 0
	if wide {
		radius = 1
	}
	for dx := -radius; dx <= radius; dx++ {
		for dz := -radius; dz <= radius; dz++ {
			corner := wide && absInt(dx) == radius && absInt(dz) == radius
			for dy := 0; dy < height; dy++ {
				p := blockPosition{origin.x + dx, origin.y + dy, origin.z + dz}
				if f.canReplace(system, chunk, p, true) && (f.planted || !corner || random.NextFloat() < .1) {
					setTreeBlock(chunk, p, f.stem)
					chunk.RemoveBlockEntity(int32(p.x), int32(p.y), int32(p.z))
				}
			}
		}
	}
}

// placeFungusCap distinguishes hanging cap fringes, inner gaps, and lit corners.
func (f *hugeFungusFeature) placeFungusCap(chunk *section.Chunk, random worldrandom.Source, origin blockPosition, height int, wide bool) {
	capHeight := min(int(random.NextIntBound(int32(1+height/3)))+5, height)
	bottom := height - capHeight
	for dy := bottom; dy <= height; dy++ {
		radius := 1
		if dy < height-int(random.NextIntBound(3)) {
			radius = 2
		}
		if capHeight > 8 && dy < bottom+4 {
			radius = 3
		}
		if wide {
			radius++
		}
		for dx := -radius; dx <= radius; dx++ {
			for dz := -radius; dz <= radius; dz++ {
				p := blockPosition{origin.x + dx, origin.y + dy, origin.z + dz}
				if !f.canReplace(nil, chunk, p, false) {
					continue
				}
				edgeX, edgeZ := absInt(dx) == radius, absInt(dz) == radius
				inside := !edgeX && !edgeZ && dy != height
				if dy < bottom+3 {
					if inside {
						continue
					}
					below := getTreeBlock(chunk, blockPosition{p.x, p.y - 1, p.z})
					if block.BlockIDFromState(below) == block.BlockIDFromState(f.hat) {
						setTreeBlock(chunk, p, f.hat)
					} else if float64(random.NextFloat()) < .15 {
						setTreeBlock(chunk, p, f.hat)
						if f.crimson && random.NextIntBound(11) == 0 {
							f.placeFungusVines(chunk, random, p)
						}
					}
					continue
				}
				lightChance, hatChance, vineChance := float32(.0005), float32(.98), float32(.07)
				if inside {
					lightChance, hatChance, vineChance = .1, .2, .1
				} else if edgeX && edgeZ {
					lightChance, hatChance, vineChance = .01, .7, .083
				}
				if !f.crimson {
					vineChance = 0
				}
				if random.NextFloat() < lightChance {
					setTreeBlock(chunk, p, f.decor)
				} else if random.NextFloat() < hatChance {
					setTreeBlock(chunk, p, f.hat)
					if random.NextFloat() < vineChance {
						f.placeFungusVines(chunk, random, p)
					}
				}
			}
		}
	}
}

// placeFungusVines grows hanging vines with the narrower cap-specific head age range.
func (f *hugeFungusFeature) placeFungusVines(chunk *section.Chunk, random worldrandom.Source, cap blockPosition) {
	p := blockPosition{cap.x, cap.y - 1, cap.z}
	if !block.IsAir(getTreeBlock(chunk, p)) {
		return
	}
	height := 1 + int(random.NextIntBound(5))
	if random.NextIntBound(7) == 0 {
		height *= 2
	}
	for dy := 0; dy <= height && p.y >= chunk.MinY(); dy++ {
		if block.IsAir(getTreeBlock(chunk, p)) {
			if dy == height || !block.IsAir(getTreeBlock(chunk, blockPosition{p.x, p.y - 1, p.z})) {
				setTreeBlock(chunk, p, f.vineHeads[random.NextIntBound(3)])
				return
			}
			setTreeBlock(chunk, p, f.vineStem)
		}
		p.y--
	}
}
