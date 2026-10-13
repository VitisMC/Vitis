package section

import (
	"fmt"
	"strings"

	"github.com/vitismc/vitis/internal/block"
)

const importedFeaturePriority uint32 = 1 << 31

// FeatureBorderWrite pairs a block state with its deterministic decoration order.
type FeatureBorderWrite struct {
	State    int32
	Priority uint32
}

// indexedBorderWrite retains the local voxel index without hash-table overhead.
type indexedBorderWrite struct {
	index uint32
	write FeatureBorderWrite
}

// FeatureBorderBuffer stores neighboring writes by chunk and compact local voxel index.
// Published buffers are immutable after decoration finishes.
type FeatureBorderBuffer struct {
	x, z       int32
	minY, maxY int
	radius     int
	chunks     []map[uint32]FeatureBorderWrite
	compact    [][]indexedBorderWrite
}

// Compact releases mutable hash tables after decoration, before publishing the buffer.
// The caller must exclusively own the buffer while compacting it.
func (b *FeatureBorderBuffer) Compact() {
	if b == nil {
		return
	}
	if b.compact == nil {
		b.compact = make([][]indexedBorderWrite, len(b.chunks))
	}
	for bucket, writes := range b.chunks {
		if len(writes) == 0 {
			continue
		}
		entries := make([]indexedBorderWrite, 0, len(writes))
		for index, write := range writes {
			entries = append(entries, indexedBorderWrite{index: index, write: write})
		}
		b.compact[bucket] = entries
		b.chunks[bucket] = nil
	}
}

// index resolves a world position into its neighboring chunk and local voxel index.
func (b *FeatureBorderBuffer) index(x, y, z int) (int, uint32, bool) {
	dx, dz := (x>>4)-int(b.x), (z>>4)-int(b.z)
	if dx < -b.radius || dx > b.radius || dz < -b.radius || dz > b.radius || y < b.minY || y > b.maxY {
		return 0, 0, false
	}
	width := b.radius*2 + 1
	return (dx+b.radius)*width + dz + b.radius, uint32((y-b.minY)*256 + (z&15)*16 + (x & 15)), true
}

// set records a bounded neighboring write without storing redundant world coordinates.
func (b *FeatureBorderBuffer) set(x, y, z int, write FeatureBorderWrite) {
	bucket, index, ok := b.index(x, y, z)
	if !ok {
		return
	}
	if b.chunks[bucket] == nil {
		b.chunks[bucket] = make(map[uint32]FeatureBorderWrite, 128)
		if b.compact != nil {
			for _, entry := range b.compact[bucket] {
				b.chunks[bucket][entry.index] = entry.write
			}
			b.compact[bucket] = nil
		}
	}
	b.chunks[bucket][index] = write
}

// get returns an explicit write, including writes of air.
func (b *FeatureBorderBuffer) get(x, y, z int) (FeatureBorderWrite, bool) {
	bucket, index, ok := b.index(x, y, z)
	if !ok {
		return FeatureBorderWrite{}, false
	}
	write, found := b.chunks[bucket][index]
	if !found && b.compact != nil {
		for _, entry := range b.compact[bucket] {
			if entry.index == index {
				return entry.write, true
			}
		}
	}
	return write, found
}

// ForChunk visits only writes owned by the requested destination chunk.
func (b *FeatureBorderBuffer) ForChunk(x, z int32, visit func(BorderPosition, FeatureBorderWrite)) {
	if b == nil {
		return
	}
	dx, dz := int(x)-int(b.x), int(z)-int(b.z)
	if dx < -b.radius || dx > b.radius || dz < -b.radius || dz > b.radius {
		return
	}
	width := b.radius*2 + 1
	bucket := (dx+b.radius)*width + dz + b.radius
	if b.compact != nil {
		for _, entry := range b.compact[bucket] {
			index := entry.index
			visit(BorderPosition{X: int(x)*16 + int(index&15), Y: b.minY + int(index>>8), Z: int(z)*16 + int((index>>4)&15)}, entry.write)
		}
	}
	for index, write := range b.chunks[bucket] {
		visit(BorderPosition{X: int(x)*16 + int(index&15), Y: b.minY + int(index>>8), Z: int(z)*16 + int((index>>4)&15)}, write)
	}
}

// Range visits all writes without materializing world-coordinate maps.
func (b *FeatureBorderBuffer) Range(visit func(BorderPosition, FeatureBorderWrite)) {
	if b == nil {
		return
	}
	for dx := -b.radius; dx <= b.radius; dx++ {
		for dz := -b.radius; dz <= b.radius; dz++ {
			b.ForChunk(b.x+int32(dx), b.z+int32(dz), visit)
		}
	}
}

