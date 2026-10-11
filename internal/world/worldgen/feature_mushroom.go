package worldgen

import (
	"encoding/json"
	"fmt"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type hugeMushroomFeature struct {
	red       bool
	radius    int
	cap, stem stateProvider
}

// decodeHugeMushroomFeature compiles the cap and stem providers.
func decodeHugeMushroomFeature(kind string, raw json.RawMessage) (configuredFeature, bool, error) {
	var data struct {
		Config struct {
			Cap    json.RawMessage `json:"cap_provider"`
			Stem   json.RawMessage `json:"stem_provider"`
			Radius int             `json:"foliage_radius"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, true, err
	}
	if data.Config.Radius < 1 || data.Config.Radius > 8 {
		return nil, true, fmt.Errorf("invalid mushroom foliage radius")
	}
	cap, err := decodeStateProvider(data.Config.Cap)
	if err != nil {
		return nil, true, err
	}
	stem, err := decodeStateProvider(data.Config.Stem)
	if err != nil {
		return nil, true, err
	}
	return &hugeMushroomFeature{red: kind == "minecraft:huge_red_mushroom", radius: data.Config.Radius, cap: cap, stem: stem}, true, nil
}

// placeFeature grows a huge mushroom after checking its full cap clearance.
func (f *hugeMushroomFeature) placeFeature(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	height := 4 + int(random.NextIntBound(3))
	if random.NextIntBound(12) == 0 {
		height *= 2
	}
	if origin.y < chunk.MinY()+1 || origin.y+height+1 > chunk.MaxY() {
		return false
	}
	ground := block.NameFromState(getTreeBlock(chunk, blockPosition{origin.x, origin.y - 1, origin.z}))
	if ground != "minecraft:dirt" && ground != "minecraft:grass_block" && ground != "minecraft:mycelium" && ground != "minecraft:podzol" && ground != "minecraft:coarse_dirt" && ground != "minecraft:moss_block" && ground != "minecraft:crimson_nylium" && ground != "minecraft:warped_nylium" {
		return false
	}
	for y := 0; y <= height; y++ {
		radius := 0
		if f.red && y >= height-3 || !f.red && y > 3 {
			radius = f.radius
		}
		for dx := -radius; dx <= radius; dx++ {
			for dz := -radius; dz <= radius; dz++ {
				state := getTreeBlock(chunk, blockPosition{origin.x + dx, origin.y + y, origin.z + dz})
				if !block.IsAir(state) && !isLeafState(state) {
					return false
				}
			}
		}
	}
	placeCap := func(dx, dy, dz int, west, east, north, south, up bool) {
		p := blockPosition{origin.x + dx, origin.y + dy, origin.z + dz}
		if block.IsSolid(getTreeBlock(chunk, p)) {
			return
		}
		state := f.cap.state(system, chunk, random, origin)
		if value, err := block.BlockStateFromID(state); err == nil {
			for property, enabled := range map[string]bool{"west": west, "east": east, "north": north, "south": south, "up": up} {
				if next, err := value.WithProperty(property, fmt.Sprint(enabled)); err == nil {
					value = next
				}
			}
			state = value.ID()
		}
		setTreeBlock(chunk, p, state)
	}
	if f.red {
		for y := height - 3; y <= height; y++ {
			r := f.radius
			if y == height {
				r--
			}
			for dx := -r; dx <= r; dx++ {
				for dz := -r; dz <= r; dz++ {
					xEdge, zEdge := dx == -r || dx == r, dz == -r || dz == r
					if y == height || xEdge != zEdge {
						placeCap(dx, y, dz, dx < -(f.radius-2), dx > f.radius-2, dz < -(f.radius-2), dz > f.radius-2, y >= height-1)
					}
				}
			}
		}
	} else {
		r := f.radius
		for dx := -r; dx <= r; dx++ {
			for dz := -r; dz <= r; dz++ {
				if (dx == -r || dx == r) && (dz == -r || dz == r) {
					continue
				}
				placeCap(dx, height, dz, dx == -r || (dz == -r || dz == r) && dx == 1-r, dx == r || (dz == -r || dz == r) && dx == r-1, dz == -r || (dx == -r || dx == r) && dz == 1-r, dz == r || (dx == -r || dx == r) && dz == r-1, true)
			}
		}
	}
	for y := 0; y < height; y++ {
		p := blockPosition{origin.x, origin.y + y, origin.z}
		if !block.IsSolid(getTreeBlock(chunk, p)) {
			setTreeBlock(chunk, p, f.stem.state(system, chunk, random, origin))
		}
	}
	return true
}

// isLeafState reports whether a block can be replaced by a mushroom cap.
func isLeafState(state int32) bool {
	name := block.NameFromState(state)
	return len(name) >= 7 && name[len(name)-7:] == "_leaves"
}
