package density

import "math"

// Position is an integer block position sampled by a density function.
type Position struct {
	X          int
	Y          int
	Z          int
	Evaluation *Evaluation
}

type cachedValue struct {
	key   uint64
	value float64
	set   bool
}

type interpolationCell struct {
	x, y, z int
	values  [8]float64
	set     bool
}

type interpolationGrid struct {
	values []float64
	set    []uint64
}

type horizontalGrid struct {
	values [17 * 17]float64
	set    [5]uint64
}

type interpolationCellGrid struct {
	values [][8]float64
	set    []uint64
	xSize  int
	ySize  int
	zSize  int
	lastX  int
	lastY  int
	lastZ  int
	last   *[8]float64
}

type interpolationPartials struct {
	values [][2]float64
	set    []uint64
	x, z   int
}

// Evaluation stores marker results for one independent chunk evaluation.
type Evaluation struct {
	values        []map[uint64]float64
	once          []cachedValue
	cells         []interpolationCell
	cellGrids     []interpolationCellGrid
	partials      []interpolationPartials
	grids         []interpolationGrid
	horizontal    []*horizontalGrid
	blended       []*blendedColumnValue
	interpolation interpolationScratch
	normalColumns []normalColumnBatch

	originX, minY, originZ int
	height                 int
	bounded                bool
}

func (e *Evaluation) interpolationCellValue(id, baseX, baseY, baseZ, width, height int, function Function, position Position) *[8]float64 {
	if id < len(e.cellGrids) {
		grid := &e.cellGrids[id]
		if grid.last != nil && grid.lastX == baseX && grid.lastY == baseY && grid.lastZ == baseZ {
			return grid.last
		}
	}
	if !e.bounded || width <= 0 || height <= 0 {
		cell := e.interpolationCell(id)
		if cell.set && cell.x == baseX && cell.y == baseY && cell.z == baseZ {
			return &e.cells[id].values
		}
		values := sampleInterpolationCell(id, function, position, baseX, baseY, baseZ, width, height)
		e.storeInterpolationCell(id, interpolationCell{x: baseX, y: baseY, z: baseZ, values: values, set: true})
		return &e.cells[id].values
	}

	dx, dy, dz := baseX-e.originX, baseY-e.minY, baseZ-e.originZ
	if dx < 0 || dx >= 16 || dz < 0 || dz >= 16 || dy < 0 || dy >= e.height || dx%width != 0 || dz%width != 0 || dy%height != 0 {
		cell := e.interpolationCell(id)
		if cell.set && cell.x == baseX && cell.y == baseY && cell.z == baseZ {
			return &e.cells[id].values
		}
		values := sampleInterpolationCell(id, function, position, baseX, baseY, baseZ, width, height)
		e.storeInterpolationCell(id, interpolationCell{x: baseX, y: baseY, z: baseZ, values: values, set: true})
		return &e.cells[id].values
	}
	for len(e.cellGrids) <= id {
		e.cellGrids = append(e.cellGrids, interpolationCellGrid{})
	}
	grid := &e.cellGrids[id]
	if grid.values == nil {
		grid.xSize = (16 + width - 1) / width
		grid.ySize = (e.height + height - 1) / height
		grid.zSize = grid.xSize
		entries := grid.xSize * grid.ySize * grid.zSize
		grid.values = make([][8]float64, entries)
		grid.set = make([]uint64, (entries+63)/64)
	}
	index := ((dx/width)*grid.zSize+dz/width)*grid.ySize + dy/height
	word, bit := index>>6, uint(index&63)
	if grid.set[word]&(uint64(1)<<bit) != 0 {
		grid.lastX, grid.lastY, grid.lastZ = baseX, baseY, baseZ
		grid.last = &grid.values[index]
		return grid.last
	}
	values := sampleInterpolationCell(id, function, position, baseX, baseY, baseZ, width, height)
	// Nested interpolation may grow cellGrids, so reacquire the grid after sampling.
	grid = &e.cellGrids[id]
	grid.values[index] = values
	grid.set[word] |= uint64(1) << bit
	grid.lastX, grid.lastY, grid.lastZ = baseX, baseY, baseZ
	grid.last = &grid.values[index]
	return grid.last
}

