package worldgen

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/vitismc/vitis/internal/block"
	blockfluid "github.com/vitismc/vitis/internal/block/fluid"
	generatedbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	"github.com/vitismc/vitis/internal/world/worldgen/density"
	"github.com/vitismc/vitis/internal/world/worldgen/noise"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

const noWaterHeight = math.MinInt32

type surfaceRule interface {
	apply(*surfaceContext) (int32, bool)
}

type surfaceCondition interface {
	test(*surfaceContext) bool
}

type surfaceSystem struct {
	defaultBlock         int32
	seaLevel             int
	minY                 int
	height               int
	cellHeight           int
	seed                 int64
	zoomSeed             int64
	biomes               biomeSource
	initial              density.Function
	rule                 surfaceRule
	random               worldrandom.PositionalFactory
	surface              *noise.Normal
	secondary            *noise.Normal
	clayOffset           *noise.Normal
	clayBands            [192]int32
	temperature          *noise.PerlinSimplex
	frozenTemp           *noise.PerlinSimplex
	biomeInfo            *noise.PerlinSimplex
	columnConditionCount int
	standard             bool
	standardNether       bool
	standardConditions   []surfaceCondition
	standardStates       []int32
	standardGradients    []worldrandom.PositionalFactory
	zoomPool             sync.Pool

	preliminaryMu    sync.RWMutex
	preliminaryCache map[uint64]int
	preliminaryKeys  []uint64
	preliminaryNext  int
}

const preliminarySurfaceCacheCapacity = 8192

type surfaceContext struct {
	system           *surfaceSystem
	chunk            *section.Chunk
	x, y, z          int
	localX, localZ   int
	surfaceDepth     int
	surfaceSecondary float64
	waterHeight      int
	stoneDepthAbove  int
	stoneDepthBelow  int
	biome            int32
	biomeSet         bool
	biomeCache       map[biomeQuartPosition]int32
	biomes           biomeSource
	minSurfaceLevel  int
	heights          *[16][16]int
	columnConditions []uint8
	zoom             *surfaceZoom
}

type biomeQuartPosition struct{ x, y, z int }

func compileSurfaceSystem(resources *ResourceSet, settings NoiseSettings, compiler *density.Compiler, seed int64, defaultBlock int32, biomes biomeSource, initial density.Function) (*surfaceSystem, error) {
	var source worldrandom.Source = worldrandom.NewXoroshiro(seed)
	if settings.LegacyRandomSource {
		source = worldrandom.NewLegacy(seed)
	}
	factory := source.ForkPositional()
	resolveNoise := func(id string) (*noise.Normal, error) {
		parameters, ok := resources.noise[id]
		if !ok {
			return nil, fmt.Errorf("surface: unknown noise %s", id)
		}
		return noise.NewNormal(factory.FromHashOf(id), parameters)
	}
	surfaceNoise, err := resolveNoise("minecraft:surface")
	if err != nil {
		return nil, err
	}
	secondary, err := resolveNoise("minecraft:surface_secondary")
	if err != nil {
		return nil, err
	}
	clayOffset, err := resolveNoise("minecraft:clay_bands_offset")
	if err != nil {
		return nil, err
	}
	temperature, _ := noise.NewPerlinSimplex(worldrandom.NewLegacy(1234), []int{0})
	frozenTemperature, _ := noise.NewPerlinSimplex(worldrandom.NewLegacy(3456), []int{-2, -1, 0})
	biomeInfo, _ := noise.NewPerlinSimplex(worldrandom.NewLegacy(2345), []int{0})
	system := &surfaceSystem{
		defaultBlock: defaultBlock, seaLevel: settings.SeaLevel,
		minY: settings.Noise.MinY, height: settings.Noise.Height, cellHeight: settings.Noise.SizeVertical * 4,
		seed: seed, zoomSeed: biomeZoomSeed(seed), biomes: biomes, initial: initial, random: factory,
		surface: surfaceNoise, secondary: secondary, clayOffset: clayOffset,
		temperature: temperature, frozenTemp: frozenTemperature, biomeInfo: biomeInfo,
		preliminaryCache: make(map[uint64]int, preliminarySurfaceCacheCapacity),
		preliminaryKeys:  make([]uint64, 0, preliminarySurfaceCacheCapacity),
	}
	if err := system.generateClayBands(); err != nil {
		return nil, err
	}
	rule, err := system.compileRule(settings.SurfaceRule, resources, resolveNoise)
	if err != nil {
		return nil, fmt.Errorf("surface rule: %w", err)
	}
	system.rule = rule
	if resources.standardTerrainSettings("minecraft:overworld") && string(settings.SurfaceRule) == string(resources.noiseSettings["minecraft:overworld"].SurfaceRule) {
		if err := system.bindStandardSurface(resolveNoise); err != nil {
			return nil, err
		}
		system.standard = true
	}
	if resources.standardTerrainSettings("minecraft:nether") && string(settings.SurfaceRule) == string(resources.noiseSettings["minecraft:nether"].SurfaceRule) {
		if err := system.bindStandardNetherSurface(resolveNoise); err != nil {
			return nil, err
		}
		system.standardNether = true
	}
	return system, nil
}

