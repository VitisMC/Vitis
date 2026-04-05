package streaming

// Diff holds the result of comparing old and new chunk view regions.
type Diff struct {
	ToLoad   []ChunkPos
	ToUnload []ChunkPos
}

// ComputeDiff computes which chunks need to be loaded and unloaded when a player
// moves from oldCenter to newCenter with the given view distance.
// loaded is the set of chunks currently sent to the client.
func ComputeDiff(oldCenter, newCenter ChunkPos, viewDistance int32, loaded map[ChunkPos]struct{}) Diff {
	if oldCenter == newCenter {
		return Diff{}
	}

	var diff Diff

	for pos := range loaded {
		if !inRange(pos, newCenter, viewDistance) {
			diff.ToUnload = append(diff.ToUnload, pos)
		}
	}

	minX := newCenter.X - viewDistance
	maxX := newCenter.X + viewDistance
	minZ := newCenter.Z - viewDistance
	maxZ := newCenter.Z + viewDistance

	for x := minX; x <= maxX; x++ {
		for z := minZ; z <= maxZ; z++ {
			pos := ChunkPos{X: x, Z: z}
			if _, alreadyLoaded := loaded[pos]; alreadyLoaded {
				continue
			}
			if inRange(pos, oldCenter, viewDistance) {
				continue
			}
			diff.ToLoad = append(diff.ToLoad, pos)
		}
	}

	return diff
}

// ComputeFullLoad returns all chunk positions within view distance that are not yet loaded.
func ComputeFullLoad(center ChunkPos, viewDistance int32, loaded map[ChunkPos]struct{}) []ChunkPos {
	minX := center.X - viewDistance
	maxX := center.X + viewDistance
	minZ := center.Z - viewDistance
	maxZ := center.Z + viewDistance

	count := int((2*viewDistance + 1) * (2*viewDistance + 1))
	result := make([]ChunkPos, 0, count)

	for x := minX; x <= maxX; x++ {
		for z := minZ; z <= maxZ; z++ {
			pos := ChunkPos{X: x, Z: z}
			if _, ok := loaded[pos]; !ok {
				result = append(result, pos)
			}
		}
	}
	return result
}

func inRange(pos, center ChunkPos, viewDistance int32) bool {
	dx := pos.X - center.X
	if dx < 0 {
		dx = -dx
	}
	dz := pos.Z - center.Z
	if dz < 0 {
		dz = -dz
	}
	return dx <= viewDistance && dz <= viewDistance
}
