package worldgen

import (
	"fmt"
	"strconv"

	"github.com/vitismc/vitis/internal/block"
	blockfluid "github.com/vitismc/vitis/internal/block/fluid"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type coralFeature struct {
	kind                   string
	blocks, fans, wallFans []int32
}

// decodeCoralFeature resolves the coral block and decoration tags.
func decodeCoralFeature(snapshot *datapack.Snapshot, kind string) (configuredFeature, bool, error) {
	f := &coralFeature{kind: kind}
	for _, item := range []struct {
		tag    string
		target *[]int32
	}{{"minecraft:coral_blocks", &f.blocks}, {"minecraft:corals", &f.fans}, {"minecraft:wall_corals", &f.wallFans}} {
		names, err := snapshot.ResolveTag("block", item.tag)
		if err != nil {
			return nil, true, err
		}
		for _, name := range names {
			if block.InfoByName(name) == nil {
				return nil, true, fmt.Errorf("unknown coral block %s", name)
			}
			*item.target = append(*item.target, block.DefaultStateID(name))
		}
	}
	if len(f.blocks) == 0 {
		return nil, true, fmt.Errorf("empty coral block tag")
	}
	return f, true, nil
}

// placeFeature grows one of the three vanilla coral shapes in water.
func (f *coralFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	state := f.blocks[random.NextIntBound(int32(len(f.blocks)))]
	switch f.kind {
	case "minecraft:coral_tree":
		p := origin
		for i := 0; i < 1+int(random.NextIntBound(3)); i++ {
			if !f.placeBlock(chunk, random, p, state) {
				return i > 0
			}
			p.y++
		}
		base := p
		branches := 2 + int(random.NextIntBound(3))
		order := []int{0, 1, 2, 3}
		shuffleCoralDirections(order, random)
		for _, index := range order[:branches] {
			d := treeHorizontalDirections[index]
			p = blockPosition{base.x + d[0], base.y, base.z + d[1]}
			length := 2 + int(random.NextIntBound(5))
			vertical := 0
			for i := 0; i < length && f.placeBlock(chunk, random, p, state); i++ {
				p.y++
				vertical++
				if i == 0 || vertical >= 2 && random.NextFloat() < 0.25 {
					p.x += d[0]
					p.z += d[1]
					vertical = 0
				}
			}
		}
		return true
	case "minecraft:coral_claw":
		if !f.placeBlock(chunk, random, origin, state) {
			return false
		}
		main := int(random.NextIntBound(4))
		order := []int{main, (main + 1) % 4, (main + 3) % 4}
		shuffleCoralDirections(order, random)
		branches := 2 + int(random.NextIntBound(2))
		for _, index := range order[:branches] {
			d := treeHorizontalDirections[index]
			p := blockPosition{origin.x + d[0], origin.y, origin.z + d[1]}
			if index != main {
				p.y++
			}
			initial := 1 + int(random.NextIntBound(2))
			for i := 0; i < initial && f.placeBlock(chunk, random, p, state); i++ {
				if index == main || random.NextBoolean() {
					p.x += d[0]
					p.z += d[1]
				} else {
					p.y++
				}
			}
			p.y++
			length := 2 + int(random.NextIntBound(4))
			if index != main {
				length++
			}
			mainDirection := treeHorizontalDirections[main]
			for i := 0; i < length; i++ {
				p.x += mainDirection[0]
				p.z += mainDirection[1]
				if !f.placeBlock(chunk, random, p, state) {
					break
				}
				if random.NextFloat() < 0.25 {
					p.y++
				}
			}
		}
		return true
	default:
		width, height, depth := 3+int(random.NextIntBound(3)), 3+int(random.NextIntBound(3)), 3+int(random.NextIntBound(3))
		down := 1 + int(random.NextIntBound(3))
		placed := false
		for dx := 0; dx <= width; dx++ {
			for dy := 0; dy <= height; dy++ {
				for dz := 0; dz <= depth; dz++ {
					xEdge := dx == 0 || dx == width
					yEdge := dy == 0 || dy == height
					zEdge := dz == 0 || dz == depth
					if (xEdge || yEdge) && (zEdge || yEdge) && (xEdge || zEdge) && (xEdge || yEdge || zEdge) && random.NextFloat() >= 0.1 {
						p := blockPosition{origin.x + dx, origin.y + dy - down, origin.z + dz}
						placed = f.placeBlock(chunk, random, p, state) || placed
					}
				}
			}
		}
		return placed
	}
}

// placeBlock places coral and its optional fans only when surrounded by water.
func (f *coralFeature) placeBlock(chunk *section.Chunk, random worldrandom.Source, p blockPosition, state int32) bool {
	current := getChunkBlock(chunk, p)
	if !blockfluid.IsWater(current) && !isCoralState(current) {
		return false
	}
	above := blockPosition{p.x, p.y + 1, p.z}
	if !blockfluid.IsWater(getChunkBlock(chunk, above)) {
		return false
	}
	setChunkBlock(chunk, p, state)
	if len(f.fans) > 0 && random.NextFloat() < 0.25 {
		setChunkBlock(chunk, above, f.fans[random.NextIntBound(int32(len(f.fans)))])
	} else if random.NextFloat() < 0.05 {
		pickles := 1 + int(random.NextIntBound(4))
		if value, err := block.BlockStateFromProperties("minecraft:sea_pickle", map[string]string{"pickles": strconv.Itoa(pickles), "waterlogged": "true"}); err == nil {
			setChunkBlock(chunk, above, value.ID())
		}
	}
	for index, d := range treeHorizontalDirections {
		if len(f.wallFans) == 0 || random.NextFloat() >= 0.2 {
			continue
		}
		side := blockPosition{p.x + d[0], p.y, p.z + d[1]}
		if !blockfluid.IsWater(getChunkBlock(chunk, side)) {
			continue
		}
		value, err := block.BlockStateFromID(f.wallFans[random.NextIntBound(int32(len(f.wallFans)))])
		if err != nil {
			continue
		}
		facing := []string{"north", "east", "south", "west"}[index]
		if oriented, err := value.WithProperty("facing", facing); err == nil {
			value = oriented
		}
		setChunkBlock(chunk, side, value.ID())
	}
	return true
}

// isCoralState reports whether a living coral block may be replaced by growth.
func isCoralState(state int32) bool {
	switch block.NameFromState(state) {
	case "minecraft:tube_coral", "minecraft:brain_coral", "minecraft:bubble_coral", "minecraft:fire_coral", "minecraft:horn_coral", "minecraft:tube_coral_block", "minecraft:brain_coral_block", "minecraft:bubble_coral_block", "minecraft:fire_coral_block", "minecraft:horn_coral_block":
		return true
	default:
		return false
	}
}

// shuffleCoralDirections draws a random order without allocating a permutation.
func shuffleCoralDirections(order []int, random worldrandom.Source) {
	for i := len(order) - 1; i > 0; i-- {
		j := int(random.NextIntBound(int32(i + 1)))
		order[i], order[j] = order[j], order[i]
	}
}
