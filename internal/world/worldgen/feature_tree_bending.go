package worldgen

import (
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type bendingTreeConfig struct {
	bendLength         intProvider
	minHeightForLeaves int
}

// placeBendingTrunk grows a short curving stem with foliage attachments along it.
func (f *treeFeature) placeBendingTrunk(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition, height int) ([]treeAttachment, []blockPosition) {
	c := f.bending
	direction := int(random.NextIntBound(4))
	dx, dz := treeHorizontalDirections[direction][0], treeHorizontalDirections[direction][1]
	current := origin
	var attachments []treeAttachment
	var logs []blockPosition
	for layer := 0; layer < height; layer++ {
		if layer+1 >= height-1+int(random.NextIntBound(2)) {
			current.x += dx
			current.z += dz
		}
		if f.placeLog(system, chunk, random, current, "y") {
			logs = append(logs, current)
		}
		if layer >= c.minHeightForLeaves {
			attachments = append(attachments, treeAttachment{position: current})
		}
		current.y++
	}
	length := c.bendLength.sample(random)
	for step := 0; step <= length; step++ {
		if f.placeLog(system, chunk, random, current, "y") {
			logs = append(logs, current)
		}
		attachments = append(attachments, treeAttachment{position: current})
		current.x += dx
		current.z += dz
	}
	return attachments, logs
}

// placeRandomSpreadFoliage scatters leaves around one stem attachment.
func (f *treeFeature) placeRandomSpreadFoliage(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, attachment treeAttachment, height, radius int) []blockPosition {
	_ = f.offset.sample(random)
	if height <= 0 || radius <= 0 {
		return nil
	}
	var leaves []blockPosition
	for attempt := 0; attempt < f.leafAttempts; attempt++ {
		p := blockPosition{
			x: attachment.position.x + int(random.NextIntBound(int32(radius))) - int(random.NextIntBound(int32(radius))),
			y: attachment.position.y + int(random.NextIntBound(int32(height))) - int(random.NextIntBound(int32(height))),
			z: attachment.position.z + int(random.NextIntBound(int32(radius))) - int(random.NextIntBound(int32(radius))),
		}
		if f.validPosition(chunk, p) {
			setTreeBlock(chunk, p, f.foliage.state(system, chunk, random, p))
			leaves = append(leaves, p)
		}
	}
	return leaves
}
