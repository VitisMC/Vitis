package worldgen

import (
	"encoding/json"
	"fmt"
	"sort"

	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type structureEntry struct {
	id     string
	weight int
}

type structurePlacement struct {
	kind            string
	salt            int32
	spacing         int
	separation      int
	triangular      bool
	frequency       float32
	reduction       string
	locateOffset    [3]int
	exclusionSet    string
	exclusionRadius int
	ringDistance    int
	ringSpread      int
	ringCount       int
	preferredBiomes json.RawMessage
}

type structureSet struct {
	entries   []structureEntry
	placement structurePlacement
}

// compileStructureSets validates structure references and placement settings.
func compileStructureSets(resources *ResourceSet) error {
	resources.structureSets = make(map[string]*structureSet, len(resources.resources["structure_set"]))
	ids := make([]string, 0, len(resources.resources["structure_set"]))
	for id := range resources.resources["structure_set"] {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		set, err := decodeStructureSet(resources.resources["structure_set"][id], resources.resources["structure"])
		if err != nil {
			return fmt.Errorf("structure set %s: %w", id, err)
		}
		resources.structureSets[id] = set
	}
	for _, id := range ids {
		other := resources.structureSets[id].placement.exclusionSet
		if other != "" && resources.structureSets[other] == nil {
			return fmt.Errorf("structure set %s: unknown excluded set %s", id, other)
		}
	}
	for _, id := range ids {
		seen := make(map[string]bool)
		for current := id; current != ""; current = resources.structureSets[current].placement.exclusionSet {
			if seen[current] {
				return fmt.Errorf("structure set %s: cyclic exclusion zone", id)
			}
			seen[current] = true
		}
	}
	return nil
}

// isStructureChunk applies the candidate, frequency, and exclusion rules.
func (resources *ResourceSet) isStructureChunk(setID string, seed int64, chunkX, chunkZ int32) bool {
	set := resources.structureSets[setID]
	if set == nil {
		return false
	}
	p := set.placement
	x, z, ok := p.potentialChunk(seed, chunkX, chunkZ)
	if !ok || x != chunkX || z != chunkZ || !p.passesFrequency(seed, chunkX, chunkZ) {
		return false
	}
	if p.exclusionSet == "" {
		return true
	}
	radius := int32(p.exclusionRadius)
	for x := chunkX - radius; x <= chunkX+radius; x++ {
		for z := chunkZ - radius; z <= chunkZ+radius; z++ {
			if resources.isStructureChunk(p.exclusionSet, seed, x, z) {
				return false
			}
		}
	}
	return true
}

// decodeStructureSet decodes one weighted set and its placement rule.
func decodeStructureSet(raw json.RawMessage, structures map[string]json.RawMessage) (*structureSet, error) {
	var value struct {
		Structures []struct {
			Structure string `json:"structure"`
			Weight    *int   `json:"weight"`
		} `json:"structures"`
		Placement struct {
			Type         string   `json:"type"`
			Salt         *int64   `json:"salt"`
			Spacing      *int     `json:"spacing"`
			Separation   *int     `json:"separation"`
			SpreadType   string   `json:"spread_type"`
			Frequency    *float32 `json:"frequency"`
			Reduction    string   `json:"frequency_reduction_method"`
			LocateOffset []int    `json:"locate_offset"`
			Exclusion    *struct {
				OtherSet   string `json:"other_set"`
				ChunkCount int    `json:"chunk_count"`
			} `json:"exclusion_zone"`
			Distance        int             `json:"distance"`
			Spread          int             `json:"spread"`
			Count           int             `json:"count"`
			PreferredBiomes json.RawMessage `json:"preferred_biomes"`
		} `json:"placement"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	if len(value.Structures) == 0 {
		return nil, fmt.Errorf("empty structures")
	}
	result := &structureSet{entries: make([]structureEntry, 0, len(value.Structures))}
	for _, entry := range value.Structures {
		id := normalizeResourceID(entry.Structure)
		if entry.Structure == "" || structures[id] == nil {
			return nil, fmt.Errorf("unknown structure %s", entry.Structure)
		}
		if entry.Weight == nil || *entry.Weight < 1 {
			return nil, fmt.Errorf("structure %s: weight must be positive", id)
		}
		result.entries = append(result.entries, structureEntry{id: id, weight: *entry.Weight})
	}
	p := &result.placement
	p.kind = value.Placement.Type
	if value.Placement.Salt == nil || *value.Placement.Salt < 0 || *value.Placement.Salt > 2147483647 {
		return nil, fmt.Errorf("placement salt must be a non-negative 32-bit integer")
	}
	p.salt = int32(*value.Placement.Salt)
	p.frequency = 1
	if value.Placement.Frequency != nil {
		p.frequency = *value.Placement.Frequency
	}
	if p.frequency < 0 || p.frequency > 1 {
		return nil, fmt.Errorf("placement frequency must be in [0,1]")
	}
	p.reduction = value.Placement.Reduction
	if p.reduction == "" {
		p.reduction = "default"
	}
	switch p.reduction {
	case "default", "legacy_type_1", "legacy_type_2", "legacy_type_3":
	default:
		return nil, fmt.Errorf("unknown frequency reduction method %q", p.reduction)
	}
	if len(value.Placement.LocateOffset) != 0 {
		if len(value.Placement.LocateOffset) != 3 {
			return nil, fmt.Errorf("locate offset must have three coordinates")
		}
		for i, coordinate := range value.Placement.LocateOffset {
			if coordinate < -16 || coordinate > 16 {
				return nil, fmt.Errorf("locate offset is outside [-16,16]")
			}
			p.locateOffset[i] = coordinate
		}
	}
	if value.Placement.Exclusion != nil {
		if value.Placement.Exclusion.OtherSet == "" || value.Placement.Exclusion.ChunkCount < 1 || value.Placement.Exclusion.ChunkCount > 16 {
			return nil, fmt.Errorf("invalid exclusion zone")
		}
		p.exclusionSet = normalizeResourceID(value.Placement.Exclusion.OtherSet)
		p.exclusionRadius = value.Placement.Exclusion.ChunkCount
	}
	switch p.kind {
	case "minecraft:random_spread":
		if value.Placement.Spacing == nil || value.Placement.Separation == nil || *value.Placement.Spacing < 1 || *value.Placement.Spacing > 4096 || *value.Placement.Separation < 0 || *value.Placement.Separation >= *value.Placement.Spacing {
			return nil, fmt.Errorf("random spread requires 1 <= spacing <= 4096 and 0 <= separation < spacing")
		}
		p.spacing, p.separation = *value.Placement.Spacing, *value.Placement.Separation
		switch value.Placement.SpreadType {
		case "", "linear":
		case "triangular":
			p.triangular = true
		default:
			return nil, fmt.Errorf("unknown spread type %q", value.Placement.SpreadType)
		}
	case "minecraft:concentric_rings":
		if value.Placement.Distance < 0 || value.Placement.Distance > 1023 || value.Placement.Spread < 0 || value.Placement.Spread > 1023 || value.Placement.Count < 1 || value.Placement.Count > 4095 || len(value.Placement.PreferredBiomes) == 0 {
			return nil, fmt.Errorf("invalid concentric ring placement")
		}
		var single string
		if err := json.Unmarshal(value.Placement.PreferredBiomes, &single); err == nil {
			if single == "" {
				return nil, fmt.Errorf("empty preferred biomes")
			}
		} else {
			var entries []string
			if err := json.Unmarshal(value.Placement.PreferredBiomes, &entries); err != nil || len(entries) == 0 {
				return nil, fmt.Errorf("invalid preferred biomes")
			}
			for _, entry := range entries {
				if entry == "" {
					return nil, fmt.Errorf("empty preferred biome")
				}
			}
		}
		p.ringDistance, p.ringSpread, p.ringCount = value.Placement.Distance, value.Placement.Spread, value.Placement.Count
		p.preferredBiomes = append(json.RawMessage(nil), value.Placement.PreferredBiomes...)
	default:
		return nil, fmt.Errorf("unknown placement type %q", p.kind)
	}
	return result, nil
}

// potentialChunk returns the random-spread candidate in the queried region.
func (p structurePlacement) potentialChunk(seed int64, chunkX, chunkZ int32) (int32, int32, bool) {
	if p.kind != "minecraft:random_spread" {
		return 0, 0, false
	}
	regionX, regionZ := floorDiv(int(chunkX), p.spacing), floorDiv(int(chunkZ), p.spacing)
	// A one-position spread cannot move inside its region; its private RNG has no later consumer.
	if p.spacing-p.separation == 1 {
		return int32(regionX * p.spacing), int32(regionZ * p.spacing), true
	}
	random := worldrandom.NewWorldgen(worldrandom.NewLegacy(0))
	random.SetLargeFeatureWithSalt(seed, int32(regionX), int32(regionZ), p.salt)
	bound := int32(p.spacing - p.separation)
	spread := func() int32 {
		first := random.NextIntBound(bound)
		if p.triangular {
			return (first + random.NextIntBound(bound)) / 2
		}
		return first
	}
	return int32(regionX*p.spacing) + spread(), int32(regionZ*p.spacing) + spread(), true
}

// passesFrequency applies the placement's deterministic frequency gate.
func (p structurePlacement) passesFrequency(seed int64, chunkX, chunkZ int32) bool {
	if p.frequency >= 1 {
		return true
	}
	if p.frequency <= 0 {
		return false
	}
	random := worldrandom.NewWorldgen(worldrandom.NewLegacy(0))
	switch p.reduction {
	case "legacy_type_2":
		random.SetLargeFeatureWithSalt(seed, chunkX, chunkZ, 10387320)
	case "legacy_type_3":
		random.SetLargeFeatureSeed(seed, chunkX, chunkZ)
		return random.NextDouble() < float64(p.frequency)
	case "legacy_type_1":
		regionX, regionZ := chunkX>>4, chunkZ>>4
		random.SetSeed(int64(regionX^(regionZ<<4)) ^ seed)
		random.NextInt()
		return random.NextIntBound(int32(1/p.frequency)) == 0
	default:
		random.SetLargeFeatureWithSalt(seed, chunkX, chunkZ, p.salt)
	}
	return random.NextFloat() < p.frequency
}