func (s *surfaceSystem) apply(chunk *section.Chunk, chunkX, chunkZ int32, biomes biomeSource) {
	if s == nil || s.rule == nil || chunk == nil {
		return
	}
	s.applyWithHeights(chunk, chunkX, chunkZ, biomes, surfaceHeights(chunk))
}

// applyWithHeights uses terrain's existing column heights without scanning the working buffer.
func (s *surfaceSystem) applyWithHeights(chunk *section.Chunk, chunkX, chunkZ int32, biomes biomeSource, heights [16][16]int) {
	if s == nil || s.rule == nil || chunk == nil {
		return
	}
	baseX, baseZ := int(chunkX)*16, int(chunkZ)*16
	preliminary := s.preliminarySurfaceCorners(baseX, baseZ)
	columnConditions := make([]uint8, s.columnConditionCount)
	if biomes == nil {
		biomes = s.biomes
	}
	var zoom *surfaceZoom
	if cached := s.zoomPool.Get(); cached != nil {
		zoom = cached.(*surfaceZoom)
	} else {
		zoom = &surfaceZoom{}
	}
	zoom.reset(s, baseX, baseZ, chunk.MinY(), chunk.MaxY()-chunk.MinY()+1, biomes)
	defer func() { zoom.source = nil; s.zoomPool.Put(zoom) }()
	var context surfaceContext
	blocks := chunk.DenseBlocks()
	minY, maxY := chunk.MinY(), chunk.MaxY()
	for localX := 0; localX < 16; localX++ {
		for localZ := 0; localZ < 16; localZ++ {
			clear(columnConditions)
			x, z := baseX+localX, baseZ+localZ
			depth := int(s.surface.SampleXZ(float64(x), float64(z))*2.75 + 3 + worldrandom.FirstDoubleAt(s.random, int32(x), 0, int32(z))*0.25)
			context = surfaceContext{
				system: s, chunk: chunk, x: x, z: z, localX: localX, localZ: localZ,
				surfaceDepth: depth, surfaceSecondary: s.secondary.SampleXZ(float64(x), float64(z)),
				heights: &heights, biomes: biomes, zoom: zoom,
				columnConditions: columnConditions,
			}
			context.minSurfaceLevel = minimumSurfaceLevel(preliminary, x, z, depth)
			stoneDepthAbove, waterHeight := 0, noWaterHeight
			stoneBottom := math.MaxInt
			for y := min(maxY, heights[localX][localZ]+2); y >= minY; y-- {
				var state int32
				if blocks != nil {
					state = blocks[(y-minY)*256+localZ*16+localX]
				} else {
					state = chunk.GetBlock(localX, y, localZ)
				}
				if s.standard && y < context.minSurfaceLevel {
					if state == s.defaultBlock {
						context.y = y
						if replacement, ok := applyStandardDeepSurface(&context); ok {
							chunk.SetBlock(localX, y, localZ, replacement)
						}
					}
					continue
				}
				if isAirState(state) {
					stoneDepthAbove, waterHeight = 0, noWaterHeight
					continue
				}
				if blockfluid.IsFluid(state) {
					if waterHeight == noWaterHeight {
						waterHeight = y + 1
					}
					continue
				}
				if stoneBottom >= y {
					stoneBottom = chunk.MinY() - 1
					for below := y - 1; below >= minY-1; below-- {
						var belowState int32
						if below >= minY {
							if blocks != nil {
								belowState = blocks[(below-minY)*256+localZ*16+localX]
							} else {
								belowState = chunk.GetBlock(localX, below, localZ)
							}
						}
						if below < minY || !isStoneState(belowState) {
							stoneBottom = below + 1
							break
						}
					}
				}
				stoneDepthAbove++
				context.y, context.waterHeight = y, waterHeight
				context.stoneDepthAbove, context.stoneDepthBelow = stoneDepthAbove, y-stoneBottom+1
				context.biomeSet = false
				if state == s.defaultBlock {
					var replacement int32
					var ok bool
					if s.standard {
						replacement, ok = applyStandardSurface(&context)
					} else if s.standardNether {
						replacement, ok = applyStandardNetherSurface(&context)
					} else {
						replacement, ok = s.rule.apply(&context)
					}
					if ok {
						chunk.SetBlock(localX, y, localZ, replacement)
					}
				}
			}
		}
	}
}

