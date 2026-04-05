package lighting

import (
	genblock "github.com/vitismc/vitis/internal/data/generated/block"
)

const (
	maxLight = 15
)

// BlockAccess provides read access to block states in a chunk column.
type BlockAccess interface {
	// GetBlock returns the block state ID at absolute coordinates within this chunk column.
	// Returns 0 (air) for out-of-range coordinates.
	GetBlock(x, y, z int) int32
	// MinY returns the minimum Y coordinate of the chunk column.
	MinY() int
	// MaxY returns the maximum Y coordinate of the chunk column.
	MaxY() int
	// NumSections returns the number of sections in the chunk column.
	NumSections() int
}

// NeighborAccess provides read access to block states in neighboring chunk columns.
// Returns 0 (air) if the neighbor is not loaded.
type NeighborAccess interface {
	// GetNeighborBlock returns the block state at absolute world coordinates.
	// The coordinates may be in a neighboring chunk.
	GetNeighborBlock(x, y, z int) int32
}

type lightNode struct {
	x, y, z int
	level   uint8
}

// Engine computes block light and sky light for a chunk column.
type Engine struct {
	minY        int
	maxY        int
	numSections int

	BlockLightSections []*NibbleArray
	SkyLightSections   []*NibbleArray
}

// NewEngine creates a lighting engine for a chunk column with the given parameters.
func NewEngine(numSections, minY, maxY int) *Engine {
	lightSections := numSections + 2

	blockLight := make([]*NibbleArray, lightSections)
	skyLight := make([]*NibbleArray, lightSections)
	for i := range blockLight {
		blockLight[i] = &NibbleArray{}
		skyLight[i] = &NibbleArray{}
	}

	return &Engine{
		minY:               minY,
		maxY:               maxY,
		numSections:        numSections,
		BlockLightSections: blockLight,
		SkyLightSections:   skyLight,
	}
}

// ComputeBlockLight calculates block light propagation from all light-emitting blocks.
func (e *Engine) ComputeBlockLight(access BlockAccess) {
	queue := make([]lightNode, 0, 256)

	for sy := 0; sy < access.NumSections(); sy++ {
		baseY := access.MinY() + sy*16
		for ly := 0; ly < 16; ly++ {
			for lz := 0; lz < 16; lz++ {
				for lx := 0; lx < 16; lx++ {
					stateID := access.GetBlock(lx, baseY+ly, lz)
					emission := blockEmission(stateID)
					if emission > 0 {
						e.setBlockLight(lx, baseY+ly, lz, emission)
						queue = append(queue, lightNode{lx, baseY + ly, lz, emission})
					}
				}
			}
		}
	}

	e.propagate(queue, access, false)
}

// ComputeSkyLight calculates sky light propagation from the top of the chunk column.
func (e *Engine) ComputeSkyLight(access BlockAccess) {
	queue := make([]lightNode, 0, 4096)

	heightMap := e.computeHeightMap(access)

	for lz := 0; lz < 16; lz++ {
		for lx := 0; lx < 16; lx++ {
			topBlock := heightMap[lz*16+lx]

			for y := access.MaxY(); y > topBlock; y-- {
				e.setSkyLight(lx, y, lz, maxLight)
			}

			if topBlock >= access.MinY() && topBlock <= access.MaxY() {
				e.setSkyLight(lx, topBlock, lz, maxLight)
				queue = append(queue, lightNode{lx, topBlock, lz, maxLight})
			}
		}
	}

	e.propagate(queue, access, true)
}

// ComputeAll calculates both block light and sky light.
func (e *Engine) ComputeAll(access BlockAccess) {
	e.ComputeBlockLight(access)
	e.ComputeSkyLight(access)
}

// GetBlockLightSection returns the block light nibble array for the given section index
// (0 = below chunk, 1..numSections = normal sections, numSections+1 = above chunk).
func (e *Engine) GetBlockLightSection(lightSectionIdx int) *NibbleArray {
	if lightSectionIdx < 0 || lightSectionIdx >= len(e.BlockLightSections) {
		return nil
	}
	return e.BlockLightSections[lightSectionIdx]
}

