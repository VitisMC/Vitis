package terrain

import (
	"math"

	genblock "github.com/vitismc/vitis/internal/data/generated/block"
	"github.com/vitismc/vitis/internal/world/chunk/section"
)

// NoiseGenerator produces terrain using simplex-like noise for heightmaps.
type NoiseGenerator struct {
	seed    int64
	biomeID int32
}

// NewNoiseGenerator creates a noise-based terrain generator.
func NewNoiseGenerator(seed int64, biomeID int32) *NoiseGenerator {
	return &NoiseGenerator{seed: seed, biomeID: biomeID}
}

// Generate produces a chunk with noise-based terrain at the given chunk coordinates.
func (g *NoiseGenerator) Generate(cx, cz int32) *section.Chunk {
	c := section.NewChunk(cx, cz, section.OverworldSections, g.biomeID)

	for lx := 0; lx < 16; lx++ {
		for lz := 0; lz < 16; lz++ {
			absX := float64(int(cx)*16 + lx)
			absZ := float64(int(cz)*16 + lz)

			height := g.heightAt(absX, absZ)

			c.SetBlock(lx, section.OverworldMinY, lz, genblock.BedrockDefaultState)

			for y := section.OverworldMinY + 1; y < height-3; y++ {
				c.SetBlock(lx, y, lz, genblock.StoneDefaultState)
			}

			for y := max(height-3, section.OverworldMinY+1); y < height; y++ {
				c.SetBlock(lx, y, lz, genblock.DirtDefaultState)
			}

			if height >= 63 {
				c.SetBlock(lx, height, lz, genblock.GrassBlockDefaultState)
			} else {
				c.SetBlock(lx, height, lz, genblock.DirtDefaultState)
				for y := height + 1; y <= 62; y++ {
					c.SetBlock(lx, y, lz, genblock.WaterDefaultState)
				}
			}
		}
	}

	return c
}

// SpawnY returns a reasonable spawn Y for noise terrain.
func (g *NoiseGenerator) SpawnY() int {
	h := g.heightAt(0, 0)
	if h < 63 {
		return 63 + 1
	}
	return h + 1
}

func (g *NoiseGenerator) heightAt(x, z float64) int {
	n1 := octaveNoise(x, z, g.seed, 0.005, 4)
	n2 := octaveNoise(x, z, g.seed+1000, 0.02, 2)

	height := 64.0 + n1*32.0 + n2*8.0
	return int(math.Floor(height))
}

func octaveNoise(x, z float64, seed int64, frequency float64, octaves int) float64 {
	var total float64
	var amplitude float64 = 1.0
	var maxAmplitude float64
	freq := frequency

	for i := 0; i < octaves; i++ {
		total += simplexNoise2D(x*freq, z*freq, seed+int64(i)*31) * amplitude
		maxAmplitude += amplitude
		amplitude *= 0.5
		freq *= 2.0
	}

	return total / maxAmplitude
}

func simplexNoise2D(x, y float64, seed int64) float64 {
	const (
		f2 = 0.3660254037844386
		g2 = 0.21132486540518713
	)

	s := (x + y) * f2
	i := math.Floor(x + s)
	j := math.Floor(y + s)

	t := (i + j) * g2
	x0 := x - (i - t)
	y0 := y - (j - t)

	var i1, j1 float64
	if x0 > y0 {
		i1, j1 = 1, 0
	} else {
		i1, j1 = 0, 1
	}

	x1 := x0 - i1 + g2
	y1 := y0 - j1 + g2
	x2 := x0 - 1.0 + 2.0*g2
	y2 := y0 - 1.0 + 2.0*g2

	ii := int64(i) & 0xFF
	jj := int64(j) & 0xFF

	var n0, n1, n2 float64

	t0 := 0.5 - x0*x0 - y0*y0
	if t0 >= 0 {
		t0 *= t0
		g := grad2D(hash2D(ii, jj, seed), x0, y0)
		n0 = t0 * t0 * g
	}

	t1 := 0.5 - x1*x1 - y1*y1
	if t1 >= 0 {
		t1 *= t1
		g := grad2D(hash2D(ii+int64(i1), jj+int64(j1), seed), x1, y1)
		n1 = t1 * t1 * g
	}

	t2 := 0.5 - x2*x2 - y2*y2
	if t2 >= 0 {
		t2 *= t2
		g := grad2D(hash2D(ii+1, jj+1, seed), x2, y2)
		n2 = t2 * t2 * g
	}

	return 70.0 * (n0 + n1 + n2)
}

func hash2D(x, y, seed int64) int64 {
	h := uint64(seed)
	h ^= uint64(x) * 0x9E3779B97F4A7C15
	h = (h ^ (h >> 30)) * 0xBF58476D1CE4E5B9
	h ^= uint64(y) * 0x517CC1B727220A95
	h = (h ^ (h >> 27)) * 0x94D049BB133111EB
	return int64(h ^ (h >> 31))
}

func grad2D(hash int64, x, y float64) float64 {
	switch hash & 3 {
	case 0:
		return x + y
	case 1:
		return -x + y
	case 2:
		return x - y
	default:
		return -x - y
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