func (s *surfaceSystem) topMaterial(chunk *section.Chunk, localX, y, localZ, x, z int, waterAbove bool) (int32, bool) {
	if s == nil || s.rule == nil {
		return 0, false
	}
	// Steepness is the only rule that reads neighboring column heights here.
	var heights [16][16]int
	for _, coordinate := range [][2]int{{localX, max(localZ-1, 0)}, {localX, min(localZ+1, 15)}, {max(localX-1, 0), localZ}, {min(localX+1, 15), localZ}} {
		heights[coordinate[0]][coordinate[1]] = surfaceColumnHeight(chunk, coordinate[0], coordinate[1])
	}
	depth := int(s.surface.SampleXZ(float64(x), float64(z))*2.75 + 3 + worldrandom.FirstDoubleAt(s.random, int32(x), 0, int32(z))*0.25)
	waterHeight := noWaterHeight
	if waterAbove {
		waterHeight = y + 1
	}
	context := &surfaceContext{
		system: s, chunk: chunk, x: x, y: y, z: z, localX: localX, localZ: localZ,
		surfaceDepth: depth, surfaceSecondary: s.secondary.SampleXZ(float64(x), float64(z)),
		waterHeight: waterHeight, stoneDepthAbove: 1, stoneDepthBelow: 1, heights: &heights,
		columnConditions: make([]uint8, s.columnConditionCount),
	}
	context.minSurfaceLevel = s.minimumSurfaceLevel(x, z, depth)
	if s.standard {
		return applyStandardSurface(context)
	}
	if s.standardNether {
		return applyStandardNetherSurface(context)
	}
	return s.rule.apply(context)
}

