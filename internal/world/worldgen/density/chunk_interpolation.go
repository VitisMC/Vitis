package density

import "math"

// ChunkInterpolation evaluates independent channels at cell corners before block filling.
// The compiled graph still applies outer operations after interpolation.
type ChunkInterpolation struct {
	channels       []*interpolated
	width, height  int
	valueCount     int
	terrainChannel int
	dryCeiling     int
	skipAir        bool
	fillChannels   [5]int
	oreChannel     int
	cornerCache    *cornerColumnCache
}

// interpolationScratch owns reusable buffers for one independent chunk evaluation.
type interpolationScratch struct {
	corners   []float64
	partials  []float64
	pairs     []int
	row       int
	fraction  float64
	x, y, z   int
	active    bool
	empty     []bool
	solid     []bool
	preparing bool
	step      int
}

// ChunkInterpolation captures the compiler's current immutable interpolation channels.
func (c *Compiler) ChunkInterpolation() *ChunkInterpolation {
	c.mu.Lock()
	defer c.mu.Unlock()
	p := &ChunkInterpolation{
		channels: append([]*interpolated(nil), c.interpolators...),
		width:    c.registry.CellWidth, height: c.registry.CellHeight, valueCount: c.nextCacheID,
		terrainChannel: -1,
		oreChannel:     -1,
	}
	if c.standard != nil {
		p.cornerCache = &cornerColumnCache{limit: 4096}
		for index, channel := range p.channels {
			if channel.id == c.standard.oreToggleID {
				p.oreChannel = index
			}
		}
	}
	return p
}

// SetDryCeiling enables proven air-cell elision for a validated standard terrain graph.
func (p *ChunkInterpolation) SetDryCeiling(final Function, ceiling int) {
	if p == nil {
		return
	}
	function, ok := final.(*overworldFillDensity)
	if !ok {
		return
	}
	for offset, id := range function.ids {
		for index, channel := range p.channels {
			if channel.id == id {
				p.fillChannels[offset] = index
			}
		}
	}
	for index, channel := range p.channels {
		if channel.id == function.ids[0] {
			p.terrainChannel, p.dryCeiling = index, ceiling
			p.skipAir = true
			return
		}
	}
}

// Fill visits all blocks with cached channel values, or returns false for unaligned bounds.
// Evaluation and visit belong to the caller; independent chunks never share scratch buffers.
func (p *ChunkInterpolation) Fill(e *Evaluation, visit func(Position)) bool {
	return p.FillCells(e, visit, nil)
}

// WithoutCellElision retains cached interpolation while disabling proofs invalidated by added density.
func (p *ChunkInterpolation) WithoutCellElision() *ChunkInterpolation {
	if p == nil {
		return nil
	}
	copy := *p
	copy.skipAir = false
	return &copy
}

