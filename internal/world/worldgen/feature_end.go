package worldgen

import (
	"encoding/json"
	"fmt"

	"github.com/vitismc/vitis/internal/block"
	blockentity "github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type endGatewayFeature struct {
	hasExit bool
	exit    [3]int32
	exact   bool
	gateway int32
	bedrock int32
}

type chorusPlantFeature struct {
	plant  int32
	flower int32
}

var chorusHorizontalDirections = [...]blockPosition{
	{x: 0, z: -1},
	{x: 1, z: 0},
	{x: 0, z: 1},
	{x: -1, z: 0},
}

// placeFeature generates a chorus plant rooted on end stone.
func (f *chorusPlantFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	below := blockPosition{x: origin.x, y: origin.y - 1, z: origin.z}
	if !isAirAt(chunk, origin) || block.NameFromState(getChunkBlock(chunk, below)) != "minecraft:end_stone" {
		return false
	}
	f.setPlant(chunk, origin)
	f.grow(chunk, random, origin, origin, 8, 0)
	return true
}

// grow recursively creates the vertical trunk and horizontal branches.
func (f *chorusPlantFeature) grow(chunk *section.Chunk, random worldrandom.Source, origin, root blockPosition, radius, depth int) {
	height := int(random.NextIntBound(4)) + 1
	if depth == 0 {
		height++
	}
	for offset := 1; offset <= height; offset++ {
		position := blockPosition{x: origin.x, y: origin.y + offset, z: origin.z}
		if !chorusNeighborsEmpty(chunk, position, nil) {
			return
		}
		f.setPlant(chunk, position)
		f.setPlant(chunk, blockPosition{x: position.x, y: position.y - 1, z: position.z})
	}
	branched := false
	if depth < 4 {
		branches := int(random.NextIntBound(4))
		if depth == 0 {
			branches++
		}
		for range branches {
			direction := chorusHorizontalDirections[random.NextIntBound(4)]
			branch := blockPosition{x: origin.x + direction.x, y: origin.y + height, z: origin.z + direction.z}
			below := blockPosition{x: branch.x, y: branch.y - 1, z: branch.z}
			excluded := blockPosition{x: -direction.x, z: -direction.z}
			if absInt(branch.x-root.x) >= radius || absInt(branch.z-root.z) >= radius || !isAirAt(chunk, branch) || !isAirAt(chunk, below) || !chorusNeighborsEmpty(chunk, branch, &excluded) {
				continue
			}
			branched = true
			f.setPlant(chunk, branch)
			f.setPlant(chunk, blockPosition{x: branch.x + excluded.x, y: branch.y, z: branch.z + excluded.z})
			f.grow(chunk, random, branch, root, radius, depth+1)
		}
	}
	if !branched {
		setChunkBlock(chunk, blockPosition{x: origin.x, y: origin.y + height, z: origin.z}, f.flower)
	}
}

// chorusNeighborsEmpty checks horizontal clearance around a growth position.
func chorusNeighborsEmpty(chunk *section.Chunk, position blockPosition, excluded *blockPosition) bool {
	for _, direction := range chorusHorizontalDirections {
		if excluded != nil && direction.x == excluded.x && direction.z == excluded.z {
			continue
		}
		neighbor := blockPosition{x: position.x + direction.x, y: position.y, z: position.z + direction.z}
		if !isAirAt(chunk, neighbor) {
			return false
		}
	}
	return true
}

// setPlant places a chorus plant state with current neighbor connections.
func (f *chorusPlantFeature) setPlant(chunk *section.Chunk, position blockPosition) {
	state, err := block.BlockStateFromID(f.plant)
	if err != nil {
		return
	}
	directions := [...]struct {
		name            string
		offset          blockPosition
		includeEndStone bool
	}{
		{name: "down", offset: blockPosition{y: -1}, includeEndStone: true},
		{name: "up", offset: blockPosition{y: 1}},
		{name: "north", offset: blockPosition{z: -1}},
		{name: "east", offset: blockPosition{x: 1}},
		{name: "south", offset: blockPosition{z: 1}},
		{name: "west", offset: blockPosition{x: -1}},
	}
	for _, direction := range directions {
		neighbor := getChunkBlock(chunk, blockPosition{x: position.x + direction.offset.x, y: position.y + direction.offset.y, z: position.z + direction.offset.z})
		name := block.NameFromState(neighbor)
		connected := name == "minecraft:chorus_plant" || name == "minecraft:chorus_flower" || direction.includeEndStone && name == "minecraft:end_stone"
		state, err = state.WithProperty(direction.name, fmt.Sprintf("%t", connected))
		if err != nil {
			return
		}
	}
	setChunkBlock(chunk, position, state.ID())
}

func (f *endGatewayFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, _ worldrandom.Source, origin blockPosition) bool {
	for x := origin.x - 1; x <= origin.x+1; x++ {
		for y := origin.y - 2; y <= origin.y+2; y++ {
			for z := origin.z - 1; z <= origin.z+1; z++ {
				centerX, centerY, centerZ := x == origin.x, y == origin.y, z == origin.z
				verticalEnd := absInt(y-origin.y) == 2
				state := int32(0)
				switch {
				case centerX && centerY && centerZ:
					state = f.gateway
				case centerY:
				case verticalEnd && centerX && centerZ:
					state = f.bedrock
				case (centerX || centerZ) && !verticalEnd:
					state = f.bedrock
				}
				setChunkBlock(chunk, blockPosition{x: x, y: y, z: z}, state)
			}
		}
	}
	root := nbt.NewCompound().
		PutString("id", "minecraft:end_gateway").
		PutInt("x", int32(origin.x)).
		PutInt("y", int32(origin.y)).
		PutInt("z", int32(origin.z)).
		PutLong("Age", 0)
	if f.hasExit {
		root.PutIntArray("exit_portal", []int32{f.exit[0], f.exit[1], f.exit[2]})
	}
	if f.exact {
		root.PutBool("ExactTeleport", true)
	}
	chunk.AddBlockEntity(blockentity.NewRawBlockEntity(root))
	return true
}

func decodeEndFeature(kind string, raw json.RawMessage) (configuredFeature, bool, error) {
	if kind == "minecraft:chorus_plant" {
		flower, err := block.BlockStateFromProperties("minecraft:chorus_flower", map[string]string{"age": "5"})
		if err != nil {
			return nil, true, err
		}
		feature := &chorusPlantFeature{plant: block.DefaultStateID("minecraft:chorus_plant"), flower: flower.ID()}
		if feature.plant < 0 {
			return nil, true, fmt.Errorf("chorus plant block is unavailable")
		}
		return feature, true, nil
	}
	if kind != "minecraft:end_gateway" {
		return nil, false, nil
	}
	var value struct {
		Config struct {
			Exit  []int32 `json:"exit"`
			Exact bool    `json:"exact"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, true, err
	}
	if len(value.Config.Exit) != 0 && len(value.Config.Exit) != 3 {
		return nil, true, fmt.Errorf("end gateway exit must contain three coordinates")
	}
	feature := &endGatewayFeature{
		hasExit: len(value.Config.Exit) == 3,
		exact:   value.Config.Exact,
		gateway: block.DefaultStateID("minecraft:end_gateway"),
		bedrock: block.DefaultStateID("minecraft:bedrock"),
	}
	if feature.gateway < 0 || feature.bedrock < 0 {
		return nil, true, fmt.Errorf("end gateway blocks are unavailable")
	}
	if feature.hasExit {
		copy(feature.exit[:], value.Config.Exit)
	}
	return feature, true, nil
}
