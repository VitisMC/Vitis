package worldgen

import (
	"encoding/json"
	"fmt"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// fossilFeature owns paired immutable skeleton/ore templates and ordered processors.
type fossilFeature struct {
	base, overlay                     []*structureTemplate
	baseProcessors, overlayProcessors []templateProcessor
	maxEmptyCorners                   int
}

// decodeFossilFeature validates paired templates and their snapshot-specific processor lists.
func decodeFossilFeature(snapshot *datapack.Snapshot, store *structureTemplates, raw json.RawMessage) (*fossilFeature, error) {
	var value struct {
		Config struct {
			Base              []string        `json:"fossil_structures"`
			Overlay           []string        `json:"overlay_structures"`
			BaseProcessors    json.RawMessage `json:"fossil_processors"`
			OverlayProcessors json.RawMessage `json:"overlay_processors"`
			MaxEmptyCorners   *int            `json:"max_empty_corners_allowed"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	config := value.Config
	if len(config.Base) == 0 || len(config.Base) != len(config.Overlay) || len(config.Base) > 256 || config.MaxEmptyCorners == nil || *config.MaxEmptyCorners < 0 || *config.MaxEmptyCorners > 7 {
		return nil, fmt.Errorf("invalid fossil template pairs or empty corner limit")
	}
	feature := &fossilFeature{maxEmptyCorners: *config.MaxEmptyCorners}
	for index, id := range config.Base {
		base, err := store.load(id)
		if err != nil {
			return nil, err
		}
		overlay, err := store.load(config.Overlay[index])
		if err != nil {
			return nil, err
		}
		feature.base = append(feature.base, base)
		feature.overlay = append(feature.overlay, overlay)
	}
	var err error
	feature.baseProcessors, err = decodeTemplateProcessors(snapshot, config.BaseProcessors)
	if err != nil {
		return nil, fmt.Errorf("fossil processors: %w", err)
	}
	feature.overlayProcessors, err = decodeTemplateProcessors(snapshot, config.OverlayProcessors)
	if err != nil {
		return nil, fmt.Errorf("fossil overlay processors: %w", err)
	}
	return feature, nil
}

// placeFeature preserves rotation, template choice, burial depth, and overlay RNG ordering.
func (f *fossilFeature) placeFeature(_ *featureSystem, target *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	transform := templateTransform{rotation: templateRotation(random.NextIntBound(4))}
	index := random.NextIntBound(int32(len(f.base)))
	base, overlay := f.base[index], f.overlay[index]
	size := base.rotatedSize(transform.rotation)
	minimum := blockPosition{origin.x - size.x/2, origin.y, origin.z - size.z/2}
	lowest := origin.y
	for dx := 0; dx < size.x; dx++ {
		for dz := 0; dz < size.z; dz++ {
			x, z := minimum.x+dx, minimum.z+dz
			if !target.FeatureRegionEnabled() && (x < int(target.X)*16 || x >= int(target.X)*16+16 || z < int(target.Z)*16 || z >= int(target.Z)*16+16) {
				continue
			}
			height, _ := target.FeatureHeightmap(x, z, section.HeightmapOceanFloor)
			lowest = min(lowest, int(height))
		}
	}
	minimum.y = max(lowest-15-int(random.NextIntBound(10)), target.MinY()+10)
	position := base.anchoredOrigin(minimum, transform)
	low, high := base.bounds(position, transform)
	empty := 0
	for _, x := range [2]int{low.x, high.x} {
		for _, y := range [2]int{low.y, high.y} {
			for _, z := range [2]int{low.z, high.z} {
				state := getChunkBlock(target, blockPosition{x, y, z})
				name := block.NameFromState(state)
				if block.IsAir(state) || name == "minecraft:water" || name == "minecraft:lava" {
					empty++
				}
			}
		}
	}
	if empty > f.maxEmptyCorners {
		return false
	}
	base.place(target, random, position, transform, f.baseProcessors)
	overlay.place(target, random, position, transform, f.overlayProcessors)
	return true
}
