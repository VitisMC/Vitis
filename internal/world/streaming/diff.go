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

	minX := newCenter.X - viewDistance - 1
	maxX := newCenter.X + viewDistance + 1
	minZ := newCenter.Z - viewDistance - 1
	maxZ := newCenter.Z + viewDistance + 1

	for x := minX; x <= maxX; x++ {
		for z := minZ; z <= maxZ; z++ {
			pos := ChunkPos{X: x, Z: z}
			if !inRange(pos, newCenter, viewDistance) {
				continue
			}
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
	minX := center.X - viewDistance - 1
	maxX := center.X + viewDistance + 1
	minZ := center.Z - viewDistance - 1
	maxZ := center.Z + viewDistance + 1

	count := int((2*viewDistance + 1) * (2*viewDistance + 1))
	result := make([]ChunkPos, 0, count)

	for x := minX; x <= maxX; x++ {
		for z := minZ; z <= maxZ; z++ {
			pos := ChunkPos{X: x, Z: z}
			if !inRange(pos, center, viewDistance) {
				continue
			}
			if _, ok := loaded[pos]; !ok {
				result = append(result, pos)
			}
		}
	}
	return result
}

func inRange(pos, center ChunkPos, viewDistance int32) bool {
	if viewDistance <= 1 {
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
	dx := pos.X - center.X
	if dx < 0 {
		dx = -dx
	}
	dz := pos.Z - center.Z
	if dz < 0 {
		dz = -dz
	}
	dx = max32(0, dx-2)
	dz = max32(0, dz-2)
	return int64(dx)*int64(dx)+int64(dz)*int64(dz) < int64(viewDistance)*int64(viewDistance)
}

func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