// surfaceColumnHeight scans one column with the surface rule's complete air predicate.
func surfaceColumnHeight(chunk *section.Chunk, x, z int) int {
	if blocks := chunk.DenseBlocks(); blocks != nil {
		for y := chunk.MaxY(); y >= chunk.MinY(); y-- {
			if !isAirState(blocks[(y-chunk.MinY())*256+z*16+x]) {
				return y
			}
		}
		return chunk.MinY() - 1
	}
	for index := len(chunk.Sections) - 1; index >= 0; index-- {
		current := chunk.Sections[index]
		if palette := current.Blocks.Palette(); palette != nil {
			possible := false
			for _, state := range palette {
				possible = possible || !isAirState(state)
			}
			if !possible {
				continue
			}
			if current.Blocks.IsSingle() {
				return chunk.MinY() + (index+1)*section.SectionHeight - 1
			}
		}
		bottom := chunk.MinY() + index*section.SectionHeight
		for y := min(chunk.MaxY(), bottom+section.SectionHeight-1); y >= bottom; y-- {
			if !isAirState(current.GetBlock(x, y, z)) {
				return y
			}
		}
	}
	return chunk.MinY() - 1
}

func (c *surfaceContext) biomeID() int32 {
	if !c.biomeSet {
		if c.zoom != nil {
			c.biome = c.zoom.biome(c.x, c.y, c.z)
		} else {
			c.biome = c.system.biomeAtBlockCached(c.x, c.y, c.z, c.biomeCache, c.biomes)
		}
		c.biomeSet = true
	}
	return c.biome
}

func surfaceHeights(chunk *section.Chunk) [16][16]int {
	var heights [16][16]int
	if blocks := chunk.DenseBlocks(); blocks != nil {
		for x := 0; x < 16; x++ {
			for z := 0; z < 16; z++ {
				heights[x][z] = chunk.MinY() - 1
				for y := chunk.MaxY(); y >= chunk.MinY(); y-- {
					if !isAirState(blocks[(y-chunk.MinY())*256+z*16+x]) {
						heights[x][z] = y
						break
					}
				}
			}
		}
		return heights
	}
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			heights[x][z] = chunk.MinY() - 1
			for sectionIndex := len(chunk.Sections) - 1; sectionIndex >= 0; sectionIndex-- {
				current := chunk.Sections[sectionIndex]
				palette := current.Blocks.Palette()
				if palette != nil {
					containsBlock := false
					for _, state := range palette {
						if !isAirState(state) {
							containsBlock = true
							break
						}
					}
					if !containsBlock {
						continue
					}
					if current.Blocks.IsSingle() {
						heights[x][z] = chunk.MinY() + (sectionIndex+1)*section.SectionHeight - 1
						break
					}
				}
				bottom := chunk.MinY() + sectionIndex*section.SectionHeight
				for y := bottom + section.SectionHeight - 1; y >= bottom; y-- {
					if !isAirState(current.GetBlock(x, y, z)) {
						heights[x][z] = y
						break
					}
				}
				if heights[x][z] >= bottom {
					break
				}
			}
		}
	}
	return heights
}

func (s *surfaceSystem) minimumSurfaceLevel(x, z, depth int) int {
	cellX, cellZ := x>>4, z>>4
	levels := s.preliminarySurfaceCorners(cellX<<4, cellZ<<4)
	return minimumSurfaceLevel(levels, x, z, depth)
}

// preliminarySurfaceCorners samples the four interpolation corners of a chunk.
func (s *surfaceSystem) preliminarySurfaceCorners(baseX, baseZ int) [4]int {
	return [4]int{
		s.preliminarySurface(baseX, baseZ), s.preliminarySurface(baseX+16, baseZ),
		s.preliminarySurface(baseX, baseZ+16), s.preliminarySurface(baseX+16, baseZ+16),
	}
}

// minimumSurfaceLevel interpolates a column's minimum surface from cached corners.
func minimumSurfaceLevel(levels [4]int, x, z, depth int) int {
	if levels[0] == math.MaxInt32 || levels[1] == math.MaxInt32 || levels[2] == math.MaxInt32 || levels[3] == math.MaxInt32 {
		return math.MaxInt32
	}
	xDelta, zDelta := float64(x&15)/16, float64(z&15)/16
	first := float64(levels[0]) + xDelta*float64(levels[1]-levels[0])
	second := float64(levels[2]) + xDelta*float64(levels[3]-levels[2])
	return int(math.Floor(first+zDelta*(second-first))) + depth - 8
}