func sampleInterpolationCell(id int, function Function, parent Position, baseX, baseY, baseZ, width, height int) [8]float64 {
	corner := func(x, y, z int) float64 {
		if parent.Evaluation != nil {
			return parent.Evaluation.interpolationSample(id, x, y, z, width, height, function)
		}
		return function.Sample(Position{X: x, Y: y, Z: z, Evaluation: parent.Evaluation})
	}
	return [8]float64{
		corner(baseX, baseY, baseZ),
		corner(baseX+width, baseY, baseZ),
		corner(baseX, baseY+height, baseZ),
		corner(baseX+width, baseY+height, baseZ),
		corner(baseX, baseY, baseZ+width),
		corner(baseX+width, baseY, baseZ+width),
		corner(baseX, baseY+height, baseZ+width),
		corner(baseX+width, baseY+height, baseZ+width),
	}
}

func (e *Evaluation) interpolationCell(id int) interpolationCell {
	for len(e.cells) <= id {
		e.cells = append(e.cells, interpolationCell{})
	}
	return e.cells[id]
}

func (e *Evaluation) storeInterpolationCell(id int, cell interpolationCell) {
	for len(e.cells) <= id {
		e.cells = append(e.cells, interpolationCell{})
	}
	e.cells[id] = cell
}

// NewEvaluation creates an isolated density cache for a chunk generation pass.
func NewEvaluation() *Evaluation {
	return &Evaluation{}
}

// NewChunkEvaluation creates density caches specialized for one chunk column.
func NewChunkEvaluation(originX, minY, originZ, height int) *Evaluation {
	return &Evaluation{originX: originX, minY: minY, originZ: originZ, height: height, bounded: height > 0}
}

// ResetChunk clears cached values before reusing an evaluation for another chunk.
func (e *Evaluation) ResetChunk(originX, minY, originZ, height int) {
	e.interpolation.active = false
	e.interpolation.preparing = false
	for index := range e.normalColumns {
		e.normalColumns[index].set = false
	}
	e.originX, e.minY, e.originZ, e.height = originX, minY, originZ, height
	e.bounded = height > 0
	for _, value := range e.blended {
		if value != nil {
			value.set = false
		}
	}
	for _, values := range e.values {
		clear(values)
	}
	for index := range e.once {
		e.once[index].set = false
	}
	for index := range e.cells {
		e.cells[index].set = false
	}
	for index := range e.cellGrids {
		grid := &e.cellGrids[index]
		clear(grid.set)
		grid.last = nil
	}
	for index := range e.partials {
		clear(e.partials[index].set)
	}
	for index := range e.grids {
		clear(e.grids[index].set)
	}
	for index := range e.horizontal {
		if e.horizontal[index] != nil {
			clear(e.horizontal[index].set[:])
		}
	}
}

func (e *Evaluation) interpolationSample(id, x, y, z, width, height int, function Function) float64 {
	if !e.bounded || width <= 0 || height <= 0 {
		return e.sample(id, Position{X: x, Y: y, Z: z, Evaluation: e}, function)
	}
	dx, dy, dz := x-e.originX, y-e.minY, z-e.originZ
	if dx < 0 || dx > 16 || dz < 0 || dz > 16 || dy < 0 || dy > e.height || dx%width != 0 || dz%width != 0 || dy%height != 0 {
		return e.sample(id, Position{X: x, Y: y, Z: z, Evaluation: e}, function)
	}
	xSize, ySize := 16/width+1, e.height/height+1
	index := (dx/width*ySize+dy/height)*xSize + dz/width
	for len(e.grids) <= id {
		e.grids = append(e.grids, interpolationGrid{})
	}
	grid := e.grids[id]
	if grid.values == nil {
		entries := xSize * ySize * xSize
		grid.values = make([]float64, entries)
		grid.set = make([]uint64, (entries+63)/64)
		e.grids[id] = grid
	}
	word, bit := index>>6, uint(index&63)
	if grid.set[word]&(uint64(1)<<bit) != 0 {
		return grid.values[index]
	}
	value := function.Sample(Position{X: x, Y: y, Z: z, Evaluation: e})
	grid = e.grids[id]
	grid.values[index] = value
	grid.set[word] |= uint64(1) << bit
	e.grids[id] = grid
	return value
}

