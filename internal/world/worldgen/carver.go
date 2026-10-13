package worldgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"sync"

	"github.com/vitismc/vitis/internal/block"
	blockfluid "github.com/vitismc/vitis/internal/block/fluid"
	generatedbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	"github.com/vitismc/vitis/internal/world/worldgen/density"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type configuredCarver struct {
	kind        string
	probability float32
	y           heightProvider
	yScale      floatProvider
	lavaLevel   verticalAnchor
	replaceable map[string]struct{}
	horizontal  floatProvider
	vertical    floatProvider
	floor       floatProvider
	rotation    floatProvider
	shape       canyonShape
}

type canyonShape struct {
	distance, thickness, horizontal floatProvider
	widthSmoothness                 int
	verticalDefault, verticalCenter float32
}

type verticalAnchor struct {
	kind   uint8
	offset int
}

func (a verticalAnchor) resolve(minY, height int) int {
	switch a.kind {
	case 1:
		return minY + a.offset
	case 2:
		return minY + height - 1 - a.offset
	default:
		return a.offset
	}
}

type floatProvider interface {
	sample(worldrandom.Source) float32
}

type constantFloat float32

func (p constantFloat) sample(worldrandom.Source) float32 { return float32(p) }

type uniformFloat struct{ min, max float32 }

func (p uniformFloat) sample(random worldrandom.Source) float32 {
	return p.min + random.NextFloat()*(p.max-p.min)
}

type clampedNormalFloat struct {
	mean, deviation float32
	min, max        float32
}

// sample retains the reference float conversion and clamp order.
func (p clampedNormalFloat) sample(random worldrandom.Source) float32 {
	value := p.mean + float32(random.NextGaussian())*p.deviation
	return max(p.min, min(p.max, value))
}

type trapezoidFloat struct{ min, max, plateau float32 }

func (p trapezoidFloat) sample(random worldrandom.Source) float32 {
	span := p.max - p.min
	lowerSlope := (span - p.plateau) / 2
	upperWidth := span - lowerSlope
	return p.min + random.NextFloat()*upperWidth + random.NextFloat()*lowerSlope
}

type heightProvider struct{ min, max verticalAnchor }

func (p heightProvider) sample(random worldrandom.Source, minY, height int) int {
	minimum, maximum := p.min.resolve(minY, height), p.max.resolve(minY, height)
	if minimum >= maximum {
		return minimum
	}
	return minimum + int(random.NextIntBound(int32(maximum-minimum+1)))
}

type carverSystem struct {
	seed         int64
	minY, height int
	carvers      map[string]*configuredCarver
	biomeCarvers map[string][]string
	biomes       biomeSource
	aquifer      aquiferConfig
	surface      *surfaceSystem
	uniform      bool
	uniformIDs   []string
	featureSeeds [][2]int64
	maskPool     sync.Pool
	aquiferPool  sync.Pool
}

func compileCarverResources(snapshot *datapack.Snapshot, resources *ResourceSet) error {
	resources.carvers = make(map[string]*configuredCarver)
	for id, raw := range resources.resources["configured_carver"] {
		carver, err := decodeConfiguredCarver(snapshot, raw)
		if err != nil {
			return fmt.Errorf("configured carver %s: %w", id, err)
		}
		resources.carvers[id] = carver
	}
	resources.biomeCarvers = make(map[string][]string)
	for id, raw := range resources.resources["biome"] {
		var definition struct {
			Carvers json.RawMessage `json:"carvers"`
		}
		if err := json.Unmarshal(raw, &definition); err != nil {
			return fmt.Errorf("biome %s: %w", id, err)
		}
		var entries []json.RawMessage
		var single string
		if err := json.Unmarshal(definition.Carvers, &single); err == nil {
			encoded, _ := json.Marshal(single)
			entries = []json.RawMessage{encoded}
		} else if err := json.Unmarshal(definition.Carvers, &entries); err != nil {
			return fmt.Errorf("biome %s carvers: %w", id, err)
		}
		list := make([]string, len(entries))
		for i, entry := range entries {
			trimmed := bytes.TrimSpace(entry)
			if len(trimmed) > 0 && trimmed[0] == '"' {
				if err := json.Unmarshal(entry, &list[i]); err != nil {
					return fmt.Errorf("biome %s carver %d: %w", id, i, err)
				}
				list[i] = normalizeResourceID(list[i])
				if resources.carvers[list[i]] == nil {
					return fmt.Errorf("biome %s: unknown carver %s", id, list[i])
				}
				continue
			}
			carver, err := decodeConfiguredCarver(snapshot, entry)
			if err != nil {
				return fmt.Errorf("biome %s carver %d: %w", id, i, err)
			}
			key := "\x00" + id + "/" + strconv.Itoa(i)
			resources.carvers[key] = carver
			list[i] = key
		}
		resources.biomeCarvers[id] = list
	}
	return nil
}

