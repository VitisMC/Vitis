package worldgen

import "math"

// zoomCell caches biome membership for the eight corners of one biome-zoom cell.
type zoomCell struct {
	stamp  uint32
	common int32
	biomes [8]int32
}

// zoomCorner caches the immutable seed-derived displacement of one quart coordinate.
type zoomCorner struct {
	stamp        uint32
	displacement [3]float64
}

// surfaceZoom owns reusable dense lookup scratch for one independently processed chunk.
type surfaceZoom struct {
	x, y, z, height int
	stamp           uint32
	seed            int64
	source          biomeSource
	cells           []zoomCell
	corners         []zoomCorner
}

// reset changes the source chunk without retaining biome or jitter results across resources.
func (s *surfaceZoom) reset(system *surfaceSystem, baseX, baseZ, minY, height int, source biomeSource) {
	s.x, s.y, s.z = (baseX-2)>>2, (minY-2)>>2, (baseZ-2)>>2
	s.height = height/4 + 2
	s.seed, s.source = system.zoomSeed, source
	s.stamp++
	length := 6 * 6 * s.height
	if len(s.cells) != length {
		s.cells = make([]zoomCell, length)
		s.corners = make([]zoomCorner, length)
	}
	if s.stamp == 0 {
		clear(s.cells)
		clear(s.corners)
		s.stamp = 1
	}
}

// index maps an absolute quart position into the chunk and its zoom halo.
func (s *surfaceZoom) index(x, y, z int) int { return ((x-s.x)*6+z-s.z)*s.height + y - s.y }

// jitter returns the original seed-mixing result once per quart corner.
func (s *surfaceZoom) jitter(x, y, z int) [3]float64 {
	value := &s.corners[s.index(x, y, z)]
	if value.stamp != s.stamp {
		mixed := lcgNext(s.seed, int64(x))
		mixed = lcgNext(mixed, int64(y))
		mixed = lcgNext(mixed, int64(z))
		mixed = lcgNext(mixed, int64(x))
		mixed = lcgNext(mixed, int64(y))
		mixed = lcgNext(mixed, int64(z))
		value.displacement[0] = fiddle(mixed)
		mixed = lcgNext(mixed, s.seed)
		value.displacement[1] = fiddle(mixed)
		mixed = lcgNext(mixed, s.seed)
		value.displacement[2] = fiddle(mixed)
		value.stamp = s.stamp
	}
	return value.displacement
}

// biome preserves zoom-cell tie order and skips distance work when all corners agree.
func (s *surfaceZoom) biome(x, y, z int) int32 {
	x, y, z = x-2, y-2, z-2
	bx, by, bz := x>>2, y>>2, z>>2
	cell := &s.cells[s.index(bx, by, bz)]
	if cell.stamp != s.stamp {
		cell.common = -1
		for corner := range cell.biomes {
			qx, qy, qz := bx+(corner>>2&1), by+(corner>>1&1), bz+(corner&1)
			cell.biomes[corner] = s.source.sample(qx, qy, qz)
		}
		common := true
		for _, biome := range cell.biomes[1:] {
			common = common && biome == cell.biomes[0]
		}
		if common {
			cell.common = cell.biomes[0]
		}
		cell.stamp = s.stamp
	}
	if cell.common >= 0 {
		return cell.common
	}
	dx, dy, dz := float64(x&3)/4, float64(y&3)/4, float64(z&3)/4
	selected, best := 0, math.Inf(1)
	for corner := range cell.biomes {
		cx, cy, cz := corner>>2&1, corner>>1&1, corner&1
		displacement := s.jitter(bx+cx, by+cy, bz+cz)
		xd, yd, zd := dx-float64(cx)+displacement[0], dy-float64(cy)+displacement[1], dz-float64(cz)+displacement[2]
		distance := xd*xd + yd*yd + zd*zd
		if distance < best {
			best, selected = distance, corner
		}
	}
	return cell.biomes[selected]
}