func (s *surfaceSystem) preliminarySurface(x, z int) int {
	return s.preliminarySurfaceWithEvaluation(x, z, nil)
}

func (s *surfaceSystem) preliminarySurfaceWithEvaluation(x, z int, evaluation *density.Evaluation) int {
	key := uint64(uint32(x))<<32 | uint64(uint32(z))
	s.preliminaryMu.RLock()
	value, ok := s.preliminaryCache[key]
	s.preliminaryMu.RUnlock()
	if ok {
		return value
	}
	value = s.computePreliminarySurface(x, z, evaluation)
	s.preliminaryMu.Lock()
	if s.preliminaryCache == nil {
		s.preliminaryCache = make(map[uint64]int, preliminarySurfaceCacheCapacity)
		s.preliminaryKeys = make([]uint64, 0, preliminarySurfaceCacheCapacity)
	}
	if cached, exists := s.preliminaryCache[key]; exists {
		value = cached
	} else {
		if len(s.preliminaryKeys) < preliminarySurfaceCacheCapacity {
			s.preliminaryKeys = append(s.preliminaryKeys, key)
		} else {
			delete(s.preliminaryCache, s.preliminaryKeys[s.preliminaryNext])
			s.preliminaryKeys[s.preliminaryNext] = key
			s.preliminaryNext = (s.preliminaryNext + 1) % preliminarySurfaceCacheCapacity
		}
		s.preliminaryCache[key] = value
	}
	s.preliminaryMu.Unlock()
	return value
}

func (s *surfaceSystem) computePreliminarySurface(x, z int, evaluation *density.Evaluation) int {
	for y := s.minY + s.height; y >= s.minY; y -= s.cellHeight {
		if s.initial.Sample(density.Position{X: x, Y: y, Z: z, Evaluation: evaluation}) > 0.390625 {
			return y
		}
	}
	return math.MaxInt32
}

var airStateIDs = [3]int32{
	block.DefaultStateID("minecraft:air"),
	block.DefaultStateID("minecraft:cave_air"),
	block.DefaultStateID("minecraft:void_air"),
}

func isAirState(state int32) bool {
	return state == airStateIDs[0] || state == airStateIDs[1] || state == airStateIDs[2]
}

func isStoneState(state int32) bool { return !isAirState(state) && !blockfluid.IsFluid(state) }

func (s *surfaceSystem) compileRule(raw json.RawMessage, resources *ResourceSet, resolveNoise func(string) (*noise.Normal, error)) (surfaceRule, error) {
	var header struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return nil, err
	}
	switch header.Type {
	case "minecraft:block":
		var value struct {
			Result BlockState `json:"result_state"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		state, err := block.BlockStateFromProperties(value.Result.Name, value.Result.Properties)
		if err != nil {
			return nil, err
		}
		return stateSurfaceRule(state.ID()), nil
	case "minecraft:bandlands":
		return bandSurfaceRule{s}, nil
	case "minecraft:sequence":
		var value struct {
			Sequence []json.RawMessage `json:"sequence"`
		}
		if err := json.Unmarshal(raw, &value); err != nil || len(value.Sequence) == 0 {
			return nil, fmt.Errorf("invalid surface sequence")
		}
		rules := make(sequenceSurfaceRule, len(value.Sequence))
		for i, child := range value.Sequence {
			compiled, err := s.compileRule(child, resources, resolveNoise)
			if err != nil {
				return nil, err
			}
			rules[i] = compiled
		}
		return rules, nil
	case "minecraft:condition":
		var value struct {
			If   json.RawMessage `json:"if_true"`
			Then json.RawMessage `json:"then_run"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		condition, err := s.compileCondition(value.If, resources, resolveNoise)
		if err != nil {
			return nil, err
		}
		followup, err := s.compileRule(value.Then, resources, resolveNoise)
		if err != nil {
			return nil, err
		}
		return conditionalSurfaceRule{condition: condition, rule: followup}, nil
	default:
		return nil, fmt.Errorf("unsupported rule type %q", header.Type)
	}
}