func decodeConfiguredCarver(snapshot *datapack.Snapshot, raw json.RawMessage) (*configuredCarver, error) {
	var value struct {
		Type   string `json:"type"`
		Config struct {
			Probability float32         `json:"probability"`
			Y           json.RawMessage `json:"y"`
			YScale      json.RawMessage `json:"yScale"`
			LavaLevel   json.RawMessage `json:"lava_level"`
			Replaceable json.RawMessage `json:"replaceable"`
			Horizontal  json.RawMessage `json:"horizontal_radius_multiplier"`
			Vertical    json.RawMessage `json:"vertical_radius_multiplier"`
			Floor       json.RawMessage `json:"floor_level"`
			Rotation    json.RawMessage `json:"vertical_rotation"`
			Shape       struct {
				Distance        json.RawMessage `json:"distance_factor"`
				Thickness       json.RawMessage `json:"thickness"`
				WidthSmoothness int             `json:"width_smoothness"`
				Horizontal      json.RawMessage `json:"horizontal_radius_factor"`
				VerticalDefault float32         `json:"vertical_radius_default_factor"`
				VerticalCenter  float32         `json:"vertical_radius_center_factor"`
			} `json:"shape"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	if value.Type != "minecraft:cave" && value.Type != "minecraft:nether_cave" && value.Type != "minecraft:canyon" {
		return nil, fmt.Errorf("unsupported type %q", value.Type)
	}
	if value.Config.Probability < 0 || value.Config.Probability > 1 {
		return nil, fmt.Errorf("invalid probability")
	}
	y, err := decodeHeightProvider(value.Config.Y)
	if err != nil {
		return nil, fmt.Errorf("y: %w", err)
	}
	yScale, err := decodeFloatProvider(value.Config.YScale)
	if err != nil {
		return nil, fmt.Errorf("yScale: %w", err)
	}
	lava, err := decodeVerticalAnchor(value.Config.LavaLevel)
	if err != nil {
		return nil, fmt.Errorf("lava level: %w", err)
	}
	replaceable, err := decodeReplaceable(snapshot, value.Config.Replaceable)
	if err != nil {
		return nil, err
	}
	result := &configuredCarver{kind: value.Type, probability: value.Config.Probability, y: y, yScale: yScale, lavaLevel: lava, replaceable: replaceable}
	if value.Type == "minecraft:cave" || value.Type == "minecraft:nether_cave" {
		result.horizontal, err = decodeFloatProvider(value.Config.Horizontal)
		if err != nil {
			return nil, fmt.Errorf("horizontal radius: %w", err)
		}
		result.vertical, err = decodeFloatProvider(value.Config.Vertical)
		if err != nil {
			return nil, fmt.Errorf("vertical radius: %w", err)
		}
		result.floor, err = decodeFloatProvider(value.Config.Floor)
		if err != nil {
			return nil, fmt.Errorf("floor: %w", err)
		}
		return result, nil
	}
	result.rotation, err = decodeFloatProvider(value.Config.Rotation)
	if err != nil {
		return nil, fmt.Errorf("rotation: %w", err)
	}
	result.shape.distance, err = decodeFloatProvider(value.Config.Shape.Distance)
	if err != nil {
		return nil, err
	}
	result.shape.thickness, err = decodeFloatProvider(value.Config.Shape.Thickness)
	if err != nil {
		return nil, err
	}
	result.shape.horizontal, err = decodeFloatProvider(value.Config.Shape.Horizontal)
	if err != nil {
		return nil, err
	}
	if value.Config.Shape.WidthSmoothness <= 0 {
		return nil, fmt.Errorf("invalid canyon width smoothness")
	}
	result.shape.widthSmoothness = value.Config.Shape.WidthSmoothness
	result.shape.verticalDefault = value.Config.Shape.VerticalDefault
	result.shape.verticalCenter = value.Config.Shape.VerticalCenter
	return result, nil
}

func decodeFloatProvider(raw json.RawMessage) (floatProvider, error) {
	var constant float32
	if err := json.Unmarshal(raw, &constant); err == nil {
		if math.IsNaN(float64(constant)) || math.IsInf(float64(constant), 0) {
			return nil, fmt.Errorf("non-finite constant")
		}
		return constantFloat(constant), nil
	}
	var value struct {
		Type              string `json:"type"`
		Min, Max, Plateau float32
		MinInclusive      float32 `json:"min_inclusive"`
		MaxExclusive      float32 `json:"max_exclusive"`
		Mean, Deviation   float32
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	switch value.Type {
	case "minecraft:uniform":
		if value.MaxExclusive <= value.MinInclusive {
			return nil, fmt.Errorf("invalid uniform range")
		}
		return uniformFloat{value.MinInclusive, value.MaxExclusive}, nil
	case "minecraft:clamped_normal":
		if value.Max < value.Min || math.IsNaN(float64(value.Mean)) || math.IsInf(float64(value.Mean), 0) ||
			math.IsNaN(float64(value.Deviation)) || math.IsInf(float64(value.Deviation), 0) {
			return nil, fmt.Errorf("invalid clamped normal float provider")
		}
		return clampedNormalFloat{value.Mean, value.Deviation, value.Min, value.Max}, nil
	case "minecraft:trapezoid":
		if value.Max < value.Min || value.Plateau < 0 || value.Plateau > value.Max-value.Min {
			return nil, fmt.Errorf("invalid trapezoid range")
		}
		return trapezoidFloat{value.Min, value.Max, value.Plateau}, nil
	default:
		return nil, fmt.Errorf("unsupported float provider %q", value.Type)
	}
}

func decodeHeightProvider(raw json.RawMessage) (heightProvider, error) {
	var value struct {
		Type string          `json:"type"`
		Min  json.RawMessage `json:"min_inclusive"`
		Max  json.RawMessage `json:"max_inclusive"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return heightProvider{}, err
	}
	if value.Type != "minecraft:uniform" {
		return heightProvider{}, fmt.Errorf("unsupported height provider %q", value.Type)
	}
	minimum, err := decodeVerticalAnchor(value.Min)
	if err != nil {
		return heightProvider{}, err
	}
	maximum, err := decodeVerticalAnchor(value.Max)
	if err != nil {
		return heightProvider{}, err
	}
	return heightProvider{minimum, maximum}, nil
}

func decodeVerticalAnchor(raw json.RawMessage) (verticalAnchor, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return verticalAnchor{}, err
	}
	count := 0
	result := verticalAnchor{}
	if data, ok := fields["absolute"]; ok {
		count++
		if err := json.Unmarshal(data, &result.offset); err != nil {
			return result, err
		}
	}
	if data, ok := fields["above_bottom"]; ok {
		count++
		result.kind = 1
		if err := json.Unmarshal(data, &result.offset); err != nil {
			return result, err
		}
	}
	if data, ok := fields["below_top"]; ok {
		count++
		result.kind = 2
		if err := json.Unmarshal(data, &result.offset); err != nil {
			return result, err
		}
	}
	if count != 1 {
		return verticalAnchor{}, fmt.Errorf("invalid vertical anchor")
	}
	return result, nil
}

