package worldgen

import (
	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// placeDarkOakTrunk grows a two-by-two trunk with its upper branches.
func (f *treeFeature) placeDarkOakTrunk(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition, height int) ([]treeAttachment, []blockPosition) {
	for _, offset := range [][2]int{{1, 0}, {0, 1}, {1, 1}} {
		position := blockPosition{x: origin.x + offset[0], y: origin.y - 1, z: origin.z + offset[1]}
		state := getTreeBlock(chunk, position)
		_, isDirt := f.dirtStates[state]
		if f.forceDirt || !isDirt || block.NameFromState(state) == "minecraft:grass_block" || block.NameFromState(state) == "minecraft:mycelium" {
			setTreeBlock(chunk, position, f.dirt.state(system, chunk, random, position))
		}
	}
	direction := int(random.NextIntBound(4))
	bendStart := height - int(random.NextIntBound(4))
	bendSteps := 2 - int(random.NextIntBound(3))
	x, z := origin.x, origin.z
	topY := origin.y + height - 1
	var logs []blockPosition
	for layer := 0; layer < height; layer++ {
		if layer >= bendStart && bendSteps > 0 {
			x += treeHorizontalDirections[direction][0]
			z += treeHorizontalDirections[direction][1]
			bendSteps--
		}
		center := blockPosition{x: x, y: origin.y + layer, z: z}
		centerState := getTreeBlock(chunk, center)
		_, isLeaf := f.leaves[centerState]
		if !block.IsAir(centerState) && !isLeaf {
			continue
		}
		for _, offset := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
			position := blockPosition{x: center.x + offset[0], y: center.y, z: center.z + offset[1]}
			if f.placeLog(system, chunk, random, position, "y") {
				logs = append(logs, position)
			}
		}
	}
	attachments := []treeAttachment{{position: blockPosition{x: x, y: topY, z: z}, doubleTrunk: true}}
	for dx := -1; dx <= 2; dx++ {
		for dz := -1; dz <= 2; dz++ {
			if dx >= 0 && dx <= 1 && dz >= 0 && dz <= 1 || random.NextIntBound(3) > 0 {
				continue
			}
			branchHeight := int(random.NextIntBound(3)) + 2
			for layer := 0; layer < branchHeight; layer++ {
				position := blockPosition{x: origin.x + dx, y: topY - layer - 1, z: origin.z + dz}
				if f.placeLog(system, chunk, random, position, "y") {
					logs = append(logs, position)
				}
			}
			attachments = append(attachments, treeAttachment{position: blockPosition{x: origin.x + dx, y: topY, z: origin.z + dz}})
		}
	}
	return attachments, logs
}