// GetSkyLightSection returns the sky light nibble array for the given section index.
func (e *Engine) GetSkyLightSection(lightSectionIdx int) *NibbleArray {
	if lightSectionIdx < 0 || lightSectionIdx >= len(e.SkyLightSections) {
		return nil
	}
	return e.SkyLightSections[lightSectionIdx]
}

func (e *Engine) propagate(queue []lightNode, access BlockAccess, isSky bool) {
	dirs := [6][3]int{
		{1, 0, 0}, {-1, 0, 0},
		{0, 1, 0}, {0, -1, 0},
		{0, 0, 1}, {0, 0, -1},
	}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		for _, d := range dirs {
			nx, ny, nz := node.x+d[0], node.y+d[1], node.z+d[2]

			if ny < e.minY-16 || ny > e.maxY+16 {
				continue
			}

			if nx < 0 || nx > 15 || nz < 0 || nz > 15 {
				continue
			}

			stateID := access.GetBlock(nx, ny, nz)
			opacity := blockOpacity(stateID)

			newLevel := int(node.level) - maxI(1, int(opacity))
			if newLevel <= 0 {
				continue
			}

			var current uint8
			if isSky {
				current = e.getSkyLight(nx, ny, nz)
			} else {
				current = e.getBlockLight(nx, ny, nz)
			}

			if uint8(newLevel) > current {
				if isSky {
					e.setSkyLight(nx, ny, nz, uint8(newLevel))
				} else {
					e.setBlockLight(nx, ny, nz, uint8(newLevel))
				}
				queue = append(queue, lightNode{nx, ny, nz, uint8(newLevel)})
			}
		}
	}
}

func (e *Engine) computeHeightMap(access BlockAccess) [256]int {
	var hm [256]int
	for i := range hm {
		hm[i] = access.MinY() - 1
	}

	for lz := 0; lz < 16; lz++ {
		for lx := 0; lx < 16; lx++ {
			for y := access.MaxY(); y >= access.MinY(); y-- {
				stateID := access.GetBlock(lx, y, lz)
				if blockOpacity(stateID) > 0 {
					hm[lz*16+lx] = y
					break
				}
			}
		}
	}
	return hm
}

func (e *Engine) lightSectionIndex(y int) int {
	return ((y - e.minY) >> 4) + 1
}

func (e *Engine) getBlockLight(x, y, z int) uint8 {
	idx := e.lightSectionIndex(y)
	if idx < 0 || idx >= len(e.BlockLightSections) {
		return 0
	}
	return e.BlockLightSections[idx].GetXYZ(x, y, z)
}

func (e *Engine) setBlockLight(x, y, z int, level uint8) {
	idx := e.lightSectionIndex(y)
	if idx < 0 || idx >= len(e.BlockLightSections) {
		return
	}
	e.BlockLightSections[idx].SetXYZ(x, y, z, level)
}

func (e *Engine) getSkyLight(x, y, z int) uint8 {
	idx := e.lightSectionIndex(y)
	if idx < 0 || idx >= len(e.SkyLightSections) {
		return 0
	}
	return e.SkyLightSections[idx].GetXYZ(x, y, z)
}

func (e *Engine) setSkyLight(x, y, z int, level uint8) {
	idx := e.lightSectionIndex(y)
	if idx < 0 || idx >= len(e.SkyLightSections) {
		return
	}
	e.SkyLightSections[idx].SetXYZ(x, y, z, level)
}

func blockEmission(stateID int32) uint8 {
	if stateID <= 0 || stateID >= genblock.TotalStates {
		return 0
	}
	blockID := genblock.StateToBlock[stateID]
	if blockID < 0 || blockID >= int32(len(genblock.Blocks)) {
		return 0
	}
	return uint8(genblock.Blocks[blockID].EmitLight)
}

func blockOpacity(stateID int32) uint8 {
	if stateID <= 0 || stateID >= genblock.TotalStates {
		return 0
	}
	blockID := genblock.StateToBlock[stateID]
	if blockID < 0 || blockID >= int32(len(genblock.Blocks)) {
		return 0
	}
	return uint8(genblock.Blocks[blockID].FilterLight)
}

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}