type stateSurfaceRule int32

func (r stateSurfaceRule) apply(*surfaceContext) (int32, bool) { return int32(r), true }

type bandSurfaceRule struct{ system *surfaceSystem }

func (r bandSurfaceRule) apply(context *surfaceContext) (int32, bool) {
	offset := int(math.Round(r.system.clayOffset.SampleXZ(float64(context.x), float64(context.z)) * 4))
	return r.system.clayBands[floorMod(context.y+offset, len(r.system.clayBands))], true
}

type sequenceSurfaceRule []surfaceRule

func (r sequenceSurfaceRule) apply(context *surfaceContext) (int32, bool) {
	for _, child := range r {
		if state, ok := child.apply(context); ok {
			return state, true
		}
	}
	return 0, false
}

type conditionalSurfaceRule struct {
	condition surfaceCondition
	rule      surfaceRule
}

func (r conditionalSurfaceRule) apply(context *surfaceContext) (int32, bool) {
	if !r.condition.test(context) {
		return 0, false
	}
	return r.rule.apply(context)
}

type conditionFunc func(*surfaceContext) bool

func (f conditionFunc) test(context *surfaceContext) bool { return f(context) }

func (s *surfaceSystem) generateClayBands() error {
	terracotta, err := block.DefaultBlockState("minecraft:terracotta")
	if err != nil {
		return err
	}
	for i := range s.clayBands {
		s.clayBands[i] = terracotta.ID()
	}
	random := s.random.FromHashOf("minecraft:clay_bands")
	for index := 0; index < len(s.clayBands); index++ {
		index += int(random.NextIntBound(5)) + 1
		if index < len(s.clayBands) {
			s.clayBands[index] = mustDefaultState("minecraft:orange_terracotta")
		}
	}
	s.makeBands(random, 1, "minecraft:yellow_terracotta")
	s.makeBands(random, 2, "minecraft:brown_terracotta")
	s.makeBands(random, 1, "minecraft:red_terracotta")
	count := 9 + int(random.NextIntBound(7))
	placed := 0
	for index := 0; placed < count && index < len(s.clayBands); index += int(random.NextIntBound(16)) + 4 {
		s.clayBands[index] = mustDefaultState("minecraft:white_terracotta")
		if index-1 > 0 && random.NextBoolean() {
			s.clayBands[index-1] = mustDefaultState("minecraft:light_gray_terracotta")
		}
		if index+1 < len(s.clayBands) && random.NextBoolean() {
			s.clayBands[index+1] = mustDefaultState("minecraft:light_gray_terracotta")
		}
		placed++
	}
	return nil
}

func (s *surfaceSystem) makeBands(random worldrandom.Source, baseWidth int, name string) {
	count := 6 + int(random.NextIntBound(10))
	state := mustDefaultState(name)
	for range count {
		width := baseWidth + int(random.NextIntBound(3))
		start := int(random.NextIntBound(int32(len(s.clayBands))))
		for offset := 0; start+offset < len(s.clayBands) && offset < width; offset++ {
			s.clayBands[start+offset] = state
		}
	}
}

func mustDefaultState(name string) int32 {
	state, err := block.DefaultBlockState(name)
	if err != nil {
		panic(err)
	}
	return state.ID()
}

func normalizeResourceID(id string) string {
	if strings.Contains(id, ":") {
		return id
	}
	return "minecraft:" + id
}

func biomeZoomSeed(seed int64) int64 {
	var data [8]byte
	binary.LittleEndian.PutUint64(data[:], uint64(seed))
	digest := sha256.Sum256(data[:])
	return int64(binary.LittleEndian.Uint64(digest[:8]))
}