func decodeReplaceable(snapshot *datapack.Snapshot, raw json.RawMessage) (map[string]struct{}, error) {
	var reference string
	if err := json.Unmarshal(raw, &reference); err != nil {
		return nil, fmt.Errorf("replaceable block set must be a tag or block ID")
	}
	values := []string{normalizeResourceID(reference)}
	if len(reference) > 0 && reference[0] == '#' {
		var err error
		values, err = snapshot.ResolveTag("block", reference[1:])
		if err != nil {
			return nil, fmt.Errorf("replaceable %s: %w", reference, err)
		}
	}
	result := make(map[string]struct{}, len(values))
	for _, name := range values {
		if block.InfoByName(name) == nil {
			return nil, fmt.Errorf("replaceable: unknown block %s", name)
		}
		result[name] = struct{}{}
	}
	return result, nil
}

func newCarverSystem(resources *ResourceSet, settings NoiseSettings, seed int64, biomes biomeSource, aquifer aquiferConfig, surface *surfaceSystem) *carverSystem {
	system := &carverSystem{seed: seed, minY: settings.Noise.MinY, height: settings.Noise.Height, carvers: resources.carvers, biomeCarvers: resources.biomeCarvers, biomes: biomes, aquifer: aquifer, surface: surface}
	possible := possibleBiomeNames(biomes)
	if len(possible) != 0 {
		system.uniform = true
		system.uniformIDs = resources.biomeCarvers[possible[0]]
		for _, name := range possible[1:] {
			if !slices.Equal(system.uniformIDs, resources.biomeCarvers[name]) {
				system.uniform = false
				break
			}
		}
	}
	var count int
	for _, ids := range system.biomeCarvers {
		count = max(count, len(ids))
	}
	system.featureSeeds = make([][2]int64, count)
	random := worldrandom.NewLegacy(0)
	for index := range system.featureSeeds {
		random.SetSeed(seed + int64(index))
		system.featureSeeds[index] = [2]int64{random.NextLong(), random.NextLong()}
	}
	return system
}

