package worldgen

import (
	"reflect"
	"sort"
	"testing"

	"github.com/vitismc/vitis/internal/datapack"
)

// TestVanillaFeatureSupportInventory pins missing content and detects decoder regressions.
func TestVanillaFeatureSupportInventory(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	set := runtime.current()
	var missing []string
	for id := range set.resources["configured_feature"] {
		if set.features.configured[id] == nil {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	expected := []string{
		"minecraft:end_spike",
		"minecraft:sculk_patch_ancient_city", "minecraft:sculk_patch_deep_dark",
	}
	if !reflect.DeepEqual(missing, expected) {
		t.Fatalf("missing configured features changed: %v", missing)
	}
	missingBiomeFeatures := make(map[string]struct{})
	for _, steps := range set.features.biomeFeatures {
		for _, ids := range steps {
			for _, id := range ids {
				if set.features.placed[id] == nil {
					missingBiomeFeatures[id] = struct{}{}
				}
			}
		}
	}
	biomeMissing := make([]string, 0, len(missingBiomeFeatures))
	for id := range missingBiomeFeatures {
		biomeMissing = append(biomeMissing, id)
	}
	sort.Strings(biomeMissing)
	t.Logf("configured=%d/%d placed=%d/%d missing_biome_placements=%v", len(set.features.configured), len(set.resources["configured_feature"]), len(set.features.placed), len(set.resources["placed_feature"]), biomeMissing)
}
