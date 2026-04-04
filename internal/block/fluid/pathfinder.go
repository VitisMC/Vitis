package fluid

type Direction int8

const (
	DirNorth Direction = iota
	DirSouth
	DirWest
	DirEast
)

func (d Direction) Offset() (dx, dy, dz int) {
	switch d {
	case DirNorth:
		return 0, 0, -1
	case DirSouth:
		return 0, 0, 1
	case DirWest:
		return -1, 0, 0
	case DirEast:
		return 1, 0, 0
	}
	return 0, 0, 0
}

func (d Direction) Opposite() Direction {
	switch d {
	case DirNorth:
		return DirSouth
	case DirSouth:
		return DirNorth
	case DirWest:
		return DirEast
	case DirEast:
		return DirWest
	}
	return d
}

var AllHorizontalDirs = []Direction{DirNorth, DirSouth, DirWest, DirEast}

type pathNode struct {
	x, z       int
	distance   int
	excludeDir Direction
}

type BlockGetter func(x, y, z int) int32

func FindFlowDirections(x, y, z int, fluidType FluidType, maxDistance int, getBlock BlockGetter) []Direction {
	belowState := getBlock(x, y-1, z)
	if CanBeReplacedByFluid(belowState, fluidType) {
		return nil
	}

	var result []Direction
	minDist := 1000

	for _, dir := range AllHorizontalDirs {
		dx, _, dz := dir.Offset()
		nx, nz := x+dx, z+dz

		sideState := getBlock(nx, y, nz)
		if !CanFluidFlowInto(sideState, fluidType) {
			continue
		}

		if GetFluidType(sideState) == fluidType && IsSource(sideState) {
			continue
		}

		belowSide := getBlock(nx, y-1, nz)
		if CanBeReplacedByFluid(belowSide, fluidType) {
			if 0 < minDist {
				result = result[:0]
				minDist = 0
			}
			result = append(result, dir)
			continue
		}

		dist := findHoleDistance(nx, y, nz, dir.Opposite(), fluidType, maxDistance, getBlock)
		if dist < minDist {
			result = result[:0]
			minDist = dist
		}
		if dist <= minDist && dist < 1000 {
			result = append(result, dir)
		}
	}

	return result
}

func findHoleDistance(startX, y, startZ int, excludeDir Direction, fluidType FluidType, maxDistance int, getBlock BlockGetter) int {
	const maxQueueSize = 64
	var queue [maxQueueSize]pathNode
	queueStart, queueEnd := 0, 0

	queue[queueEnd] = pathNode{x: startX, z: startZ, distance: 1, excludeDir: excludeDir}
	queueEnd++

	visited := make(map[int64]struct{}, 64)

	for queueStart < queueEnd {
		node := queue[queueStart]
		queueStart++

		if node.distance > maxDistance {
			continue
		}

		key := int64(node.x)<<32 | int64(uint32(node.z))
		if _, seen := visited[key]; seen {
			continue
		}
		visited[key] = struct{}{}

		belowState := getBlock(node.x, y-1, node.z)
		if CanBeReplacedByFluid(belowState, fluidType) {
			return node.distance
		}

		for _, dir := range AllHorizontalDirs {
			if dir == node.excludeDir {
				continue
			}

			dx, _, dz := dir.Offset()
			nx, nz := node.x+dx, node.z+dz

			nextState := getBlock(nx, y, nz)
			if !CanFluidFlowInto(nextState, fluidType) {
				continue
			}

			if GetFluidType(nextState) == fluidType && IsSource(nextState) {
				continue
			}

			if queueEnd < maxQueueSize {
				queue[queueEnd] = pathNode{
					x:          nx,
					z:          nz,
					distance:   node.distance + 1,
					excludeDir: dir.Opposite(),
				}
				queueEnd++
			}
		}
	}

	return 1000
}