// featureRegionState owns transient dependency readers and deterministic decoration write ordering.
type featureRegionState struct {
	readBlock    func(x, y, z int) int32
	readHeight   func(x, z int, kind HeightmapKind) (int, bool)
	writes       []uint32
	priority     uint32
	borderWrites *FeatureBorderBuffer
	columnWrites *featureColumnWrites
	radius       int
}

// featureColumnWrites tracks modified neighboring columns without per-block map hashing.
type featureColumnWrites struct {
	heights [48 * 48]int16
	valid   [48 * 48 / 64]uint64
	outer   map[[2]int]int16
}

// UseFeatureRegion attaches dependency readers and worker-owned write stamps during generation only.
func (c *Chunk) UseFeatureRegion(readBlock func(int, int, int) int32, readHeight func(int, int, HeightmapKind) (int, bool), writes []uint32) error {
	return c.UseFeatureRegionRadius(readBlock, readHeight, writes, 1)
}

// UseFeatureRegionRadius attaches a bounded multi-chunk decoration view.
func (c *Chunk) UseFeatureRegionRadius(readBlock func(int, int, int) int32, readHeight func(int, int, HeightmapKind) (int, bool), writes []uint32, radius int) error {
	if c == nil || readBlock == nil || readHeight == nil || len(writes) != len(c.Sections)*BlocksPerSection || radius < 1 || radius > 32 {
		return fmt.Errorf("invalid feature region storage or readers")
	}
	clear(writes)
	c.EnableBorderBlocks()
	width := radius*2 + 1
	c.featureRegion = &featureRegionState{readBlock: readBlock, readHeight: readHeight, writes: writes, priority: 1, radius: 1,
		borderWrites: &FeatureBorderBuffer{x: c.X, z: c.Z, minY: c.minY, maxY: c.maxY, radius: radius, chunks: make([]map[uint32]FeatureBorderWrite, width*width)}, columnWrites: &featureColumnWrites{}}
	return nil
}

// FeatureRegionEnabled reports whether reads can resolve immutable neighboring terrain.
func (c *Chunk) FeatureRegionEnabled() bool { return c != nil && c.featureRegion != nil }

// SetFeatureStep assigns increasing stage/feature order independently of worker completion.
func (c *Chunk) SetFeatureStep(step, index int) {
	if c != nil && c.featureRegion != nil {
		c.featureRegion.priority = uint32(step)<<16 | (uint32(index) + 2)
	}
}

// SetFeatureWriteRadius selects the current feature's bounded neighboring writes.
func (c *Chunk) SetFeatureWriteRadius(radius int) {
	if c != nil && c.featureRegion != nil && radius >= 1 && radius <= c.featureRegion.borderWrites.radius {
		c.featureRegion.radius = radius
	}
}

// SetStructureStep reserves the first stamp of a stage before its configured features.
func (c *Chunk) SetStructureStep(step int) {
	if c != nil && c.featureRegion != nil {
		c.featureRegion.priority = uint32(step)<<16 | 1
		c.featureRegion.radius = 1
	}
}

// FeatureBorderPriorities materializes world-coordinate stamps for compatibility callers.
func (c *Chunk) FeatureBorderPriorities() map[BorderPosition]uint32 {
	if c == nil || c.featureRegion == nil {
		return nil
	}
	priorities := make(map[BorderPosition]uint32)
	c.featureRegion.borderWrites.Range(func(position BorderPosition, write FeatureBorderWrite) { priorities[position] = write.Priority })
	return priorities
}

// FeatureBorderWrites returns the buffer retained for immutable neighboring merges.
func (c *Chunk) FeatureBorderWrites() *FeatureBorderBuffer {
	if c == nil || c.featureRegion == nil {
		return nil
	}
	return c.featureRegion.borderWrites
}

// ReleaseFeatureRegion detaches readers before returning the owned stamp buffer to its pool.
func (c *Chunk) ReleaseFeatureRegion() []uint32 {
	if c == nil || c.featureRegion == nil {
		return nil
	}
	writes := c.featureRegion.writes
	c.featureRegion = nil
	return writes
}

// FinishFeatureReads releases terrain dependencies while retaining stamps needed for final merging.
func (c *Chunk) FinishFeatureReads() {
	if c != nil && c.featureRegion != nil {
		c.featureRegion.readBlock = nil
		c.featureRegion.readHeight = nil
		c.featureRegion.columnWrites = nil
	}
}

