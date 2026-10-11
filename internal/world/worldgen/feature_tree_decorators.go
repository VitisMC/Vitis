package worldgen

import (
	"sort"
	"strconv"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type treeDecorator struct {
	kind                                    string
	probability                             float32
	provider                                stateProvider
	exclusionXZ, exclusionY, requiredEmpty  int
	directions                              [][3]int
	leavesChance, trunkChance, groundChance float32
}

type treeSide struct {
	dx, dz int
	vine   string
	facing string
}

var treeSides = [...]treeSide{
	{dx: -1, vine: "east", facing: "east"},
	{dx: 1, vine: "west", facing: "west"},
	{dz: -1, vine: "south", facing: "south"},
	{dz: 1, vine: "north", facing: "north"},
}

// placeDecorators applies the configured decorations after the trunk and crown exist.
func (f *treeFeature) placeDecorators(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, logs, leaves []blockPosition) {
	if len(f.decorators) == 0 {
		return
	}
	logs = uniqueTreePositions(logs)
	leaves = uniqueTreePositions(leaves)
	for _, decorator := range f.decorators {
		switch decorator.kind {
		case "minecraft:alter_ground":
			f.placeAlterGround(system, chunk, random, decorator, logs)
		case "minecraft:attached_to_leaves":
			f.placeAttachedToLeaves(system, chunk, random, decorator, leaves)
		case "minecraft:pale_moss":
			f.placePaleMoss(system, chunk, random, decorator, logs, leaves)
		case "minecraft:creaking_heart":
			f.placeCreakingHeart(chunk, random, decorator.probability, logs)
		case "minecraft:beehive":
			f.placeBeeNest(chunk, random, decorator.probability, logs, leaves)
		case "minecraft:trunk_vine":
			for _, p := range logs {
				for _, side := range treeSides {
					if random.NextIntBound(3) > 0 {
						f.placeTreeVine(chunk, blockPosition{x: p.x + side.dx, y: p.y, z: p.z + side.dz}, side.vine)
					}
				}
			}
		case "minecraft:leave_vine":
			for _, p := range leaves {
				for _, side := range treeSides {
					if random.NextFloat() >= decorator.probability {
						continue
					}
					vine := blockPosition{x: p.x + side.dx, y: p.y, z: p.z + side.dz}
					if !f.placeTreeVine(chunk, vine, side.vine) {
						continue
					}
					for length := 0; length < 4; length++ {
						vine.y--
						if !f.placeTreeVine(chunk, vine, side.vine) {
							break
						}
					}
				}
			}
		case "minecraft:cocoa":
			if random.NextFloat() >= decorator.probability || len(logs) == 0 {
				continue
			}
			baseY := logs[0].y
			for _, p := range logs {
				if p.y-baseY > 2 {
					break
				}
				for _, side := range treeSides {
					if random.NextFloat() > .25 {
						continue
					}
					position := blockPosition{x: p.x + side.dx, y: p.y, z: p.z + side.dz}
					if !f.treeDecoratorAir(chunk, position) {
						continue
					}
					state, err := block.DefaultBlockState("minecraft:cocoa")
					if err != nil {
						continue
					}
					state, err = state.WithProperty("age", strconv.Itoa(int(random.NextIntBound(3))))
					if err != nil {
						continue
					}
					if state, err = state.WithProperty("facing", side.facing); err == nil {
						setTreeBlock(chunk, position, state.ID())
					}
				}
			}
		}
	}
}

// placeTreeVine attaches a vine only to an empty position in the nearby chunk area.
func (f *treeFeature) placeTreeVine(chunk *section.Chunk, position blockPosition, side string) bool {
	if !f.treeDecoratorAir(chunk, position) {
		return false
	}
	state, err := block.DefaultBlockState("minecraft:vine")
	if err != nil {
		return false
	}
	if state, err = state.WithProperty(side, "true"); err != nil {
		return false
	}
	setTreeBlock(chunk, position, state.ID())
	return true
}

// treeDecoratorAir checks the chunk extent before reading an empty placement target.
func (f *treeFeature) treeDecoratorAir(chunk *section.Chunk, position blockPosition) bool {
	if position.y < chunk.MinY() || position.y > chunk.MaxY() {
		return false
	}
	baseX, baseZ := int(chunk.X)*16, int(chunk.Z)*16
	if position.x < baseX || position.x >= baseX+16 || position.z < baseZ || position.z >= baseZ+16 {
		if !chunk.BorderBlocksEnabled() || position.x < baseX-16 || position.x >= baseX+32 || position.z < baseZ-16 || position.z >= baseZ+32 {
			return false
		}
	}
	return block.IsAir(getTreeBlock(chunk, position))
}

// uniqueTreePositions removes duplicate writes and orders blocks from bottom to top.
func uniqueTreePositions(positions []blockPosition) []blockPosition {
	result := append([]blockPosition(nil), positions...)
	sort.Slice(result, func(i, j int) bool {
		if result[i].y != result[j].y {
			return result[i].y < result[j].y
		}
		if result[i].x != result[j].x {
			return result[i].x < result[j].x
		}
		return result[i].z < result[j].z
	})
	unique := result[:0]
	for _, position := range result {
		if len(unique) == 0 || unique[len(unique)-1] != position {
			unique = append(unique, position)
		}
	}
	return unique
}
