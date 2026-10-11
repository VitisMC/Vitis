package worldgen

import (
	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// placeAlterGround replaces exposed soil in patches around the lowest trunk blocks.
func (f *treeFeature) placeAlterGround(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, decorator treeDecorator, logs []blockPosition) {
	if len(logs) == 0 {
		return
	}
	for _, origin := range logs {
		if origin.y != logs[0].y {
			break
		}
		for _, offset := range [][2]int{{-1, -1}, {2, -1}, {-1, 2}, {2, 2}} {
			f.placeGroundCircle(system, chunk, random, decorator.provider, blockPosition{origin.x + offset[0], origin.y, origin.z + offset[1]})
		}
		for attempt := 0; attempt < 5; attempt++ {
			value := int(random.NextIntBound(64))
			dx, dz := value%8, value/8
			if dx == 0 || dx == 7 || dz == 0 || dz == 7 {
				f.placeGroundCircle(system, chunk, random, decorator.provider, blockPosition{origin.x - 3 + dx, origin.y, origin.z - 3 + dz})
			}
		}
	}
}

// placeGroundCircle finds the first exposed dirt-tagged block in each rounded column.
func (f *treeFeature) placeGroundCircle(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, provider stateProvider, origin blockPosition) {
	for dx := -2; dx <= 2; dx++ {
		for dz := -2; dz <= 2; dz++ {
			if absInt(dx) == 2 && absInt(dz) == 2 {
				continue
			}
			column := blockPosition{origin.x + dx, origin.y, origin.z + dz}
			for dy := 2; dy >= -3; dy-- {
				p := blockPosition{column.x, column.y + dy, column.z}
				state := getTreeBlock(chunk, p)
				if _, dirt := f.dirtStates[state]; dirt {
					setTreeBlock(chunk, p, provider.state(system, chunk, random, column))
					break
				}
				if !block.IsAir(state) && dy < 0 {
					break
				}
			}
		}
	}
}

// placePaleMoss adds the ground patch and moss hangers without generating live entities.
func (f *treeFeature) placePaleMoss(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, decorator treeDecorator, logs, leaves []blockPosition) {
	shuffled := append([]blockPosition(nil), logs...)
	shuffleTreePositions(random, shuffled)
	if len(shuffled) == 0 {
		return
	}
	lowest := shuffled[0]
	for _, p := range shuffled {
		if p.y < lowest.y {
			lowest = p
		}
	}
	if random.NextFloat() < decorator.groundChance && system != nil && system.resources != nil {
		if patch := system.resources.configured["minecraft:pale_moss_patch"]; patch != nil {
			lowest.y++
			patch.placeFeature(system, chunk, random, lowest)
		}
	}
	for _, group := range []struct {
		positions []blockPosition
		chance    float32
	}{{logs, decorator.trunkChance}, {leaves, decorator.leavesChance}} {
		for _, p := range group.positions {
			if random.NextFloat() < group.chance {
				p.y--
				if f.treeDecoratorAir(chunk, p) {
					f.placeMossHanger(chunk, random, p)
				}
			}
		}
	}
}

// placeMossHanger grows a contiguous column ending in a single moss tip.
func (f *treeFeature) placeMossHanger(chunk *section.Chunk, random worldrandom.Source, p blockPosition) {
	stem := block.StateID("minecraft:pale_hanging_moss", map[string]string{"tip": "false"})
	tip := block.StateID("minecraft:pale_hanging_moss", map[string]string{"tip": "true"})
	for f.treeDecoratorAir(chunk, blockPosition{p.x, p.y - 1, p.z}) && float64(random.NextFloat()) >= .5 {
		setTreeBlock(chunk, p, stem)
		p.y--
	}
	setTreeBlock(chunk, p, tip)
}

// shuffleTreePositions uses the backward Fisher-Yates order of decoration RNG draws.
func shuffleTreePositions(random worldrandom.Source, positions []blockPosition) {
	for length := len(positions); length > 1; length-- {
		index := int(random.NextIntBound(int32(length)))
		positions[length-1], positions[index] = positions[index], positions[length-1]
	}
}