func (s *carverSystem) apply(chunk *section.Chunk, chunkX, chunkZ int32) {
	if s == nil || len(s.carvers) == 0 || s.uniform && len(s.uniformIDs) == 0 {
		return
	}
	var mask []bool
	if cached := s.maskPool.Get(); cached != nil {
		mask = cached.([]bool)
		clear(mask)
	} else {
		mask = make([]bool, 16*s.height*16)
	}
	defer s.maskPool.Put(mask)
	var aquifer *chunkAquifer
	if cached := s.aquiferPool.Get(); cached != nil {
		aquifer = cached.(*chunkAquifer)
		aquifer.reset()
	} else {
		aquifer = newChunkAquifer(s.aquifer)
	}
	defer s.aquiferPool.Put(aquifer)
	random := worldrandom.NewLegacy(0)
	for offsetX := -8; offsetX <= 8; offsetX++ {
		for offsetZ := -8; offsetZ <= 8; offsetZ++ {
			sourceX, sourceZ := chunkX+int32(offsetX), chunkZ+int32(offsetZ)
			ids := s.uniformIDs
			if !s.uniform {
				biomeID := s.biomes.sample(int(sourceX)*4, 0, int(sourceZ)*4)
				info := generatedbiome.BiomeByID(biomeID)
				if info == nil {
					continue
				}
				ids = s.biomeCarvers[info.Name]
			}
			for index, id := range ids {
				carver := s.carvers[id]
				first, second := s.featureSeeds[index][0], s.featureSeeds[index][1]
				mixed := int64(uint64(int64(sourceX)*first) ^ uint64(int64(sourceZ)*second) ^ uint64(s.seed+int64(index)))
				random.SetSeed(mixed)
				if random.NextFloat() <= carver.probability {
					carver.carve(s, chunk, chunkX, chunkZ, sourceX, sourceZ, random, aquifer, mask)
				}
			}
		}
	}
}

