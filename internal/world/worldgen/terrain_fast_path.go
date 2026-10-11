package worldgen

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
)

// standardNoiseRegistryHash identifies the immutable 1.21.4 noise and density definitions.
const standardNoiseRegistryHash = "cc1220f27b1d3837bde9752ba82a68226e696db03b1948f9d239735eed45ff92"

// standardDensityDefinitions validates resources before enabling cell-based terrain filling.
func standardDensityDefinitions(resources map[string]map[string]json.RawMessage) bool {
	hash := sha256.New()
	var compact bytes.Buffer
	for _, kind := range []string{"density_function", "noise"} {
		registry := resources[kind]
		ids := make([]string, 0, len(registry))
		for id := range registry {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			compact.Reset()
			if json.Compact(&compact, registry[id]) != nil {
				return false
			}
			fmt.Fprintf(hash, "%s/%s\x00", kind, id)
			hash.Write(compact.Bytes())
			hash.Write([]byte{0})
		}
	}
	return fmt.Sprintf("%x", hash.Sum(nil)) == standardNoiseRegistryHash
}

// standardTerrainSettings keeps modified settings on the general density evaluator.
func (r *ResourceSet) standardTerrainSettings(id string) bool {
	if !r.standardDensity {
		return false
	}
	var expected string
	switch id {
	case "minecraft:overworld":
		expected = "f6e5a334f720d7d7166ccff25328480c2e4bbc7ab964e358553d3083893d5471"
	case "minecraft:nether":
		expected = "be9cd3c135dcc497566d209653b485504adf23b1e3007d852345c11d9b2e0d6d"
	case "minecraft:end":
		expected = "32c8649497b45a430dcc32d65cb43b44aeeed3c17127a1c6e311133a5178fc35"
	default:
		return false
	}
	var compact bytes.Buffer
	if json.Compact(&compact, r.resources["noise_settings"][id]) != nil {
		return false
	}
	return fmt.Sprintf("%x", sha256.Sum256(compact.Bytes())) == expected
}
