package worldgen

import (
	"math"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// placeGiantTrunk grows the two by two trunk and optional jungle branches.
func (f *treeFeature) placeGiantTrunk(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition, height int) ([]treeAttachment, []blockPosition) {
	for _, offset := range [][2]int{{1, 0}, {0, 1}, {1, 1}} {
		p := blockPosition{x: origin.x + offset[0], y: origin.y - 1, z: origin.z + offset[1]}
		state := getTreeBlock(chunk, p)
		_, isDirt := f.dirtStates[state]
		if f.forceDirt || !isDirt || block.NameFromState(state) == "minecraft:grass_block" || block.NameFromState(state) == "minecraft:mycelium" {
			setTreeBlock(chunk, p, f.dirt.state(system, chunk, random, p))
		}
	}
	logs := make([]blockPosition, 0, height*4)
	for y := 0; y < height; y++ {
		p := blockPosition{x: origin.x, y: origin.y + y, z: origin.z}
		if f.placeLog(system, chunk, random, p, "y") {
			logs = append(logs, p)
		}
		if y == height-1 {
			continue
		}
		for _, offset := range [][2]int{{1, 0}, {1, 1}, {0, 1}} {
			p := blockPosition{x: origin.x + offset[0], y: origin.y + y, z: origin.z + offset[1]}
			if f.placeLog(system, chunk, random, p, "y") {
				logs = append(logs, p)
			}
		}
	}
	attachments := []treeAttachment{{position: blockPosition{x: origin.x, y: origin.y + height, z: origin.z}, doubleTrunk: true}}
	if f.trunkKind != "minecraft:mega_jungle_trunk_placer" {
		return attachments, logs
	}
	for y := height - 2 - int(random.NextIntBound(4)); y > height/2; y -= 2 + int(random.NextIntBound(4)) {
		angle := float64(random.NextFloat()) * 2 * math.Pi
		branchX, branchZ := 0, 0
		for step := 0; step < 5; step++ {
			branchX = int(float32(1.5) + float32(math.Cos(angle))*float32(step))
			branchZ = int(float32(1.5) + float32(math.Sin(angle))*float32(step))
			p := blockPosition{x: origin.x + branchX, y: origin.y + y - 3 + step/2, z: origin.z + branchZ}
			if f.placeLog(system, chunk, random, p, "y") {
				logs = append(logs, p)
			}
		}
		attachments = append(attachments, treeAttachment{position: blockPosition{x: origin.x + branchX, y: origin.y + y, z: origin.z + branchZ}, radiusOffset: -2})
	}
	return attachments, logs
}

// placeGiantFoliage builds the rounded crowns used by giant jungle and taiga trees.
func (f *treeFeature) placeGiantFoliage(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, attachment treeAttachment, height, radius int) []blockPosition {
	offset := f.offset.sample(random)
	var leaves []blockPosition
	placeRow := func(y, rowRadius int) {
		end := rowRadius
		if attachment.doubleTrunk {
			end++
		}
		for dx := -rowRadius; dx <= end; dx++ {
			for dz := -rowRadius; dz <= end; dz++ {
				ax, az := abs(dx), abs(dz)
				if attachment.doubleTrunk {
					ax, az = min(ax, abs(dx-1)), min(az, abs(dz-1))
				}
				if ax+az >= 7 || ax*ax+az*az > rowRadius*rowRadius {
					continue
				}
				p := blockPosition{x: attachment.position.x + dx, y: attachment.position.y + y, z: attachment.position.z + dz}
				if !f.validPosition(chunk, p) {
					continue
				}
				setTreeBlock(chunk, p, f.foliage.state(system, chunk, random, p))
				leaves = append(leaves, p)
			}
		}
	}
	if f.foliageKind == "minecraft:jungle_foliage_placer" {
		layers := height
		if !attachment.doubleTrunk {
			layers = 1 + int(random.NextIntBound(2))
		}
		for y := offset; y >= offset-layers; y-- {
			placeRow(y, radius+attachment.radiusOffset+1-y)
		}
		return leaves
	}
	previousRadius := 0
	for y := offset - height; y <= offset; y++ {
		distance := offset - y
		rowRadius := radius + attachment.radiusOffset + int(math.Floor(float64(float32(distance)/float32(max(height, 1))*3.5)))
		if distance > 0 && rowRadius == previousRadius && (attachment.position.y+y)&1 == 0 {
			placeRow(y, rowRadius+1)
		} else {
			placeRow(y, rowRadius)
		}
		previousRadius = rowRadius
	}
	return leaves
}