func largeFeatureRandom(seed int64, chunkX, chunkZ int32) worldrandom.Source {
	random := worldrandom.NewLegacy(seed)
	first, second := random.NextLong(), random.NextLong()
	mixed := int64(uint64(int64(chunkX)*first) ^ uint64(int64(chunkZ)*second) ^ uint64(seed))
	random.SetSeed(mixed)
	return random
}

func (c *configuredCarver) carve(system *carverSystem, chunk *section.Chunk, targetX, targetZ, sourceX, sourceZ int32, random worldrandom.Source, aquifer *chunkAquifer, mask []bool) {
	if c.kind == "minecraft:canyon" {
		c.carveCanyon(system, chunk, targetX, targetZ, sourceX, sourceZ, random, aquifer, mask)
		return
	}
	c.carveCave(system, chunk, targetX, targetZ, sourceX, sourceZ, random, aquifer, mask)
}

func (c *configuredCarver) carveCave(system *carverSystem, chunk *section.Chunk, targetX, targetZ, sourceX, sourceZ int32, random worldrandom.Source, aquifer *chunkAquifer, mask []bool) {
	bound := 15
	yScale := 1.0
	if c.kind == "minecraft:nether_cave" {
		bound, yScale = 10, 5
	}
	length := 112
	firstBound := random.NextIntBound(int32(bound)) + 1
	secondBound := random.NextIntBound(firstBound) + 1
	count := int(random.NextIntBound(secondBound))
	for range count {
		x := float64(int(sourceX)*16 + int(random.NextIntBound(16)))
		y := float64(c.y.sample(random, system.minY, system.height))
		z := float64(int(sourceZ)*16 + int(random.NextIntBound(16)))
		horizontal, vertical, floor := float64(c.horizontal.sample(random)), float64(c.vertical.sample(random)), float64(c.floor.sample(random))
		tunnels := 1
		if random.NextIntBound(4) == 0 {
			radius := float32(1) + random.NextFloat()*6
			roomRadius := 1.5 + float64(mthSin(float32(math.Pi/2))*radius)
			c.carveEllipsoid(system, chunk, targetX, targetZ, aquifer, mask, x+1, y, z, roomRadius, roomRadius*float64(c.yScale.sample(random)), floor)
			tunnels += int(random.NextIntBound(4))
		}
		for range tunnels {
			horizontalAngle := random.NextFloat() * float32(math.Pi*2)
			verticalAngle := (random.NextFloat() - 0.5) / 4
			thickness := random.NextFloat()*2 + random.NextFloat()
			if c.kind == "minecraft:nether_cave" {
				thickness *= 2
			} else if random.NextIntBound(10) == 0 {
				thickness *= random.NextFloat()*random.NextFloat()*3 + 1
			}
			end := length - int(random.NextIntBound(int32(length/4)))
			c.carveTunnel(system, chunk, targetX, targetZ, aquifer, mask, random.NextLong(), x, y, z, horizontal, vertical, thickness, horizontalAngle, verticalAngle, 0, end, yScale, floor)
		}
	}
}

func (c *configuredCarver) carveCanyon(system *carverSystem, chunk *section.Chunk, targetX, targetZ, sourceX, sourceZ int32, random worldrandom.Source, aquifer *chunkAquifer, mask []bool) {
	carveRange := 112
	x := float64(int(sourceX)*16 + int(random.NextIntBound(16)))
	y := float64(c.y.sample(random, system.minY, system.height))
	z := float64(int(sourceZ)*16 + int(random.NextIntBound(16)))
	angle := random.NextFloat() * float32(math.Pi*2)
	pitch := c.rotation.sample(random)
	yScale := float64(c.yScale.sample(random))
	thickness := c.shape.thickness.sample(random)
	end := int(float32(carveRange) * c.shape.distance.sample(random))
	if end <= 0 {
		return
	}
	c.carveCanyonTunnel(system, chunk, targetX, targetZ, aquifer, mask, random.NextLong(), x, y, z, thickness, angle, pitch, end, yScale)
}

