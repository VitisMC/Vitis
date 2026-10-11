package lighting

import (
	genblock "github.com/vitismc/vitis/internal/data/generated/block"
	"sync"
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

// BlockPaletteAccess exposes section palettes so the light scan can skip sections without emitters.
type BlockPaletteAccess interface {
	BlockPalette(section int) []int32
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

// lightQueuePool reuses propagation scratch without retaining any chunk or engine.
var lightQueuePool = sync.Pool{New: func() any { return make([]lightNode, 0, 4096) }}

// releaseLightQueue bounds retained scratch after unusually large propagation jobs.
func releaseLightQueue(queue []lightNode) {
	if cap(queue) <= 65536 {
		lightQueuePool.Put(queue[:0])
	}
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

// NewSparseEngine allocates light sections only when generation writes nonzero light.
func NewSparseEngine(numSections, minY, maxY int) *Engine {
	lightSections := numSections + 2
	return &Engine{
		minY:               minY,
		maxY:               maxY,
		numSections:        numSections,
		BlockLightSections: make([]*NibbleArray, lightSections),
		SkyLightSections:   make([]*NibbleArray, lightSections),
	}
}

// ComputeBlockLight calculates block light propagation from all light-emitting blocks.
func (e *Engine) ComputeBlockLight(access BlockAccess) {
	e.clearBlockLight()
	queue := lightQueuePool.Get().([]lightNode)
	defer func() { releaseLightQueue(queue) }()
	palettes, hasPalettes := access.(BlockPaletteAccess)

	for sy := 0; sy < access.NumSections(); sy++ {
		if hasPalettes {
			palette := palettes.BlockPalette(sy)
			if palette != nil {
				hasEmitter := false
				for _, state := range palette {
					if blockEmission(state) > 0 {
						hasEmitter = true
						break
					}
				}
				if !hasEmitter {
					continue
				}
			}
		}
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

	queue = e.propagate(queue, access, false)
}

// ComputeSkyLight calculates sky light propagation from the top of the chunk column.
func (e *Engine) ComputeSkyLight(access BlockAccess) {
	e.clearSkyLight()
	queue := lightQueuePool.Get().([]lightNode)
	defer func() { releaseLightQueue(queue) }()

	heightMap := e.computeHeightMap(access)
	highestOpaque := access.MinY() - 1
	for _, height := range heightMap {
		if height > highestOpaque {
			highestOpaque = height
		}
	}
	firstFullSection := 0
	if highestOpaque >= access.MinY() {
		firstFullSection = (highestOpaque-access.MinY())/16 + 1
	}
	if firstFullSection > e.numSections {
		firstFullSection = e.numSections
	}
	for section := firstFullSection; section < e.numSections; section++ {
		if e.SkyLightSections[section+1] == nil {
			e.SkyLightSections[section+1] = &NibbleArray{}
		}
		e.SkyLightSections[section+1].Fill(maxLight)
	}
	individualTop := min(access.MaxY(), access.MinY()+firstFullSection*16-1)

	for lz := 0; lz < 16; lz++ {
		for lx := 0; lx < 16; lx++ {
			topBlock := heightMap[lz*16+lx]

			for y := individualTop; y > topBlock; y-- {
				e.setSkyLight(lx, y, lz, maxLight)
			}

			neighborTop := topBlock
			for _, offset := range [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
				nx, nz := lx+offset[0], lz+offset[1]
				if nx >= 0 && nx < 16 && nz >= 0 && nz < 16 {
					neighborTop = max(neighborTop, heightMap[nz*16+nx])
				}
			}
			for y := max(access.MinY(), topBlock+1); y <= min(access.MaxY(), neighborTop); y++ {
				queue = append(queue, lightNode{lx, y, lz, maxLight})
			}

			if topBlock >= access.MinY() && topBlock <= access.MaxY() {
				level := maxLight - maxI(1, int(blockOpacity(access.GetBlock(lx, topBlock, lz))))
				if level > 0 {
					e.setSkyLight(lx, topBlock, lz, uint8(level))
					queue = append(queue, lightNode{lx, topBlock, lz, uint8(level)})
				}
			}
		}
	}

	queue = e.propagate(queue, access, true)
}

func (e *Engine) clearBlockLight() {
	for _, section := range e.BlockLightSections {
		if section != nil {
			section.Clear()
		}
	}
}

func (e *Engine) clearSkyLight() {
	for _, section := range e.SkyLightSections {
		if section != nil {
			section.Clear()
		}
	}
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

// propagate retains FIFO order and returns the complete owned scratch allocation.
func (e *Engine) propagate(queue []lightNode, access BlockAccess, isSky bool) []lightNode {
	dirs := [6][3]int{
		{1, 0, 0}, {-1, 0, 0},
		{0, 1, 0}, {0, -1, 0},
		{0, 0, 1}, {0, 0, -1},
	}

	for head := 0; head < len(queue); head++ {
		node := queue[head]

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
	return queue
}

func (e *Engine) computeHeightMap(access BlockAccess) [256]int {
	var hm [256]int
	for i := range hm {
		hm[i] = access.MinY() - 1
	}
	var opaqueSections []bool
	if palettes, ok := access.(BlockPaletteAccess); ok {
		opaqueSections = make([]bool, access.NumSections())
		for section := range opaqueSections {
			palette := palettes.BlockPalette(section)
			if palette == nil {
				opaqueSections[section] = true
				continue
			}
			for _, state := range palette {
				if blockOpacity(state) > 0 {
					opaqueSections[section] = true
					break
				}
			}
		}
	}

	for lz := 0; lz < 16; lz++ {
		for lx := 0; lx < 16; lx++ {
			if opaqueSections == nil {
				for y := access.MaxY(); y >= access.MinY(); y-- {
					if blockOpacity(access.GetBlock(lx, y, lz)) > 0 {
						hm[lz*16+lx] = y
						break
					}
				}
				continue
			}
			for section := len(opaqueSections) - 1; section >= 0; section-- {
				if !opaqueSections[section] {
					continue
				}
				bottom := access.MinY() + section*16
				top := min(access.MaxY(), bottom+15)
				for y := top; y >= bottom; y-- {
					if blockOpacity(access.GetBlock(lx, y, lz)) > 0 {
						hm[lz*16+lx] = y
						break
					}
				}
				if hm[lz*16+lx] >= bottom {
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
	if idx < 0 || idx >= len(e.BlockLightSections) || e.BlockLightSections[idx] == nil {
		return 0
	}
	return e.BlockLightSections[idx].GetXYZ(x, y, z)
}

func (e *Engine) setBlockLight(x, y, z int, level uint8) {
	idx := e.lightSectionIndex(y)
	if idx < 0 || idx >= len(e.BlockLightSections) {
		return
	}
	if e.BlockLightSections[idx] == nil {
		e.BlockLightSections[idx] = &NibbleArray{}
	}
	e.BlockLightSections[idx].SetXYZ(x, y, z, level)
}

func (e *Engine) getSkyLight(x, y, z int) uint8 {
	idx := e.lightSectionIndex(y)
	if idx < 0 || idx >= len(e.SkyLightSections) || e.SkyLightSections[idx] == nil {
		return 0
	}
	return e.SkyLightSections[idx].GetXYZ(x, y, z)
}

func (e *Engine) setSkyLight(x, y, z int, level uint8) {
	idx := e.lightSectionIndex(y)
	if idx < 0 || idx >= len(e.SkyLightSections) {
		return
	}
	if e.SkyLightSections[idx] == nil {
		e.SkyLightSections[idx] = &NibbleArray{}
	}
	e.SkyLightSections[idx].SetXYZ(x, y, z, level)
}

// blockEmission resolves state-dependent light emission, including lit and attached states.
func blockEmission(stateID int32) uint8 {
	if stateID <= 0 || stateID >= genblock.TotalStates {
		return 0
	}
	return uint8(genblock.GenerationTraits[stateID] >> genblock.GenerationEmissionShift & 15)
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
