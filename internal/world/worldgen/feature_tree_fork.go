package worldgen

import (
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

var treeHorizontalDirections = [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}

// placeForkingTrunk grows the bent trunk and optional secondary branch.
func (f *treeFeature) placeForkingTrunk(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition, height int) ([]treeAttachment, []blockPosition) {
	firstDirection := int(random.NextIntBound(4))
	bendStart := height - int(random.NextIntBound(4)) - 1
	bendSteps := 3 - int(random.NextIntBound(3))
	x, z := origin.x, origin.z
	topY := -1
	var logs []blockPosition
	for layer := 0; layer < height; layer++ {
		if layer >= bendStart && bendSteps > 0 {
			x += treeHorizontalDirections[firstDirection][0]
			z += treeHorizontalDirections[firstDirection][1]
			bendSteps--
		}
		position := blockPosition{x: x, y: origin.y + layer, z: z}
		if f.placeLog(system, chunk, random, position, "y") {
			logs = append(logs, position)
			topY = position.y + 1
		}
	}
	var attachments []treeAttachment
	if topY >= 0 {
		attachments = append(attachments, treeAttachment{position: blockPosition{x: x, y: topY, z: z}, radiusOffset: 1})
	}
	secondDirection := int(random.NextIntBound(4))
	if secondDirection == firstDirection {
		return attachments, logs
	}
	branchStart := bendStart - int(random.NextIntBound(2)) - 1
	branchSteps := 1 + int(random.NextIntBound(3))
	x, z = origin.x, origin.z
	topY = -1
	for layer := branchStart; layer < height && branchSteps > 0; layer, branchSteps = layer+1, branchSteps-1 {
		if layer < 1 {
			continue
		}
		x += treeHorizontalDirections[secondDirection][0]
		z += treeHorizontalDirections[secondDirection][1]
		position := blockPosition{x: x, y: origin.y + layer, z: z}
		if f.placeLog(system, chunk, random, position, "y") {
			logs = append(logs, position)
			topY = position.y + 1
		}
	}
	if topY >= 0 {
		attachments = append(attachments, treeAttachment{position: blockPosition{x: x, y: topY, z: z}})
	}
	return attachments, logs
}