func (c *configuredCarver) carveCanyonTunnel(system *carverSystem, chunk *section.Chunk, targetX, targetZ int32, aquifer *chunkAquifer, mask []bool, seed int64, x, y, z float64, thickness, angle, pitch float32, end int, yScale float64) {
	random := worldrandom.NewLegacy(seed)
	widths := make([]float32, system.height)
	width := float32(1)
	for i := range widths {
		if i == 0 || random.NextIntBound(int32(c.shape.widthSmoothness)) == 0 {
			width = 1 + random.NextFloat()*random.NextFloat()
		}
		widths[i] = width * width
	}
	angleVelocity, pitchVelocity := float32(0), float32(0)
	for step := 0; step < end; step++ {
		horizontalRadius := 1.5 + float64(mthSin(float32(step)*float32(math.Pi)/float32(end))*thickness)
		verticalRadius := horizontalRadius * yScale
		horizontalRadius *= float64(c.shape.horizontal.sample(random))
		centerFactor := float32(1) - absFloat32(0.5-float32(step)/float32(end))*2
		verticalFactor := c.shape.verticalDefault + c.shape.verticalCenter*centerFactor
		verticalRadius *= float64(verticalFactor * (0.75 + random.NextFloat()*0.25))
		cosPitch, sinPitch := mthCos(pitch), mthSin(pitch)
		x += float64(mthCos(angle) * cosPitch)
		y += float64(sinPitch)
		z += float64(mthSin(angle) * cosPitch)
		pitch *= 0.7
		pitch += pitchVelocity * 0.05
		angle += angleVelocity * 0.05
		pitchVelocity *= 0.8
		angleVelocity *= 0.5
		pitchVelocity += (random.NextFloat() - random.NextFloat()) * random.NextFloat() * 2
		angleVelocity += (random.NextFloat() - random.NextFloat()) * random.NextFloat() * 4
		if random.NextIntBound(4) == 0 {
			continue
		}
		if !carverCanReach(targetX, targetZ, x, z, step, end, thickness) {
			return
		}
		c.carveCanyonEllipsoid(system, chunk, targetX, targetZ, aquifer, mask, x, y, z, horizontalRadius, verticalRadius, widths)
	}
}

func (c *configuredCarver) carveCanyonEllipsoid(system *carverSystem, chunk *section.Chunk, targetX, targetZ int32, aquifer *chunkAquifer, mask []bool, centerX, centerY, centerZ, horizontalRadius, verticalRadius float64, widths []float32) bool {
	middleX, middleZ := float64(int(targetX)*16+8), float64(int(targetZ)*16+8)
	if math.Abs(centerX-middleX) > 16+horizontalRadius*2 || math.Abs(centerZ-middleZ) > 16+horizontalRadius*2 {
		return false
	}
	baseX, baseZ := int(targetX)*16, int(targetZ)*16
	minX := max(int(math.Floor(centerX-horizontalRadius))-baseX-1, 0)
	maxX := min(int(math.Floor(centerX+horizontalRadius))-baseX, 15)
	minY := max(int(math.Floor(centerY-verticalRadius))-1, system.minY+1)
	maxY := min(int(math.Floor(centerY+verticalRadius))+1, system.minY+system.height-8)
	minZ := max(int(math.Floor(centerZ-horizontalRadius))-baseZ-1, 0)
	maxZ := min(int(math.Floor(centerZ+horizontalRadius))-baseZ, 15)
	changed := false
	for localX := minX; localX <= maxX; localX++ {
		xNorm := (float64(baseX+localX) + 0.5 - centerX) / horizontalRadius
		for localZ := minZ; localZ <= maxZ; localZ++ {
			zNorm := (float64(baseZ+localZ) + 0.5 - centerZ) / horizontalRadius
			if xNorm*xNorm+zNorm*zNorm >= 1 {
				continue
			}
			for blockY := maxY; blockY > minY; blockY-- {
				yNorm := (float64(blockY) - 0.5 - centerY) / verticalRadius
				widthIndex := blockY - system.minY - 1
				if widthIndex < 0 || widthIndex >= len(widths) || (xNorm*xNorm+zNorm*zNorm)*float64(widths[widthIndex])+yNorm*yNorm/6 >= 1 {
					continue
				}
				index := ((localX*16)+localZ)*system.height + (blockY - system.minY)
				if mask[index] {
					continue
				}
				mask[index] = true
				if c.carveBlock(system, chunk, localX, blockY, localZ, baseX+localX, baseZ+localZ, aquifer) {
					changed = true
				}
			}
		}
	}
	return changed
}

