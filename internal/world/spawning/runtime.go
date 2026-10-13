package spawning

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"github.com/vitismc/vitis/internal/datapack"
)

const compiledSpawningKey = "natural_spawning"

// Category identifies a spawn budget independently of mob behavior.
type Category string

const (
	Monster                  Category = "monster"
	Creature                 Category = "creature"
	Ambient                  Category = "ambient"
	WaterCreature            Category = "water_creature"
	WaterAmbient             Category = "water_ambient"
	UndergroundWaterCreature Category = "underground_water_creature"
	Axolotls                 Category = "axolotls"
)

// categoryLimits fixes deterministic vanilla spawn-budget ordering.
var categoryLimits = [...]struct {
	category Category
	cap      int
}{
	{Monster, 70}, {Creature, 10}, {Ambient, 15}, {WaterCreature, 5},
	{WaterAmbient, 20}, {UndergroundWaterCreature, 5}, {Axolotls, 5},
}

// DimensionRules contains placement bounds and monster light limits.
type DimensionRules struct {
	MinY            int  `json:"min_y"`
	Height          int  `json:"height"`
	SkyLight        bool `json:"has_skylight"`
	BlockLightLimit int  `json:"monster_spawn_block_light_limit"`
	LightMin        int
	LightMax        int
}

// spawnDefinitions is an immutable resource snapshot of biome and dimension rules.
type spawnDefinitions struct {
	biomes     map[string]map[Category][]SpawnEntry
	costs      map[string]map[string]spawnCost
	biomeTags  map[string]map[string]bool
	dimensions map[string]DimensionRules
	categories map[string]Category
}

// Runtime reads natural-spawn definitions from atomically published resources.
type Runtime struct{ resources *datapack.Manager }

// NewRuntime registers spawn-definition compilation before the first reload.
func NewRuntime(resources *datapack.Manager) (*Runtime, error) {
	if resources == nil {
		return nil, fmt.Errorf("create spawning runtime: nil resources")
	}
	if err := resources.RegisterCompiler(compiledSpawningKey, compileSpawning); err != nil {
		return nil, err
	}
	return &Runtime{resources: resources}, nil
}

// current acquires all biome, category and dimension data from one snapshot.
func (r *Runtime) current() *spawnDefinitions {
	if r == nil || r.resources == nil {
		return nil
	}
	snapshot := r.resources.Current()
	if snapshot == nil {
		return nil
	}
	value, _ := snapshot.Compiled(compiledSpawningKey)
	definitions, _ := value.(*spawnDefinitions)
	return definitions
}

// resourceID resolves one canonical datapack path for a resource directory.
func resourceID(name, directory string) (string, bool) {
	parts := strings.SplitN(name, "/", 3)
	prefix := directory + "/"
	if len(parts) != 3 || parts[0] != "data" || !strings.HasPrefix(parts[2], prefix) || !strings.HasSuffix(name, ".json") {
		return "", false
	}
	return parts[1] + ":" + strings.TrimSuffix(strings.TrimPrefix(parts[2], prefix), ".json"), true
}