func lcgNext(seed, salt int64) int64 {
	value := uint64(seed)
	value *= value*6364136223846793005 + 1442695040888963407
	return int64(value + uint64(salt))
}

func fiddle(seed int64) float64 {
	value := (seed >> 24) & 1023
	return (float64(value)/1024 - 0.5) * 0.9
}

func (s *surfaceSystem) biomeAtBlock(x, y, z int) int32 {
	return s.biomeAtBlockCached(x, y, z, nil, s.biomes)
}

func (s *surfaceSystem) biomeAtBlockCached(x, y, z int, cache map[biomeQuartPosition]int32, biomes biomeSource) int32 {
	if biomes == nil {
		biomes = s.biomes
	}
	x, y, z = x-2, y-2, z-2
	baseX, baseY, baseZ := x>>2, y>>2, z>>2
	deltaX, deltaY, deltaZ := float64(x&3)/4, float64(y&3)/4, float64(z&3)/4
	selected, best := 0, math.Inf(1)
	seed := s.zoomSeed
	for corner := 0; corner < 8; corner++ {
		lowX, lowY, lowZ := corner&4 == 0, corner&2 == 0, corner&1 == 0
		quartX, quartY, quartZ := baseX, baseY, baseZ
		dx, dy, dz := deltaX, deltaY, deltaZ
		if !lowX {
			quartX++
			dx--
		}
		if !lowY {
			quartY++
			dy--
		}
		if !lowZ {
			quartZ++
			dz--
		}
		mixed := lcgNext(seed, int64(quartX))
		mixed = lcgNext(mixed, int64(quartY))
		mixed = lcgNext(mixed, int64(quartZ))
		mixed = lcgNext(mixed, int64(quartX))
		mixed = lcgNext(mixed, int64(quartY))
		mixed = lcgNext(mixed, int64(quartZ))
		dx += fiddle(mixed)
		mixed = lcgNext(mixed, seed)
		dy += fiddle(mixed)
		mixed = lcgNext(mixed, seed)
		dz += fiddle(mixed)
		distance := dx*dx + dy*dy + dz*dz
		if distance < best {
			selected, best = corner, distance
		}
	}
	quartX, quartY, quartZ := baseX, baseY, baseZ
	if selected&4 != 0 {
		quartX++
	}
	if selected&2 != 0 {
		quartY++
	}
	if selected&1 != 0 {
		quartZ++
	}
	position := biomeQuartPosition{x: quartX, y: quartY, z: quartZ}
	if biome, ok := cache[position]; ok {
		return biome
	}
	biome := biomes.sample(quartX, quartY, quartZ)
	if cache != nil {
		cache[position] = biome
	}
	return biome
}

func (s *surfaceSystem) coldEnoughToSnow(context *surfaceContext) bool {
	info := generatedbiome.BiomeByID(context.biomeID())
	if info == nil {
		return false
	}
	temperature := float32(info.Temperature)
	if info.Name == "minecraft:frozen_ocean" || info.Name == "minecraft:deep_frozen_ocean" {
		combined := s.frozenTemp.Sample(float64(context.x)*0.05, float64(context.z)*0.05, false)*7 +
			s.biomeInfo.Sample(float64(context.x)*0.2, float64(context.z)*0.2, false)
		if combined < 0.3 && s.biomeInfo.Sample(float64(context.x)*0.09, float64(context.z)*0.09, false) < 0.8 {
			temperature = 0.2
		}
	}
	heightStart := s.seaLevel + 17
	if context.y > heightStart {
		variation := float32(s.temperature.Sample(float64(float32(context.x)/8), float64(float32(context.z)/8), false) * 8)
		temperature -= (variation + float32(context.y-heightStart)) * 0.05 / 40
	}
	return temperature < 0.15
}
