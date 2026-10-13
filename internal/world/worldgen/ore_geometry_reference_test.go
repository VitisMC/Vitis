package worldgen

import (
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
	"math"
	"sync"
)

// referenceOreVisitedPool retains the scalar reference's packed visitation layout.
var referenceOreVisitedPool = sync.Pool{New: func() any { return make([]uint64, 0, 512) }}

// oreSpheresPool keeps the unmodified reference's independent slice allocation path.
var oreSpheresPool = sync.Pool{New: func() any { return make([][4]float64, 0, 64) }}

// placeGeometryReference preserves the scalar sphere traversal for optimized-geometry comparisons.
func (o *configuredOre) placeGeometryReference(chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	if o.scattered {
		return o.placeScattered(chunk, random, origin)
	}
	angle := random.NextFloat() * float32(math.Pi)
	radius := float32(o.size) / 8
	x0 := float64(origin.x) + math.Sin(float64(angle))*float64(radius)
	x1 := float64(origin.x) - math.Sin(float64(angle))*float64(radius)
	z0 := float64(origin.z) + math.Cos(float64(angle))*float64(radius)
	z1 := float64(origin.z) - math.Cos(float64(angle))*float64(radius)
	y0 := float64(origin.y + int(random.NextIntBound(3)) - 2)
	y1 := float64(origin.y + int(random.NextIntBound(3)) - 2)
	if !oreMayIntersectSurface(chunk, origin, o.size) {
		return false
	}
	spheres := oreSpheresPool.Get().([][4]float64)
	if cap(spheres) < o.size {
		spheres = make([][4]float64, o.size)
	} else {
		spheres = spheres[:o.size]
	}
	defer func() { oreSpheresPool.Put(spheres[:0]) }()
	for i := 0; i < o.size; i++ {
		t := float32(i) / float32(o.size)
		size := random.NextDouble() * float64(o.size) / 16
		sphereRadius := (float64(mthSin(float32(math.Pi)*t)+1)*size + 1) / 2
		spheres[i] = [4]float64{x0 + float64(t)*(x1-x0), y0 + float64(t)*(y1-y0), z0 + float64(t)*(z1-z0), sphereRadius}
	}
	for i := 0; i < len(spheres)-1; i++ {
		if spheres[i][3] <= 0 {
			continue
		}
		for j := i + 1; j < len(spheres); j++ {
			if spheres[j][3] <= 0 {
				continue
			}
			dx, dy, dz, dr := spheres[i][0]-spheres[j][0], spheres[i][1]-spheres[j][1], spheres[i][2]-spheres[j][2], spheres[i][3]-spheres[j][3]
			if dr*dr > dx*dx+dy*dy+dz*dz {
				if dr > 0 {
					spheres[j][3] = -1
				} else {
					spheres[i][3] = -1
				}
			}
		}
	}
	if len(spheres) == 0 {
		return false
	}
	minX, minY, minZ := math.MaxInt, math.MaxInt, math.MaxInt
	maxX, maxY, maxZ := math.MinInt, math.MinInt, math.MinInt
	for _, sphere := range spheres {
		r := sphere[3]
		if r < 0 {
			continue
		}
		minX = min(minX, int(math.Floor(sphere[0]-r)))
		minY = min(minY, int(math.Floor(sphere[1]-r)))
		minZ = min(minZ, int(math.Floor(sphere[2]-r)))
		maxX = max(maxX, int(math.Floor(sphere[0]+r)))
		maxY = max(maxY, int(math.Floor(sphere[1]+r)))
		maxZ = max(maxZ, int(math.Floor(sphere[2]+r)))
	}
	if minX > maxX {
		return false
	}
	baseX, baseZ := int(chunk.X)*16, int(chunk.Z)*16
	minX, maxX = max(minX, baseX), min(maxX, baseX+15)
	minZ, maxZ = max(minZ, baseZ), min(maxZ, baseZ+15)
	minY, maxY = max(minY, chunk.MinY()), min(maxY, chunk.MaxY())
	if minX > maxX || minY > maxY || minZ > maxZ {
		return false
	}
	ySize, zSize := maxY-minY+1, maxZ-minZ+1
	bitCount := (maxX - minX + 1) * ySize * zSize
	wordCount := (bitCount + 63) >> 6
	visited := referenceOreVisitedPool.Get().([]uint64)
	if cap(visited) < wordCount {
		visited = make([]uint64, wordCount)
	} else {
		visited = visited[:wordCount]
		clear(visited)
	}
	defer func() { referenceOreVisitedPool.Put(visited[:0]) }()

	placed := false
	for _, sphere := range spheres {
		r := sphere[3]
		if r < 0 {
			continue
		}
		xStart, xEnd := max(minX, int(math.Floor(sphere[0]-r))), min(maxX, int(math.Floor(sphere[0]+r)))
		yStart, yEnd := max(minY, int(math.Floor(sphere[1]-r))), min(maxY, int(math.Floor(sphere[1]+r)))
		zStart, zEnd := max(minZ, int(math.Floor(sphere[2]-r))), min(maxZ, int(math.Floor(sphere[2]+r)))
		var zSquared [16]float64
		for z := zStart; z <= zEnd; z++ {
			dz := (float64(z) + .5 - sphere[2]) / r
			zSquared[z-baseZ] = dz * dz
		}
		for x := xStart; x <= xEnd; x++ {
			dx := (float64(x) + .5 - sphere[0]) / r
			if dx*dx >= 1 {
				continue
			}
			for y := yStart; y <= yEnd; y++ {
				dy := (float64(y) + .5 - sphere[1]) / r
				distance := dx*dx + dy*dy
				if distance >= 1 {
					continue
				}
				row := ((x-minX)*ySize+(y-minY))*zSize - minZ
				for z := zStart; z <= zEnd; z++ {
					if distance+zSquared[z-baseZ] < 1 {
						index := row + z
						word, bit := index>>6, uint(index&63)
						if visited[word]&(uint64(1)<<bit) == 0 {
							visited[word] |= uint64(1) << bit
							placed = o.tryPlace(chunk, random, blockPosition{x, y, z}) || placed
						}
					}
				}
			}
		}
	}
	return placed
}