// compileSpawning validates every definition before the resource snapshot is published.
func compileSpawning(snapshot *datapack.Snapshot) (any, error) {
	definitions := &spawnDefinitions{biomes: map[string]map[Category][]SpawnEntry{}, costs: map[string]map[string]spawnCost{}, dimensions: map[string]DimensionRules{}, categories: map[string]Category{
		"zombie": Monster, "skeleton": Monster, "creeper": Monster, "spider": Monster, "enderman": Monster, "witch": Monster, "slime": Monster, "husk": Monster, "drowned": Monster, "stray": Monster, "cave_spider": Monster, "blaze": Monster, "wither_skeleton": Monster, "phantom": Monster,
		"cow": Creature, "pig": Creature, "sheep": Creature, "chicken": Creature, "wolf": Creature, "bat": Ambient,
	}}
	for _, name := range snapshot.Names() {
		data, _ := snapshot.Get(name)
		if id, ok := resourceID(name, "worldgen/biome"); ok {
			var biome struct {
				Costs    map[string]json.RawMessage `json:"spawn_costs"`
				Spawners map[Category][]struct {
					Type   string `json:"type"`
					Weight int    `json:"weight"`
					Min    int    `json:"minCount"`
					Max    int    `json:"maxCount"`
				} `json:"spawners"`
			}
			if err := json.Unmarshal(data, &biome); err != nil {
				return nil, fmt.Errorf("biome spawning %s: %w", id, err)
			}
			groups := make(map[Category][]SpawnEntry)
			for category, entries := range biome.Spawners {
				known := category == "misc"
				for _, limit := range categoryLimits {
					known = known || category == limit.category
				}
				if !known {
					return nil, fmt.Errorf("biome %s: unknown spawning category %q", id, category)
				}
				for _, entry := range entries {
					if entry.Type == "" || entry.Weight < 1 || entry.Weight > 1000000 || entry.Min < 1 || entry.Max < entry.Min || entry.Max > 64 {
						return nil, fmt.Errorf("biome %s: invalid spawn entry %q", id, entry.Type)
					}
					typeName := strings.TrimPrefix(entry.Type, "minecraft:")
					groups[category] = append(groups[category], SpawnEntry{TypeName: typeName, Weight: entry.Weight, MinGroup: entry.Min, MaxGroup: entry.Max, Group: category})
					if previous, exists := definitions.categories[typeName]; exists && previous != category {
						return nil, fmt.Errorf("inconsistent spawning category for %s", entry.Type)
					}
					definitions.categories[typeName] = category
				}
			}
			definitions.biomes[id] = groups
			costs, err := compileSpawnCosts(biome.Costs)
			if err != nil {
				return nil, fmt.Errorf("biome %s: %w", id, err)
			}
			definitions.costs[id] = costs
		}
		if id, ok := resourceID(name, "dimension_type"); ok {
			var rules DimensionRules
			var raw struct {
				Light json.RawMessage `json:"monster_spawn_light_level"`
			}
			if err := json.Unmarshal(data, &rules); err != nil {
				return nil, fmt.Errorf("spawn dimension %s: %w", id, err)
			}
			if err := json.Unmarshal(data, &raw); err != nil {
				return nil, err
			}
			if err := json.Unmarshal(raw.Light, &rules.LightMin); err != nil {
				var rangeValue struct {
					Type string `json:"type"`
					Min  int    `json:"min_inclusive"`
					Max  int    `json:"max_inclusive"`
				}
				if json.Unmarshal(raw.Light, &rangeValue) != nil || rangeValue.Type != "minecraft:uniform" {
					return nil, fmt.Errorf("dimension %s: invalid monster light range", id)
				}
				rules.LightMin, rules.LightMax = rangeValue.Min, rangeValue.Max
			} else {
				rules.LightMax = rules.LightMin
			}
			if rules.MinY < -2032 || rules.Height < 16 || rules.Height > 4064 || rules.MinY%16 != 0 || rules.Height%16 != 0 || rules.MinY+rules.Height > 2032 || rules.BlockLightLimit < 0 || rules.BlockLightLimit > 15 || rules.LightMin < 0 || rules.LightMax < rules.LightMin || rules.LightMax > 15 {
				return nil, fmt.Errorf("dimension %s: invalid spawn bounds or light limits", id)
			}
			definitions.dimensions[id] = rules
		}
	}
	dimensionTypes := maps.Clone(definitions.dimensions)
	for _, name := range snapshot.Names() {
		id, ok := resourceID(name, "dimension")
		if !ok {
			continue
		}
		data, _ := snapshot.Get(name)
		var dimension struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(data, &dimension); err != nil {
			return nil, fmt.Errorf("spawning dimension %s: %w", id, err)
		}
		rules, exists := dimensionTypes[dimension.Type]
		if !exists {
			return nil, fmt.Errorf("spawning dimension %s: unknown type %s", id, dimension.Type)
		}
		definitions.dimensions[id] = rules
	}
	definitions.biomeTags = make(map[string]map[string]bool)
	for _, tag := range []string{"more_frequent_drowned_spawns", "allows_surface_slime_spawns"} {
		if _, exists := snapshot.Get("data/minecraft/tags/worldgen/biome/" + tag + ".json"); !exists {
			continue
		}
		values, err := snapshot.ResolveTag("worldgen/biome", "minecraft:"+tag)
		if err != nil {
			return nil, fmt.Errorf("spawning biome tag %s: %w", tag, err)
		}
		members := make(map[string]bool, len(values))
		for _, biome := range values {
			members[biome] = true
		}
		definitions.biomeTags[tag] = members
	}
	return definitions, nil
}