// cache returns the position cache assigned to one compiled marker.
func (e *Evaluation) cache(id int) map[uint64]float64 {
	for len(e.values) <= id {
		e.values = append(e.values, nil)
	}
	if e.values[id] == nil {
		e.values[id] = make(map[uint64]float64, 16)
	}
	return e.values[id]
}

// positionKey packs a valid world position into a comparable cache key.
func positionKey(position Position) uint64 {
	return uint64(uint32(position.X)&0x3ffffff)<<38 |
		uint64(uint32(position.Z)&0x3ffffff)<<12 |
		uint64(uint32(position.Y)&0xfff)
}

// sample returns a persistent marker result for one position.
func (e *Evaluation) sample(id int, position Position, function Function) float64 {
	if e == nil {
		return function.Sample(position)
	}
	values := e.cache(id)
	key := positionKey(position)
	if value, ok := values[key]; ok {
		return value
	}
	value := function.Sample(position)
	values[key] = value
	return value
}

// sampleHorizontal caches a column value inside the current chunk.
func (e *Evaluation) sampleHorizontal(id int, position Position, function Function) float64 {
	if e == nil || !e.bounded || position.Y != 0 {
		return e.sample(id, position, function)
	}
	dx, dz := position.X-e.originX, position.Z-e.originZ
	if dx < 0 || dx > 16 || dz < 0 || dz > 16 {
		return e.sample(id, position, function)
	}
	for len(e.horizontal) <= id {
		e.horizontal = append(e.horizontal, nil)
	}
	grid := e.horizontal[id]
	if grid == nil {
		grid = &horizontalGrid{}
		e.horizontal[id] = grid
	}
	index := dx*17 + dz
	word, mask := index>>6, uint64(1)<<uint(index&63)
	if grid.set[word]&mask != 0 {
		return grid.values[index]
	}
	value := function.Sample(position)
	grid.values[index] = value
	grid.set[word] |= mask
	return value
}

// sampleOnce caches only the last value observed by one marker.
func (e *Evaluation) sampleOnce(id int, position Position, function Function) float64 {
	if e == nil {
		return function.Sample(position)
	}
	for len(e.once) <= id {
		e.once = append(e.once, cachedValue{})
	}
	key := positionKey(position)
	if cached := e.once[id]; cached.set && cached.key == key {
		return cached.value
	}
	value := function.Sample(position)
	e.once[id] = cachedValue{key: key, value: value, set: true}
	return value
}

// Function computes density at a block position.
type Function interface {
	Sample(Position) float64
}

type functionFunc func(Position) float64

func (f functionFunc) Sample(position Position) float64 { return f(position) }

type boundedFunction struct {
	function Function
	minimum  float64
	maximum  float64
}

func (f *boundedFunction) Sample(position Position) float64 { return f.function.Sample(position) }

func withBounds(function Function, minimum, maximum float64) Function {
	return &boundedFunction{function: function, minimum: minimum, maximum: maximum}
}

func unwrapBounds(function Function) (Function, float64, float64) {
	if bounded, ok := function.(*boundedFunction); ok {
		return bounded.function, bounded.minimum, bounded.maximum
	}
	if value, ok := function.(constant); ok {
		return function, float64(value), float64(value)
	}
	return function, math.Inf(-1), math.Inf(1)
}

type constant float64

func (c constant) Sample(Position) float64 { return float64(c) }

// Marker identifies evaluation hints that the chunk sampler applies in context.
type Marker struct {
	ID       int
	Kind     string
	Function Function
}

// Sample evaluates the wrapped function using the active chunk cache.
func (m *Marker) Sample(position Position) float64 {
	switch m.Kind {
	case "flat_cache":
		position.X = floorToMultiple(position.X, 4)
		position.Y = 0
		position.Z = floorToMultiple(position.Z, 4)
	case "cache_2d":
		position.Y = 0
	}
	if m.Kind == "cache_once" {
		if position.Evaluation == nil {
			return m.Function.Sample(position)
		}
		return position.Evaluation.sampleOnce(m.ID, position, m.Function)
	}
	if position.Evaluation == nil {
		return m.Function.Sample(position)
	}
	if m.Kind == "cache_2d" || m.Kind == "flat_cache" {
		return position.Evaluation.sampleHorizontal(m.ID, position, m.Function)
	}
	return position.Evaluation.sample(m.ID, position, m.Function)
}