func (c *configuredCarver) carveTunnel(system *carverSystem, chunk *section.Chunk, targetX, targetZ int32, aquifer *chunkAquifer, mask []bool, seed int64, x, y, z, horizontal, vertical float64, thickness, angle, pitch float32, start, end int, yScale, floor float64) {
	random := worldrandom.NewLegacy(seed)
	branch := int(random.NextIntBound(int32(end/2))) + end/4
	gentle := random.NextIntBound(6) == 0
	angleVelocity, pitchVelocity := float32(0), float32(0)
	for step := start; step < end; step++ {
		radius := 1.5 + float64(mthSin(float32(math.Pi)*float32(step)/float32(end))*thickness)
		verticalRadius := radius * yScale
		cosPitch := mthCos(pitch)
		x += float64(mthCos(angle) * cosPitch)
		y += float64(mthSin(pitch))
		z += float64(mthSin(angle) * cosPitch)
		if gentle {
			pitch *= 0.92
		} else {
			pitch *= 0.7
		}
		pitch += pitchVelocity * 0.1
		angle += angleVelocity * 0.1
		pitchVelocity *= 0.9
		angleVelocity *= 0.75
		pitchVelocity += (random.NextFloat() - random.NextFloat()) * random.NextFloat() * 2
		angleVelocity += (random.NextFloat() - random.NextFloat()) * random.NextFloat() * 4
		if step == branch && thickness > 1 {
			c.carveTunnel(system, chunk, targetX, targetZ, aquifer, mask, random.NextLong(), x, y, z, horizontal, vertical, random.NextFloat()*0.5+0.5, angle-float32(math.Pi/2), pitch/3, step, end, 1, floor)
			c.carveTunnel(system, chunk, targetX, targetZ, aquifer, mask, random.NextLong(), x, y, z, horizontal, vertical, random.NextFloat()*0.5+0.5, angle+float32(math.Pi/2), pitch/3, step, end, 1, floor)
			return
		}
		if random.NextIntBound(4) != 0 {
			if !carverCanReach(targetX, targetZ, x, z, step, end, thickness) {
				return
			}
			c.carveEllipsoid(system, chunk, targetX, targetZ, aquifer, mask, x, y, z, radius*horizontal, verticalRadius*vertical, floor)
		}
	}
}