// featureWriteBounds restricts decoration writes to the active source radius.
func (c *Chunk) featureWriteBounds(x, z int) bool {
	radius := c.featureRegion.radius
	return x >= int(c.X)*16-radius*16 && x < int(c.X)*16+(radius+1)*16 &&
		z >= int(c.Z)*16-radius*16 && z < int(c.Z)*16+(radius+1)*16
}

// FeatureWriteAllowed reports whether a position is inside the active decoration write radius.
func (c *Chunk) FeatureWriteAllowed(x, y, z int) bool {
	if c == nil || y < c.minY || y > c.maxY {
		return false
	}
	if x >= int(c.X)*16 && x < int(c.X)*16+16 && z >= int(c.Z)*16 && z < int(c.Z)*16+16 {
		return true
	}
	return c.featureRegion != nil && c.featureWriteBounds(x, z)
}

// SetFeatureBlock records local write order or a bounded neighboring decoration update.
func (c *Chunk) SetFeatureBlock(x, y, z int, state int32) {
	if !c.FeatureWriteAllowed(x, y, z) {
		return
	}
	if x < int(c.X)*16 || x >= int(c.X)*16+16 || z < int(c.Z)*16 || z >= int(c.Z)*16+16 {
		c.SetBorderBlock(x, y, z, state)
		return
	}
	c.SetBlock(x, y, z, state)
	if region := c.featureRegion; region != nil {
		region.writes[(y-c.minY)*256+(z&15)*16+(x&15)] = region.priority
	}
}

// ApplyFeatureBorder resolves conflicts in stage order and ascending source X/Z order.
// Sources must be merged in ascending X/Z order; a later source wins equal stamps.
func (c *Chunk) ApplyFeatureBorder(sourceX, sourceZ int32, x, y, z int, state int32, priority uint32) bool {
	if c.featureRegion == nil || priority == 0 || priority >= importedFeaturePriority || y < c.minY || y > c.maxY || x < int(c.X)*16 || x >= int(c.X)*16+16 || z < int(c.Z)*16 || z >= int(c.Z)*16+16 {
		return false
	}
	index := (y-c.minY)*256 + (z&15)*16 + (x & 15)
	current := c.featureRegion.writes[index]
	order := current &^ importedFeaturePriority
	if priority < order || priority == order && current&importedFeaturePriority == 0 && (sourceX < c.X || sourceX == c.X && sourceZ <= c.Z) {
		return false
	}
	c.SetBlock(x, y, z, state)
	c.featureRegion.writes[index] = priority | importedFeaturePriority
	return true
}

// FeatureHeightmap reads a world-coordinate column including the source's own neighboring writes.
func (c *Chunk) FeatureHeightmap(x, z int, kind HeightmapKind) (int, bool) {
	if c == nil || kind >= heightmapKindCount {
		return 0, false
	}
	if x >= int(c.X)*16 && x < int(c.X)*16+16 && z >= int(c.Z)*16 && z < int(c.Z)*16+16 {
		return c.Heightmap(x, z, kind)
	}
	if c.featureRegion == nil || c.featureRegion.readHeight == nil {
		return c.minY, false
	}
	height, found := c.featureRegion.readHeight(x, z, kind)
	if !c.featureWriteBounds(x, z) {
		return height, found
	}
	columns := c.featureRegion.columnWrites
	dx, dz := x-int(c.X)*16, z-int(c.Z)*16
	var highest int
	var changed bool
	if dx >= -16 && dx < 32 && dz >= -16 && dz < 32 {
		column := (dz+16)*48 + dx + 16
		highest = int(columns.heights[column])
		changed = columns.valid[column>>6]&(uint64(1)<<uint(column&63)) != 0
	} else {
		value, ok := columns.outer[[2]int{x, z}]
		highest, changed = int(value), ok
	}
	if !changed || highest < height-1 {
		return height, found
	}
	for y := min(c.maxY, max(height-1, highest)); y >= c.minY; y-- {
		state, _ := c.GetBorderBlock(x, y, z)
		matches := false
		switch kind {
		case HeightmapWorldSurface:
			matches = !block.IsAir(state)
		case HeightmapOceanFloor:
			matches = block.IsSolid(state)
		case HeightmapMotionBlocking:
			matches = blocksMotion(state)
		case HeightmapMotionBlockingNoLeaves:
			matches = (block.IsSolid(state) && !strings.HasSuffix(block.NameFromState(state), "_leaves")) || block.IsWater(state) || block.IsLava(state)
		}
		if matches {
			return y + 1, true
		}
	}
	return c.minY, false
}
