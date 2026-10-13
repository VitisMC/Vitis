package worldgen

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type vegetationPatchFeature struct {
	depth, radius                                  intProvider
	ground                                         stateProvider
	vegetation                                     placedFeatureReference
	replaceable                                    map[int32]struct{}
	ceiling                                        bool
	waterlogged                                    bool
	verticalRange                                  int
	edgeChance, vegetationChance, extraDepthChance float32
}

// decodeVegetationPatchFeature compiles a cave floor or ceiling patch.
func decodeVegetationPatchFeature(snapshot *datapack.Snapshot, raw json.RawMessage) (configuredFeature, bool, error) {
	var data struct {
		Type   string `json:"type"`
		Config struct {
			Depth            json.RawMessage `json:"depth"`
			Radius           json.RawMessage `json:"xz_radius"`
			Ground           json.RawMessage `json:"ground_state"`
			Vegetation       json.RawMessage `json:"vegetation_feature"`
			Replaceable      string          `json:"replaceable"`
			Surface          string          `json:"surface"`
			VerticalRange    int             `json:"vertical_range"`
			EdgeChance       float32         `json:"extra_edge_column_chance"`
			VegetationChance float32         `json:"vegetation_chance"`
			ExtraDepthChance float32         `json:"extra_bottom_block_chance"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, true, err
	}
	c := data.Config
	if (c.Surface != "floor" && c.Surface != "ceiling") || c.VerticalRange < 1 || c.VerticalRange > 64 || c.EdgeChance < 0 || c.EdgeChance > 1 || c.VegetationChance < 0 || c.VegetationChance > 1 || c.ExtraDepthChance < 0 || c.ExtraDepthChance > 1 || !strings.HasPrefix(c.Replaceable, "#") {
		return nil, true, fmt.Errorf("invalid vegetation patch configuration")
	}
	f := &vegetationPatchFeature{ceiling: c.Surface == "ceiling", waterlogged: data.Type == "minecraft:waterlogged_vegetation_patch", verticalRange: c.VerticalRange, edgeChance: c.EdgeChance, vegetationChance: c.VegetationChance, extraDepthChance: c.ExtraDepthChance, replaceable: make(map[int32]struct{})}
	var err error
	if f.depth, err = decodeIntProviderRange(c.Depth, 1, 64); err != nil {
		return nil, true, err
	}
	if f.radius, err = decodeIntProviderRange(c.Radius, 0, 16); err != nil {
		return nil, true, err
	}
	if f.ground, err = decodeStateProvider(c.Ground); err != nil {
		return nil, true, err
	}
	if f.vegetation, err = decodePlacedFeatureReference(snapshot, c.Vegetation); err != nil {
		return nil, true, err
	}
	names, err := snapshot.ResolveTag("block", strings.TrimPrefix(c.Replaceable, "#"))
	if err != nil {
		return nil, true, err
	}
	for _, name := range names {
		info := block.InfoByName(name)
		if info == nil {
			return nil, true, fmt.Errorf("unknown patch block %s", name)
		}
		for id := info.MinStateID; id <= info.MaxStateID; id++ {
			f.replaceable[id] = struct{}{}
		}
	}
	return f, true, nil
}

// placeFeature replaces a bounded surface patch and distributes vegetation across chunk borders.
func (f *vegetationPatchFeature) placeFeature(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	xRadius, zRadius := f.radius.sample(random)+1, f.radius.sample(random)+1
	direction := -1
	if f.ceiling {
		direction = 1
	}
	grounds := make([]blockPosition, 0, (2*xRadius+1)*(2*zRadius+1))
	for dx := -xRadius; dx <= xRadius; dx++ {
		for dz := -zRadius; dz <= zRadius; dz++ {
			xEdge, zEdge := dx == -xRadius || dx == xRadius, dz == -zRadius || dz == zRadius
			if xEdge && zEdge || (xEdge || zEdge) && (f.edgeChance == 0 || random.NextFloat() > f.edgeChance) {
				continue
			}
			p := blockPosition{origin.x + dx, origin.y, origin.z + dz}
			if !chunk.FeatureWriteAllowed(p.x, p.y, p.z) {
				continue
			}
			for step := 0; step < f.verticalRange && p.y >= chunk.MinY() && p.y <= chunk.MaxY() && block.IsAir(getChunkBlock(chunk, p)); step++ {
				p.y += direction
			}
			for step := 0; step < f.verticalRange && p.y >= chunk.MinY() && p.y <= chunk.MaxY() && !block.IsAir(getChunkBlock(chunk, p)); step++ {
				p.y -= direction
			}
			ground := blockPosition{p.x, p.y + direction, p.z}
			if p.y < chunk.MinY() || p.y > chunk.MaxY() || ground.y < chunk.MinY() || ground.y > chunk.MaxY() || !block.IsAir(getChunkBlock(chunk, p)) || !block.IsSolid(getChunkBlock(chunk, ground)) {
				continue
			}
			depth := f.depth.sample(random)
			if f.extraDepthChance > 0 && random.NextFloat() < f.extraDepthChance {
				depth++
			}
			placed := false
			for i := 0; i < depth; i++ {
				state := getChunkBlock(chunk, ground)
				newState := f.ground.state(system, chunk, random, ground)
				if block.NameFromState(state) != block.NameFromState(newState) {
					if _, ok := f.replaceable[state]; !ok {
						break
					}
					setChunkBlock(chunk, ground, newState)
					ground.y += direction
				}
				placed = true
			}
			if placed {
				grounds = append(grounds, blockPosition{p.x, p.y + direction, p.z})
			}
		}
	}
	if f.waterlogged {
		sealed := grounds[:0]
		for _, ground := range grounds {
			if !patchGroundExposed(chunk, ground) {
				sealed = append(sealed, ground)
			}
		}
		grounds = sealed
		water := block.DefaultStateID("minecraft:water")
		for _, ground := range grounds {
			setChunkBlock(chunk, ground, water)
		}
	}
	for _, ground := range grounds {
		if f.vegetationChance > 0 && random.NextFloat() < f.vegetationChance {
			position := blockPosition{ground.x, ground.y - direction, ground.z}
			if f.waterlogged {
				position.y--
			}
			if f.vegetation.place(system, chunk, random, position) && f.waterlogged {
				if state, err := block.BlockStateFromID(getChunkBlock(chunk, ground)); err == nil {
					if value, present := state.Property("waterlogged"); present && value == "false" {
						if flooded, err := state.WithProperty("waterlogged", "true"); err == nil {
							setChunkBlock(chunk, ground, flooded.ID())
						}
					}
				}
			}
		}
	}
	return len(grounds) > 0
}

// patchGroundExposed rejects cave pools with open sides or an unsupported floor.
func patchGroundExposed(chunk *section.Chunk, position blockPosition) bool {
	for _, offset := range [...][3]int{{0, 0, -1}, {1, 0, 0}, {0, 0, 1}, {-1, 0, 0}, {0, -1, 0}} {
		if !block.IsSolid(getChunkBlock(chunk, blockPosition{position.x + offset[0], position.y + offset[1], position.z + offset[2]})) {
			return true
		}
	}
	return false
}