func (c *configuredCarver) carveEllipsoid(system *carverSystem, chunk *section.Chunk, targetX, targetZ int32, aquifer *chunkAquifer, mask []bool, centerX, centerY, centerZ, horizontalRadius, verticalRadius, floor float64) bool {
	middleX, middleZ := float64(int(targetX)*16+8), float64(int(targetZ)*16+8)
	if math.Abs(centerX-middleX) > 16+horizontalRadius*2 || math.Abs(centerZ-middleZ) > 16+horizontalRadius*2 {
		return false
	}
	baseX, baseZ := int(targetX)*16, int(targetZ)*16
	minX := max(int(math.Floor(centerX-horizontalRadius))-baseX-1, 0)
	maxX := min(int(math.Floor(centerX+horizontalRadius))-baseX, 15)
	minY := max(int(math.Floor(centerY-verticalRadius))-1, system.minY+1)
	maxY := min(int(math.Floor(centerY+verticalRadius))+1, system.minY+system.height-8)
	minZ := max(int(math.Floor(centerZ-horizontalRadius))-baseZ-1, 0)
	maxZ := min(int(math.Floor(centerZ+horizontalRadius))-baseZ, 15)
	changed := false
	for localX := minX; localX <= maxX; localX++ {
		xNorm := (float64(baseX+localX) + 0.5 - centerX) / horizontalRadius
		for localZ := minZ; localZ <= maxZ; localZ++ {
			zNorm := (float64(baseZ+localZ) + 0.5 - centerZ) / horizontalRadius
			if xNorm*xNorm+zNorm*zNorm >= 1 {
				continue
			}
			for y := maxY; y > minY; y-- {
				yNorm := (float64(y) - 0.5 - centerY) / verticalRadius
				if yNorm <= floor || xNorm*xNorm+yNorm*yNorm+zNorm*zNorm >= 1 {
					continue
				}
				index := ((localX*16)+localZ)*system.height + (y - system.minY)
				if mask[index] {
					continue
				}
				mask[index] = true
				if c.carveBlock(system, chunk, localX, y, localZ, baseX+localX, baseZ+localZ, aquifer) {
					changed = true
				}
			}
		}
	}
	return changed
}

func (c *configuredCarver) carveBlock(system *carverSystem, chunk *section.Chunk, localX, y, localZ, x, z int, aquifer *chunkAquifer) bool {
	state := chunk.GetBlock(localX, y, localZ)
	if _, ok := c.replaceable[block.NameFromState(state)]; !ok {
		return false
	}
	wasSurface := block.NameFromState(state) == "minecraft:grass_block" || block.NameFromState(state) == "minecraft:mycelium"
	if c.kind == "minecraft:nether_cave" {
		if y <= system.minY+31 {
			chunk.SetBlock(localX, y, localZ, blockfluid.SourceStateID(blockfluid.FluidTypeLava))
		} else {
			chunk.SetBlock(localX, y, localZ, mustDefaultBlock("minecraft:cave_air"))
		}
		return true
	}
	var replacement int32
	if y <= c.lavaLevel.resolve(system.minY, system.height) {
		replacement = blockfluid.SourceStateID(blockfluid.FluidTypeLava)
	} else {
		replacement = aquifer.substance(density.Position{X: x, Y: y, Z: z}, 0)
		if replacement < 0 {
			return false
		}
	}
	chunk.SetBlock(localX, y, localZ, replacement)
	if wasSurface && y-1 >= chunk.MinY() && block.NameFromState(chunk.GetBlock(localX, y-1, localZ)) == "minecraft:dirt" {
		if top, ok := system.surface.topMaterial(chunk, localX, y-1, localZ, x, z, blockfluid.IsFluid(replacement)); ok {
			chunk.SetBlock(localX, y-1, localZ, top)
		}
	}
	return true
}

func carverCanReach(chunkX, chunkZ int32, x, z float64, step, end int, thickness float32) bool {
	dx, dz := x-float64(int(chunkX)*16+8), z-float64(int(chunkZ)*16+8)
	remaining := float64(end - step)
	radius := float64(thickness + 18)
	return dx*dx+dz*dz-remaining*remaining <= radius*radius
}

var mthSinTable = func() [65536]float32 {
	var table [65536]float32
	for i := range table {
		table[i] = float32(math.Sin(float64(i) * math.Pi * 2 / 65536))
	}
	return table
}()

func mthSin(value float32) float32 { return mthSinTable[int(value*10430.378)&65535] }
func mthCos(value float32) float32 { return mthSinTable[int(value*10430.378+16384)&65535] }

func absFloat32(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}
