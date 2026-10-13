package worldgen

import (
	"encoding/json"
	"fmt"

	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type cherryTreeConfig struct {
	branchCount, horizontalLength, endOffset       intProvider
	startMin, startMax                             int
	wideBottomHole, cornerHole, hanging, extension float32
}

// decodeCherryTreeConfig validates branch providers and foliage probabilities.
func decodeCherryTreeConfig(count, horizontal, start, end json.RawMessage, wide, corner, hanging, extension float32) (*cherryTreeConfig, error) {
	config := &cherryTreeConfig{wideBottomHole: wide, cornerHole: corner, hanging: hanging, extension: extension}
	var err error
	if config.branchCount, err = decodeIntProviderRange(count, 1, 3); err != nil {
		return nil, fmt.Errorf("cherry branch count: %w", err)
	}
	if config.horizontalLength, err = decodeIntProviderRange(horizontal, 2, 16); err != nil {
		return nil, fmt.Errorf("cherry branch length: %w", err)
	}
	if config.endOffset, err = decodeIntProviderRange(end, -16, 16); err != nil {
		return nil, fmt.Errorf("cherry branch end: %w", err)
	}
	var bounds struct {
		Min int `json:"min_inclusive"`
		Max int `json:"max_inclusive"`
	}
	if err := json.Unmarshal(start, &bounds); err != nil {
		return nil, fmt.Errorf("cherry branch start: %w", err)
	}
	if bounds.Min < -16 || bounds.Max > 0 || bounds.Max <= bounds.Min {
		return nil, fmt.Errorf("invalid cherry branch start range")
	}
	config.startMin, config.startMax = bounds.Min, bounds.Max
	for _, chance := range []float32{wide, corner, hanging, extension} {
		if chance < 0 || chance > 1 {
			return nil, fmt.Errorf("invalid cherry foliage chance")
		}
	}
	return config, nil
}

// placeCherryTrunk grows the stem and its one or two opposing branches.
func (f *treeFeature) placeCherryTrunk(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition, height int) ([]treeAttachment, []blockPosition) {
	c := f.cherry
	startA := max(0, height-1+c.startMin+int(random.NextIntBound(int32(c.startMax-c.startMin+1))))
	startB := max(0, height-1+c.startMin+int(random.NextIntBound(int32(c.startMax-c.startMin))))
	if startB >= startA {
		startB++
	}
	count := c.branchCount.sample(random)
	stemHeight := startA + 1
	if count >= 2 {
		stemHeight = max(startA, startB) + 1
	}
	if count == 3 {
		stemHeight = height
	}
	var logs []blockPosition
	for layer := 0; layer < stemHeight; layer++ {
		p := blockPosition{x: origin.x, y: origin.y + layer, z: origin.z}
		if f.placeLog(system, chunk, random, p, "y") {
			logs = append(logs, p)
		}
	}
	var attachments []treeAttachment
	if count == 3 {
		attachments = append(attachments, treeAttachment{position: blockPosition{x: origin.x, y: origin.y + stemHeight, z: origin.z}})
	}
	direction := int(random.NextIntBound(4))
	attachment, branchLogs := f.placeCherryBranch(system, chunk, random, origin, height, startA, startA < stemHeight-1, direction)
	attachments, logs = append(attachments, attachment), append(logs, branchLogs...)
	if count >= 2 {
		attachment, branchLogs = f.placeCherryBranch(system, chunk, random, origin, height, startB, startB < stemHeight-1, (direction+2)%4)
		attachments, logs = append(attachments, attachment), append(logs, branchLogs...)
	}
	return attachments, logs
}

// placeCherryBranch follows the sloped path from the stem to a foliage attachment.
func (f *treeFeature) placeCherryBranch(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition, height, start int, bend bool, direction int) (treeAttachment, []blockPosition) {
	c := f.cherry
	endY := origin.y + height - 1 + c.endOffset.sample(random)
	if endY < origin.y+start {
		bend = true
	}
	length := c.horizontalLength.sample(random)
	initial := 1
	if bend {
		length++
		initial = 2
	}
	dx, dz := treeHorizontalDirections[direction][0], treeHorizontalDirections[direction][1]
	target := blockPosition{x: origin.x + dx*length, y: endY, z: origin.z + dz*length}
	current := blockPosition{x: origin.x, y: origin.y + start, z: origin.z}
	axis := "x"
	if dz != 0 {
		axis = "z"
	}
	var logs []blockPosition
	for i := 0; i < initial; i++ {
		current.x += dx
		current.z += dz
		if f.placeLog(system, chunk, random, current, axis) {
			logs = append(logs, current)
		}
	}
	for current != target {
		distance := abs(target.x-current.x) + abs(target.y-current.y) + abs(target.z-current.z)
		if distance == 0 {
			break
		}
		vertical := float32(abs(target.y-current.y)) / float32(distance)
		logAxis := axis
		if random.NextFloat() < vertical {
			if target.y > current.y {
				current.y++
			} else {
				current.y--
			}
			logAxis = "y"
		} else {
			current.x += dx
			current.z += dz
		}
		if f.placeLog(system, chunk, random, current, logAxis) {
			logs = append(logs, current)
		}
	}
	return treeAttachment{position: blockPosition{x: target.x, y: target.y + 1, z: target.z}}, logs
}

// placeCherryFoliage places layered leaves and hanging perimeter leaves.
func (f *treeFeature) placeCherryFoliage(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, attachment treeAttachment, height, radius int) []blockPosition {
	center := attachment.position
	center.y += f.offset.sample(random)
	r := radius + attachment.radiusOffset - 1
	var leaves []blockPosition
	row := func(y, size int) {
		if size < 0 {
			return
		}
		for dx := -size; dx <= size; dx++ {
			for dz := -size; dz <= size; dz++ {
				ax, az := abs(dx), abs(dz)
				if y == -1 && (ax == size || az == size) && random.NextFloat() < f.cherry.wideBottomHole {
					continue
				}
				corner := ax == size && az == size
				if size > 2 {
					if corner || ax+az > size*2-2 && random.NextFloat() < f.cherry.cornerHole {
						continue
					}
				} else if corner && random.NextFloat() < f.cherry.cornerHole {
					continue
				}
				p := blockPosition{x: center.x + dx, y: center.y + y, z: center.z + dz}
				if f.validPosition(chunk, p) {
					setTreeBlock(chunk, p, f.foliage.state(system, chunk, random, p))
					leaves = append(leaves, p)
				}
			}
		}
	}
	row(height-3, r-2)
	row(height-4, r-1)
	for y := height - 5; y >= 0; y-- {
		row(y, r)
	}
	for _, layer := range [][2]int{{-1, r}, {-2, r - 1}} {
		y, size := layer[0], layer[1]
		row(y, size)
		for dx := -size; dx <= size; dx++ {
			for dz := -size; dz <= size; dz++ {
				if abs(dx) != size && abs(dz) != size {
					continue
				}
				upper := blockPosition{x: center.x + dx, y: center.y + y, z: center.z + dz}
				if _, ok := f.leaves[getTreeBlock(chunk, upper)]; !ok {
					continue
				}
				if random.NextFloat() > f.cherry.hanging {
					continue
				}
				p := blockPosition{x: upper.x, y: upper.y - 1, z: upper.z}
				if f.validPosition(chunk, p) {
					setTreeBlock(chunk, p, f.foliage.state(system, chunk, random, p))
					leaves = append(leaves, p)
					if random.NextFloat() <= f.cherry.extension {
						p.y--
						if f.validPosition(chunk, p) {
							setTreeBlock(chunk, p, f.foliage.state(system, chunk, random, p))
							leaves = append(leaves, p)
						}
					}
				}
			}
		}
	}
	return leaves
}