// FillCells additionally emits proven uniform stone cells for the standard Overworld graph.
// The cell callback receives its lower corner and horizontal and vertical dimensions.
func (p *ChunkInterpolation) FillCells(e *Evaluation, visit func(Position), stone func(Position, int, int)) bool {
	if p == nil || e == nil || !e.bounded || len(p.channels) == 0 || len(p.channels) > 32 ||
		p.width <= 0 || 16%p.width != 0 || p.height <= 0 || e.height%p.height != 0 ||
		e.minY%p.height != 0 || e.originX%p.width != 0 || e.originZ%p.width != 0 {
		return false
	}
	work := &e.interpolation
	work.active = false
	work.preparing, work.step = true, p.height
	defer func() { work.active, work.preparing = false, false }()
	channels := len(p.channels)
	horizontalCells, verticalCells := 16/p.width, e.height/p.height
	horizontalCorners, verticalCorners := horizontalCells+1, verticalCells+1
	work.corners = resizeFloats(work.corners, horizontalCorners*horizontalCorners*verticalCorners*channels)
	work.partials = resizeFloats(work.partials, e.height*channels*2)
	if cap(work.pairs) < p.valueCount {
		work.pairs = make([]int, p.valueCount)
	} else {
		work.pairs = work.pairs[:p.valueCount]
	}
	for index := range work.pairs {
		work.pairs[index] = -1
	}
	for channel, function := range p.channels {
		work.pairs[function.id] = channel * 2
	}
	position := Position{Evaluation: e}
	for cx := 0; cx < horizontalCorners; cx++ {
		position.X = e.originX + cx*p.width
		for cz := 0; cz < horizontalCorners; cz++ {
			position.Z = e.originZ + cz*p.width
			if p.cornerCache != nil {
				key := cornerColumnKey{x: position.X, z: position.Z, minY: e.minY, height: e.height}
				base := (cx*horizontalCorners + cz) * verticalCorners * channels
				p.cornerCache.fill(key, work.corners[base:base+verticalCorners*channels], func(column []float64) {
					for cy := 0; cy < verticalCorners; cy++ {
						position.Y = e.minY + cy*p.height
						for channel, function := range p.channels {
							column[cy*channels+channel] = function.function.Sample(position)
						}
					}
				})
				continue
			}
			for cy := 0; cy < verticalCorners; cy++ {
				position.Y = e.minY + cy*p.height
				base := ((cx*horizontalCorners+cz)*verticalCorners + cy) * channels
				for channel, function := range p.channels {
					work.corners[base+channel] = function.function.Sample(position)
				}
			}
		}
	}
	for cx := 0; cx < horizontalCells; cx++ {
		work.preparing = false
		for cz := 0; cz < horizontalCells; cz++ {
			if cap(work.empty) < verticalCells {
				work.empty = make([]bool, verticalCells)
			} else {
				work.empty = work.empty[:verticalCells]
				clear(work.empty)
			}
			if cap(work.solid) < verticalCells {
				work.solid = make([]bool, verticalCells)
			} else {
				work.solid = work.solid[:verticalCells]
				clear(work.solid)
			}
			if p.skipAir {
				for cy := 0; cy < verticalCells; cy++ {
					if e.minY+cy*p.height < p.dryCeiling {
						continue
					}
					empty := true
					for dx := 0; dx < 2; dx++ {
						for dz := 0; dz < 2; dz++ {
							for dy := 0; dy < 2; dy++ {
								corner := (((cx+dx)*horizontalCorners+cz+dz)*verticalCorners+cy+dy)*channels + p.terrainChannel
								if work.corners[corner] > 0 {
									empty = false
								}
							}
						}
					}
					work.empty[cy] = empty
				}
			}
			if stone != nil && p.skipAir && p.oreChannel >= 0 {
				for cy := 0; cy < verticalCells; cy++ {
					if !work.empty[cy] && p.solidCell(work.corners, cx, cy, cz, horizontalCorners, verticalCorners, channels) {
						work.solid[cy] = true
						stone(Position{X: e.originX + cx*p.width, Y: e.minY + cy*p.height, Z: e.originZ + cz*p.width, Evaluation: e}, p.width, p.height)
					}
				}
			}
			for x := 0; x < p.width; x++ {
				xFraction := float64(x) / float64(p.width)
				position.X = e.originX + cx*p.width + x
				for cy := 0; cy < verticalCells; cy++ {
					if work.empty[cy] || work.solid[cy] {
						continue
					}
					a := ((cx*horizontalCorners+cz)*verticalCorners + cy) * channels
					b := (((cx+1)*horizontalCorners+cz)*verticalCorners + cy) * channels
					c := ((cx*horizontalCorners+cz+1)*verticalCorners + cy) * channels
					d := (((cx+1)*horizontalCorners+cz+1)*verticalCorners + cy) * channels
					for channel := range channels {
						x00 := work.corners[a+channel] + xFraction*(work.corners[b+channel]-work.corners[a+channel])
						x10 := work.corners[a+channels+channel] + xFraction*(work.corners[b+channels+channel]-work.corners[a+channels+channel])
						x01 := work.corners[c+channel] + xFraction*(work.corners[d+channel]-work.corners[c+channel])
						x11 := work.corners[c+channels+channel] + xFraction*(work.corners[d+channels+channel]-work.corners[c+channels+channel])
						for y := 0; y < p.height; y++ {
							yFraction := float64(y) / float64(p.height)
							row := ((cy*p.height+y)*channels + channel) * 2
							work.partials[row] = x00 + yFraction*(x10-x00)
							work.partials[row+1] = x01 + yFraction*(x11-x01)
						}
					}
				}
				for z := 0; z < p.width; z++ {
					zFraction := float64(z) / float64(p.width)
					position.Z = e.originZ + cz*p.width + z
					for cy := 0; cy < verticalCells; cy++ {
						if work.empty[cy] || work.solid[cy] {
							continue
						}
						for y := cy * p.height; y < (cy+1)*p.height; y++ {
							work.row, work.fraction = y*channels*2, zFraction
							position.Y = e.minY + y
							work.x, work.y, work.z, work.active = position.X, position.Y, position.Z, true
							visit(position)
							work.active = false
						}
					}
				}
			}
		}
	}
	return true
}

// solidCell proves positive terrain and noodles with an ore toggle below its threshold.
func (p *ChunkInterpolation) solidCell(corners []float64, cx, cy, cz, horizontal, vertical, channels int) bool {
	low := [5]float64{math.Inf(1), math.Inf(1), math.Inf(1), math.Inf(1), math.Inf(1)}
	high := [5]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for dx := 0; dx < 2; dx++ {
		for dz := 0; dz < 2; dz++ {
			for dy := 0; dy < 2; dy++ {
				base := (((cx+dx)*horizontal+cz+dz)*vertical + cy + dy) * channels
				if math.Abs(corners[base+p.oreChannel]) >= float64(float32(0.4)) || corners[base+p.terrainChannel] <= 0 {
					return false
				}
				for offset, channel := range p.fillChannels {
					value := corners[base+channel]
					low[offset], high[offset] = math.Min(low[offset], value), math.Max(high[offset], value)
				}
			}
		}
	}
	if high[1] < 0 {
		return true
	}
	minimumAbs := func(low, high float64) float64 {
		if low <= 0 && high >= 0 {
			return 0
		}
		return math.Min(math.Abs(low), math.Abs(high))
	}
	return low[2]+1.5*math.Max(minimumAbs(low[3], high[3]), minimumAbs(low[4], high[4])) > 0
}

// resizeFloats reuses a scratch allocation when its capacity permits.
func resizeFloats(values []float64, length int) []float64 {
	if cap(values) < length {
		return make([]float64, length)
	}
	return values[:length]
}
