package worldgen

import (
	"github.com/vitismc/vitis/internal/block"
	blockentity "github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// placeAttachedToLeaves applies directional decorations with spatial exclusion and clearance.
func (f *treeFeature) placeAttachedToLeaves(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, decorator treeDecorator, leaves []blockPosition) {
	positions := append([]blockPosition(nil), leaves...)
	shuffleTreePositions(random, positions)
	excluded := make(map[blockPosition]struct{})
	for _, leaf := range positions {
		offset := decorator.directions[random.NextIntBound(int32(len(decorator.directions)))]
		p := blockPosition{leaf.x + offset[0], leaf.y + offset[1], leaf.z + offset[2]}
		if _, blocked := excluded[p]; blocked {
			continue
		}
		if random.NextFloat() >= decorator.probability {
			continue
		}
		clear := true
		for distance := 1; distance <= decorator.requiredEmpty; distance++ {
			if !f.treeDecoratorAir(chunk, blockPosition{leaf.x + offset[0]*distance, leaf.y + offset[1]*distance, leaf.z + offset[2]*distance}) {
				clear = false
				break
			}
		}
		if !clear {
			continue
		}
		for dx := -decorator.exclusionXZ; dx <= decorator.exclusionXZ; dx++ {
			for dy := -decorator.exclusionY; dy <= decorator.exclusionY; dy++ {
				for dz := -decorator.exclusionXZ; dz <= decorator.exclusionXZ; dz++ {
					excluded[blockPosition{p.x + dx, p.y + dy, p.z + dz}] = struct{}{}
				}
			}
		}
		setTreeBlock(chunk, p, decorator.provider.state(system, chunk, random, p))
	}
}

// placeCreakingHeart replaces a fully enclosed log with a natural active heart.
func (f *treeFeature) placeCreakingHeart(chunk *section.Chunk, random worldrandom.Source, chance float32, logs []blockPosition) {
	if len(logs) == 0 || random.NextFloat() >= chance {
		return
	}
	positions := append([]blockPosition(nil), logs...)
	shuffleTreePositions(random, positions)
	for _, p := range positions {
		enclosed := true
		for _, offset := range [...][3]int{{0, -1, 0}, {0, 1, 0}, {0, 0, -1}, {0, 0, 1}, {-1, 0, 0}, {1, 0, 0}} {
			if _, log := f.logs[getTreeBlock(chunk, blockPosition{p.x + offset[0], p.y + offset[1], p.z + offset[2]})]; !log {
				enclosed = false
				break
			}
		}
		if !enclosed {
			continue
		}
		state := block.StateID("minecraft:creaking_heart", map[string]string{"active": "true", "natural": "true"})
		setTreeBlock(chunk, p, state)
		chunk.AddBlockEntity(blockentity.NewRawBlockEntity(featureBlockEntityNBT("minecraft:creaking_heart", p)))
		return
	}
}

// placeBeeNest persists occupied nests while leaving entity population to the runtime.
func (f *treeFeature) placeBeeNest(chunk *section.Chunk, random worldrandom.Source, chance float32, logs, leaves []blockPosition) {
	if len(logs) == 0 || random.NextFloat() >= chance {
		return
	}
	level := 0
	if len(leaves) != 0 {
		level = max(leaves[0].y-1, logs[0].y+1)
	} else {
		level = min(logs[0].y+1+int(random.NextIntBound(3)), logs[len(logs)-1].y)
	}
	var candidates []blockPosition
	for _, p := range logs {
		if p.y != level {
			continue
		}
		for _, offset := range [][2]int{{1, 0}, {0, 1}, {-1, 0}} {
			candidates = append(candidates, blockPosition{p.x + offset[0], p.y, p.z + offset[1]})
		}
	}
	shuffleTreePositions(random, candidates)
	for _, p := range candidates {
		if !f.treeDecoratorAir(chunk, p) || !f.treeDecoratorAir(chunk, blockPosition{p.x, p.y, p.z + 1}) {
			continue
		}
		state := block.StateID("minecraft:bee_nest", map[string]string{"facing": "south"})
		setTreeBlock(chunk, p, state)
		occupants := nbt.NewList(nbt.TagCompound)
		count := 2 + int(random.NextIntBound(2))
		for index := 0; index < count; index++ {
			occupants.Add(nbt.NewCompound().PutCompound("entity_data", nbt.NewCompound().PutString("id", "minecraft:bee")).PutInt("ticks_in_hive", random.NextIntBound(599)).PutInt("min_ticks_in_hive", 600))
		}
		chunk.AddBlockEntity(blockentity.NewRawBlockEntity(featureBlockEntityNBT("minecraft:beehive", p).PutList("bees", occupants)))
		return
	}
}
