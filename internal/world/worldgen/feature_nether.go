package worldgen

import (
	"encoding/json"
	"fmt"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type deltaFeature struct {
	contents int32
	rim      int32
	rimSize  intProvider
	size     intProvider
}

func (f *deltaFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	spawnRim := random.NextDouble() < 0.9
	rimX, rimZ := 0, 0
	if spawnRim {
		rimX = f.rimSize.sample(random)
		rimZ = f.rimSize.sample(random)
	}
	placeRim := spawnRim && rimX != 0 && rimZ != 0
	sizeX, sizeZ := f.size.sample(random), f.size.sample(random)
	maximum := maxIntValue(sizeX, sizeZ)
	placed := false
	for x := origin.x - sizeX; x <= origin.x+sizeX; x++ {
		for z := origin.z - sizeZ; z <= origin.z+sizeZ; z++ {
			position := blockPosition{x: x, y: origin.y, z: z}
			if absInt(x-origin.x)+absInt(z-origin.z) > maximum || !f.clear(chunk, position) {
				continue
			}
			if placeRim {
				setChunkBlock(chunk, position, f.rim)
				placed = true
			}
			contents := blockPosition{x: x + rimX, y: origin.y, z: z + rimZ}
			if f.clear(chunk, contents) {
				setChunkBlock(chunk, contents, f.contents)
				placed = true
			}
		}
	}
	return placed
}

func (f *deltaFeature) clear(chunk *section.Chunk, position blockPosition) bool {
	state := getChunkBlock(chunk, position)
	if block.NameFromState(state) == block.NameFromState(f.contents) {
		return false
	}
	switch block.NameFromState(state) {
	case "minecraft:bedrock", "minecraft:nether_bricks", "minecraft:nether_brick_fence", "minecraft:nether_brick_stairs", "minecraft:nether_wart", "minecraft:chest", "minecraft:spawner":
		return false
	}
	for index, offset := range adjacentDirections {
		air := block.IsAir(getChunkBlock(chunk, blockPosition{x: position.x + offset[0], y: position.y + offset[1], z: position.z + offset[2]}))
		up := index == 1
		if air != up {
			return false
		}
	}
	return true
}

type basaltPillarFeature struct{ basalt int32 }

func (f *basaltPillarFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	if !isAirAt(chunk, origin) || isAirAt(chunk, blockPosition{x: origin.x, y: origin.y + 1, z: origin.z}) {
		return false
	}
	position := origin
	active := [4]bool{true, true, true, true}
	horizontal := adjacentDirections[2:]
	for isAirAt(chunk, position) {
		if position.y < chunk.MinY() || position.y > chunk.MaxY() {
			return true
		}
		setChunkBlock(chunk, position, f.basalt)
		for index, offset := range horizontal {
			if !active[index] {
				continue
			}
			active[index] = random.NextIntBound(10) != 0
			if active[index] {
				setChunkBlock(chunk, blockPosition{x: position.x + offset[0], y: position.y, z: position.z + offset[2]}, f.basalt)
			}
		}
		position.y--
	}

	position.y++
	for _, offset := range horizontal {
		if random.NextBoolean() {
			setChunkBlock(chunk, blockPosition{x: position.x + offset[0], y: position.y, z: position.z + offset[2]}, f.basalt)
		}
	}
	position.y--
	for dx := -3; dx < 4; dx++ {
		for dz := -3; dz < 4; dz++ {
			if random.NextIntBound(10) >= int32(10-absInt(dx)*absInt(dz)) {
				continue
			}
			candidate := blockPosition{x: position.x + dx, y: position.y, z: position.z + dz}
			for remaining := 3; remaining > 0 && isAirAt(chunk, blockPosition{x: candidate.x, y: candidate.y - 1, z: candidate.z}); remaining-- {
				candidate.y--
			}
			if !isAirAt(chunk, blockPosition{x: candidate.x, y: candidate.y - 1, z: candidate.z}) {
				setChunkBlock(chunk, candidate, f.basalt)
			}
		}
	}
	return true
}

type basaltColumnsFeature struct {
	basalt        int32
	reach, height intProvider
}

type netherForestVegetationFeature struct {
	provider                  stateProvider
	spreadWidth, spreadHeight int
}

type twistingVinesFeature struct {
	spreadWidth, spreadHeight, maxHeight int
	head, plant                          int32
}

func (f *twistingVinesFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	if invalidTwistingVinesLocation(chunk, origin) {
		return false
	}
	for attempt := 0; attempt < f.spreadWidth*f.spreadWidth; attempt++ {
		position := blockPosition{
			x: origin.x - f.spreadWidth + int(random.NextIntBound(int32(f.spreadWidth*2+1))),
			y: origin.y - f.spreadHeight + int(random.NextIntBound(int32(f.spreadHeight*2+1))),
			z: origin.z - f.spreadWidth + int(random.NextIntBound(int32(f.spreadWidth*2+1))),
		}
		if !findFirstAirAboveGround(chunk, &position) || invalidTwistingVinesLocation(chunk, position) {
			continue
		}
		height := 1 + int(random.NextIntBound(int32(f.maxHeight)))
		if random.NextIntBound(6) == 0 {
			height *= 2
		}
		if random.NextIntBound(5) == 0 {
			height = 1
		}
		placeVinesColumn(chunk, random, position, height, 1, f.head, f.plant)
	}
	return true
}

type weepingVinesFeature struct {
	head, plant, wart int32
}

func (f *weepingVinesFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	if !isAirAt(chunk, origin) {
		return false
	}
	above := block.NameFromState(getChunkBlock(chunk, blockPosition{x: origin.x, y: origin.y + 1, z: origin.z}))
	if above != "minecraft:netherrack" && above != "minecraft:nether_wart_block" {
		return false
	}
	setChunkBlock(chunk, origin, f.wart)
	for attempt := 0; attempt < 200; attempt++ {
		position := blockPosition{
			x: origin.x + int(random.NextIntBound(6)) - int(random.NextIntBound(6)),
			y: origin.y + int(random.NextIntBound(2)) - int(random.NextIntBound(5)),
			z: origin.z + int(random.NextIntBound(6)) - int(random.NextIntBound(6)),
		}
		if !isAirAt(chunk, position) {
			continue
		}
		neighbors := 0
		for _, offset := range adjacentDirections {
			name := block.NameFromState(getChunkBlock(chunk, blockPosition{x: position.x + offset[0], y: position.y + offset[1], z: position.z + offset[2]}))
			if name == "minecraft:netherrack" || name == "minecraft:nether_wart_block" {
				neighbors++
			}
			if neighbors > 1 {
				break
			}
		}
		if neighbors == 1 {
			setChunkBlock(chunk, position, f.wart)
		}
	}
	for attempt := 0; attempt < 100; attempt++ {
		position := blockPosition{
			x: origin.x + int(random.NextIntBound(8)) - int(random.NextIntBound(8)),
			y: origin.y + int(random.NextIntBound(2)) - int(random.NextIntBound(7)),
			z: origin.z + int(random.NextIntBound(8)) - int(random.NextIntBound(8)),
		}
		if !isAirAt(chunk, position) {
			continue
		}
		above := block.NameFromState(getChunkBlock(chunk, blockPosition{x: position.x, y: position.y + 1, z: position.z}))
		if above != "minecraft:netherrack" && above != "minecraft:nether_wart_block" {
			continue
		}
		height := 1 + int(random.NextIntBound(8))
		if random.NextIntBound(6) == 0 {
			height *= 2
		}
		if random.NextIntBound(5) == 0 {
			height = 1
		}
		placeVinesColumn(chunk, random, position, height, -1, f.head, f.plant)
	}
	return true
}

func findFirstAirAboveGround(chunk *section.Chunk, position *blockPosition) bool {
	for {
		position.y--
		if position.y < chunk.MinY() || position.y > chunk.MaxY() {
			return false
		}
		if !isAirAt(chunk, *position) {
			position.y++
			return true
		}
	}
}

func invalidTwistingVinesLocation(chunk *section.Chunk, position blockPosition) bool {
	if !isAirAt(chunk, position) {
		return true
	}
	name := block.NameFromState(getChunkBlock(chunk, blockPosition{x: position.x, y: position.y - 1, z: position.z}))
	return name != "minecraft:netherrack" && name != "minecraft:warped_nylium" && name != "minecraft:warped_wart_block"
}

func placeVinesColumn(chunk *section.Chunk, random worldrandom.Source, position blockPosition, height, direction int, head, plant int32) {
	start, end := 1, height
	if direction < 0 {
		start, end = 0, height
	}
	for index := start; index <= end; index++ {
		if isAirAt(chunk, position) {
			next := blockPosition{x: position.x, y: position.y + direction, z: position.z}
			if index == end || !isAirAt(chunk, next) {
				age := 17 + int(random.NextIntBound(9))
				state, err := block.BlockStateFromID(head)
				if err == nil {
					if aged, ageErr := state.WithProperty("age", fmt.Sprintf("%d", age)); ageErr == nil {
						head = aged.ID()
					}
				}
				setChunkBlock(chunk, position, head)
				break
			}
			setChunkBlock(chunk, position, plant)
		}
		position.y += direction
	}
}

func (f *netherForestVegetationFeature) placeFeature(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	below := block.NameFromState(getChunkBlock(chunk, blockPosition{x: origin.x, y: origin.y - 1, z: origin.z}))
	if below != "minecraft:crimson_nylium" && below != "minecraft:warped_nylium" {
		return false
	}
	if origin.y < chunk.MinY()+1 || origin.y+1 > chunk.MaxY() {
		return false
	}
	placed := false
	for attempt := 0; attempt < f.spreadWidth*f.spreadWidth; attempt++ {
		position := blockPosition{
			x: origin.x + int(random.NextIntBound(int32(f.spreadWidth))) - int(random.NextIntBound(int32(f.spreadWidth))),
			y: origin.y + int(random.NextIntBound(int32(f.spreadHeight))) - int(random.NextIntBound(int32(f.spreadHeight))),
			z: origin.z + int(random.NextIntBound(int32(f.spreadWidth))) - int(random.NextIntBound(int32(f.spreadWidth))),
		}
		state := f.provider.state(system, chunk, random, position)
		if isAirAt(chunk, position) && position.y > chunk.MinY() && netherVegetationCanSurvive(chunk, position, state) {
			setChunkBlock(chunk, position, state)
			placed = true
		}
	}
	return placed
}

func netherVegetationCanSurvive(chunk *section.Chunk, position blockPosition, state int32) bool {
	below := block.NameFromState(getChunkBlock(chunk, blockPosition{x: position.x, y: position.y - 1, z: position.z}))
	switch below {
	case "minecraft:crimson_nylium", "minecraft:warped_nylium", "minecraft:soul_soil", "minecraft:farmland", "minecraft:dirt", "minecraft:grass_block", "minecraft:coarse_dirt", "minecraft:podzol", "minecraft:rooted_dirt":
		return true
	case "minecraft:mycelium":
		name := block.NameFromState(state)
		return name == "minecraft:crimson_fungus" || name == "minecraft:warped_fungus"
	default:
		return false
	}
}

func (f *basaltColumnsFeature) placeFeature(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	if !canPlaceBasaltAt(chunk, system.seaLevel, origin) {
		return false
	}
	height := f.height.sample(random)
	clustered := random.NextFloat() < 0.9
	reach := minIntValue(height, 8)
	attempts := 15
	if clustered {
		reach = minIntValue(height, 5)
		attempts = 50
	}
	placed := false
	for attempt := 0; attempt < attempts; attempt++ {
		candidate := blockPosition{
			x: origin.x - reach + int(random.NextIntBound(int32(reach*2+1))),
			y: origin.y + int(random.NextIntBound(1)),
			z: origin.z - reach + int(random.NextIntBound(int32(reach*2+1))),
		}
		columnHeight := height - manhattanDistance(candidate, origin)
		if columnHeight >= 0 {
			placed = f.placeColumn(chunk, system.seaLevel, candidate, columnHeight, f.reach.sample(random)) || placed
		}
	}
	return placed
}

func (f *basaltColumnsFeature) placeColumn(chunk *section.Chunk, seaLevel int, origin blockPosition, height, reach int) bool {
	placed := false
	for x := origin.x - reach; x <= origin.x+reach; x++ {
		for z := origin.z - reach; z <= origin.z+reach; z++ {
			candidate := blockPosition{x: x, y: origin.y, z: z}
			distance := manhattanDistance(candidate, origin)
			var start blockPosition
			var found bool
			if isAirOrLavaOcean(chunk, seaLevel, candidate) {
				start, found = findBasaltSurface(chunk, seaLevel, candidate, distance)
			} else {
				start, found = findBasaltAir(chunk, candidate, distance)
			}
			if !found {
				continue
			}
			for remaining := height - distance/2; remaining >= 0; remaining-- {
				if isAirOrLavaOcean(chunk, seaLevel, start) {
					setChunkBlock(chunk, start, f.basalt)
					placed = true
				} else if block.NameFromState(getChunkBlock(chunk, start)) != "minecraft:basalt" {
					break
				}
				start.y++
			}
		}
	}
	return placed
}

func findBasaltSurface(chunk *section.Chunk, seaLevel int, position blockPosition, remaining int) (blockPosition, bool) {
	for position.y > chunk.MinY()+1 && remaining > 0 {
		remaining--
		if canPlaceBasaltAt(chunk, seaLevel, position) {
			return position, true
		}
		position.y--
	}
	return blockPosition{}, false
}

func findBasaltAir(chunk *section.Chunk, position blockPosition, remaining int) (blockPosition, bool) {
	for position.y <= chunk.MaxY() && remaining > 0 {
		remaining--
		state := getChunkBlock(chunk, position)
		if cannotPlaceBasaltOn(state) {
			return blockPosition{}, false
		}
		if block.IsAir(state) {
			return position, true
		}
		position.y++
	}
	return blockPosition{}, false
}

func canPlaceBasaltAt(chunk *section.Chunk, seaLevel int, position blockPosition) bool {
	if !isAirOrLavaOcean(chunk, seaLevel, position) {
		return false
	}
	below := getChunkBlock(chunk, blockPosition{x: position.x, y: position.y - 1, z: position.z})
	return !block.IsAir(below) && !cannotPlaceBasaltOn(below)
}

func isAirOrLavaOcean(chunk *section.Chunk, seaLevel int, position blockPosition) bool {
	state := getChunkBlock(chunk, position)
	return block.IsAir(state) || block.NameFromState(state) == "minecraft:lava" && position.y <= seaLevel
}

func cannotPlaceBasaltOn(state int32) bool {
	switch block.NameFromState(state) {
	case "minecraft:lava", "minecraft:bedrock", "minecraft:magma_block", "minecraft:soul_sand", "minecraft:nether_bricks", "minecraft:nether_brick_fence", "minecraft:nether_brick_stairs", "minecraft:nether_wart", "minecraft:chest", "minecraft:spawner":
		return true
	default:
		return false
	}
}

func manhattanDistance(a, b blockPosition) int {
	return absInt(a.x-b.x) + absInt(a.y-b.y) + absInt(a.z-b.z)
}

func decodeNetherFeature(kind string, raw json.RawMessage) (configuredFeature, bool, error) {
	state := func(value BlockState) (int32, error) {
		resolved, err := block.BlockStateFromProperties(value.Name, value.Properties)
		if err != nil {
			return 0, err
		}
		return resolved.ID(), nil
	}
	switch kind {
	case "minecraft:delta_feature":
		var value struct {
			Config struct {
				Contents BlockState      `json:"contents"`
				Rim      BlockState      `json:"rim"`
				RimSize  json.RawMessage `json:"rim_size"`
				Size     json.RawMessage `json:"size"`
			} `json:"config"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, true, err
		}
		contents, err := state(value.Config.Contents)
		if err != nil {
			return nil, true, err
		}
		rim, err := state(value.Config.Rim)
		if err != nil {
			return nil, true, err
		}
		rimSize, err := decodeIntProviderRange(value.Config.RimSize, 0, 16)
		if err != nil {
			return nil, true, fmt.Errorf("rim size: %w", err)
		}
		size, err := decodeIntProviderRange(value.Config.Size, 0, 16)
		if err != nil {
			return nil, true, fmt.Errorf("size: %w", err)
		}
		return &deltaFeature{contents: contents, rim: rim, rimSize: rimSize, size: size}, true, nil
	case "minecraft:basalt_pillar":
		basalt := block.DefaultStateID("minecraft:basalt")
		if basalt < 0 {
			return nil, true, fmt.Errorf("unknown block minecraft:basalt")
		}
		return &basaltPillarFeature{basalt: basalt}, true, nil
	case "minecraft:basalt_columns":
		var value struct {
			Config struct {
				Reach  json.RawMessage `json:"reach"`
				Height json.RawMessage `json:"height"`
			} `json:"config"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, true, err
		}
		reach, err := decodeIntProviderRange(value.Config.Reach, 0, 3)
		if err != nil {
			return nil, true, fmt.Errorf("reach: %w", err)
		}
		height, err := decodeIntProviderRange(value.Config.Height, 1, 10)
		if err != nil {
			return nil, true, fmt.Errorf("height: %w", err)
		}
		basalt := block.DefaultStateID("minecraft:basalt")
		if basalt < 0 {
			return nil, true, fmt.Errorf("unknown block minecraft:basalt")
		}
		return &basaltColumnsFeature{basalt: basalt, reach: reach, height: height}, true, nil
	case "minecraft:nether_forest_vegetation":
		var value struct {
			Config struct {
				Provider     json.RawMessage `json:"state_provider"`
				SpreadWidth  int             `json:"spread_width"`
				SpreadHeight int             `json:"spread_height"`
			} `json:"config"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, true, err
		}
		if value.Config.SpreadWidth <= 0 || value.Config.SpreadHeight <= 0 {
			return nil, true, fmt.Errorf("spread dimensions must be positive")
		}
		provider, err := decodeStateProvider(value.Config.Provider)
		if err != nil {
			return nil, true, fmt.Errorf("state provider: %w", err)
		}
		return &netherForestVegetationFeature{
			provider: provider, spreadWidth: value.Config.SpreadWidth, spreadHeight: value.Config.SpreadHeight,
		}, true, nil
	case "minecraft:twisting_vines":
		var value struct {
			Config struct {
				SpreadWidth  int `json:"spread_width"`
				SpreadHeight int `json:"spread_height"`
				MaxHeight    int `json:"max_height"`
			} `json:"config"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, true, err
		}
		if value.Config.SpreadWidth <= 0 || value.Config.SpreadHeight <= 0 || value.Config.MaxHeight <= 0 {
			return nil, true, fmt.Errorf("twisting vines dimensions must be positive")
		}
		return &twistingVinesFeature{
			spreadWidth: value.Config.SpreadWidth, spreadHeight: value.Config.SpreadHeight, maxHeight: value.Config.MaxHeight,
			head: block.DefaultStateID("minecraft:twisting_vines"), plant: block.DefaultStateID("minecraft:twisting_vines_plant"),
		}, true, nil
	case "minecraft:weeping_vines":
		return &weepingVinesFeature{
			head: block.DefaultStateID("minecraft:weeping_vines"), plant: block.DefaultStateID("minecraft:weeping_vines_plant"), wart: block.DefaultStateID("minecraft:nether_wart_block"),
		}, true, nil
	default:
		return nil, false, nil
	}
}

func minIntValue(a, b int) int {
	if a < b {
		return a
	}
	return b
}
