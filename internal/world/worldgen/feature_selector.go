package worldgen

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

var errUnsupportedPlacedFeature = errors.New("unsupported placed feature")

type placedFeatureReference struct {
	id     string
	inline *placedFeature
}

func (r placedFeatureReference) place(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, position blockPosition) bool {
	feature := r.inline
	if feature == nil && system != nil && system.resources != nil {
		feature = system.resources.placed[r.id]
	}
	if feature == nil {
		return false
	}
	return feature.place(system, chunk, "", random, position, nil, nil)
}

type randomSelectorEntry struct {
	chance  float32
	feature placedFeatureReference
}
type randomSelectorFeature struct {
	entries  []randomSelectorEntry
	fallback placedFeatureReference
}

func (f *randomSelectorFeature) placeFeature(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, position blockPosition) bool {
	for _, entry := range f.entries {
		if random.NextFloat() < entry.chance {
			return entry.feature.place(system, chunk, random, position)
		}
	}
	return f.fallback.place(system, chunk, random, position)
}

type simpleSelectorFeature struct{ features []placedFeatureReference }

func (f *simpleSelectorFeature) placeFeature(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, position blockPosition) bool {
	if len(f.features) == 0 {
		return false
	}
	return f.features[random.NextIntBound(int32(len(f.features)))].place(system, chunk, random, position)
}

type booleanSelectorFeature struct{ whenTrue, whenFalse placedFeatureReference }

func (f *booleanSelectorFeature) placeFeature(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, position blockPosition) bool {
	if random.NextBoolean() {
		return f.whenTrue.place(system, chunk, random, position)
	}
	return f.whenFalse.place(system, chunk, random, position)
}

func decodeSelectorFeature(snapshot *datapack.Snapshot, kind string, raw json.RawMessage) (configuredFeature, bool, error) {
	switch kind {
	case "minecraft:random_selector":
		var value struct {
			Config struct {
				Default  json.RawMessage `json:"default"`
				Features []struct {
					Chance  float32         `json:"chance"`
					Feature json.RawMessage `json:"feature"`
				} `json:"features"`
			} `json:"config"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, true, err
		}
		fallback, err := decodePlacedFeatureReference(snapshot, value.Config.Default)
		if errors.Is(err, errUnsupportedPlacedFeature) {
			return nil, false, nil
		}
		if err != nil {
			return nil, true, err
		}
		feature := &randomSelectorFeature{fallback: fallback}
		for index, entry := range value.Config.Features {
			if entry.Chance < 0 || entry.Chance > 1 {
				return nil, true, fmt.Errorf("selector entry %d chance out of range", index)
			}
			reference, err := decodePlacedFeatureReference(snapshot, entry.Feature)
			if errors.Is(err, errUnsupportedPlacedFeature) {
				return nil, false, nil
			}
			if err != nil {
				return nil, true, fmt.Errorf("selector entry %d: %w", index, err)
			}
			feature.entries = append(feature.entries, randomSelectorEntry{chance: entry.Chance, feature: reference})
		}
		return feature, true, nil
	case "minecraft:simple_random_selector":
		var value struct {
			Config struct {
				Features []json.RawMessage `json:"features"`
			} `json:"config"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, true, err
		}
		feature := &simpleSelectorFeature{}
		for index, rawFeature := range value.Config.Features {
			reference, err := decodePlacedFeatureReference(snapshot, rawFeature)
			if errors.Is(err, errUnsupportedPlacedFeature) {
				return nil, false, nil
			}
			if err != nil {
				return nil, true, fmt.Errorf("selector entry %d: %w", index, err)
			}
			feature.features = append(feature.features, reference)
		}
		if len(feature.features) == 0 {
			return nil, true, fmt.Errorf("empty simple selector")
		}
		return feature, true, nil
	case "minecraft:random_boolean_selector":
		var value struct {
			Config struct {
				True  json.RawMessage `json:"feature_true"`
				False json.RawMessage `json:"feature_false"`
			} `json:"config"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, true, err
		}
		whenTrue, err := decodePlacedFeatureReference(snapshot, value.Config.True)
		if errors.Is(err, errUnsupportedPlacedFeature) {
			return nil, false, nil
		}
		if err != nil {
			return nil, true, err
		}
		whenFalse, err := decodePlacedFeatureReference(snapshot, value.Config.False)
		if errors.Is(err, errUnsupportedPlacedFeature) {
			return nil, false, nil
		}
		if err != nil {
			return nil, true, err
		}
		return &booleanSelectorFeature{whenTrue: whenTrue, whenFalse: whenFalse}, true, nil
	default:
		return nil, false, nil
	}
}

func decodePlacedFeatureReference(snapshot *datapack.Snapshot, raw json.RawMessage) (placedFeatureReference, error) {
	var id string
	if err := json.Unmarshal(raw, &id); err == nil {
		if id == "" {
			return placedFeatureReference{}, fmt.Errorf("empty placed feature reference")
		}
		return placedFeatureReference{id: normalizeResourceID(id)}, nil
	}
	placed, ok, err := decodeInlinePlacedFeature(snapshot, raw)
	if err != nil {
		return placedFeatureReference{}, err
	}
	if !ok {
		return placedFeatureReference{}, errUnsupportedPlacedFeature
	}
	return placedFeatureReference{inline: placed}, nil
}
