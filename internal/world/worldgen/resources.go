// Package worldgen loads immutable data driven world generation resources.
package worldgen

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/worldgen/density"
	"github.com/vitismc/vitis/internal/world/worldgen/noise"
)

const compiledWorldgenKey = "worldgen"

// Runtime exposes the active world generation resource snapshot.
type Runtime struct {
	resources *datapack.Manager
}

// BlockState describes a configured default block or fluid state.
type BlockState struct {
	Name       string            `json:"Name"`
	Properties map[string]string `json:"Properties,omitempty"`
}

// NoiseSettings contains the dimension shape and density graph inputs.
type NoiseSettings struct {
	AquifersEnabled      bool            `json:"aquifers_enabled"`
	DefaultBlock         BlockState      `json:"default_block"`
	DefaultFluid         BlockState      `json:"default_fluid"`
	DisableMobGeneration bool            `json:"disable_mob_generation"`
	LegacyRandomSource   bool            `json:"legacy_random_source"`
	OreVeinsEnabled      bool            `json:"ore_veins_enabled"`
	SeaLevel             int             `json:"sea_level"`
	Noise                NoiseShape      `json:"noise"`
	NoiseRouter          json.RawMessage `json:"noise_router"`
	SpawnTarget          json.RawMessage `json:"spawn_target"`
	SurfaceRule          json.RawMessage `json:"surface_rule"`
}

// NoiseShape defines the sampled vertical range and cell sizes.
type NoiseShape struct {
	MinY           int `json:"min_y"`
	Height         int `json:"height"`
	SizeHorizontal int `json:"size_horizontal"`
	SizeVertical   int `json:"size_vertical"`
}

// DimensionType defines vertical bounds and portal-related dimension behavior.
type DimensionType struct {
	AmbientLight       float32 `json:"ambient_light"`
	BedWorks           bool    `json:"bed_works"`
	CoordinateScale    float64 `json:"coordinate_scale"`
	Effects            string  `json:"effects"`
	HasCeiling         bool    `json:"has_ceiling"`
	HasRaids           bool    `json:"has_raids"`
	HasSkylight        bool    `json:"has_skylight"`
	Height             int     `json:"height"`
	Infiniburn         string  `json:"infiniburn"`
	LogicalHeight      int     `json:"logical_height"`
	MinY               int     `json:"min_y"`
	Natural            bool    `json:"natural"`
	PiglinSafe         bool    `json:"piglin_safe"`
	RespawnAnchorWorks bool    `json:"respawn_anchor_works"`
	Ultrawarm          bool    `json:"ultrawarm"`
}

// Dimension defines a dimension type and its chunk generator.
type Dimension struct {
	Type      string             `json:"type"`
	Generator DimensionGenerator `json:"generator"`
}

// DimensionGenerator defines a data-driven dimension chunk generator.
type DimensionGenerator struct {
	Type        string          `json:"type"`
	Settings    string          `json:"settings"`
	BiomeSource json.RawMessage `json:"biome_source"`
}

// ResourceSet contains all worldgen registries and decoded noise settings.
type ResourceSet struct {
	standardDensity      bool
	resources            map[string]map[string]json.RawMessage
	noiseSettings        map[string]NoiseSettings
	noise                map[string]noise.Parameters
	dimensionTypes       map[string]DimensionType
	dimensions           map[string]Dimension
	carvers              map[string]*configuredCarver
	biomeCarvers         map[string][]string
	structureSets        map[string]*structureSet
	buriedTreasureBiomes map[int32]bool
	features             *featureSystemResources
	templates            *structureTemplates
	netherFossils        map[string]*netherFossilDefinition
}

// DimensionType returns a registered dimension type.
func (r *Runtime) DimensionType(id string) (DimensionType, bool) {
	set := r.current()
	if set == nil {
		return DimensionType{}, false
	}
	value, ok := set.dimensionTypes[id]
	return value, ok
}

// Dimension returns one registered dimension definition.
func (r *Runtime) Dimension(id string) (Dimension, bool) {
	set := r.current()
	if set == nil {
		return Dimension{}, false
	}
	value, ok := set.dimensions[id]
	value.Generator.BiomeSource = append(json.RawMessage(nil), value.Generator.BiomeSource...)
	return value, ok
}

// DimensionSeaLevel reads a dimension's sea level from one immutable snapshot.
func (r *Runtime) DimensionSeaLevel(id string) (int, bool) {
	set := r.current()
	if set == nil {
		return 0, false
	}
	dimension, ok := set.dimensions[id]
	if !ok {
		return 0, false
	}
	settings, ok := set.noiseSettings[dimension.Generator.Settings]
	return settings.SeaLevel, ok
}

// Dimensions returns registered dimension identifiers in stable order.
func (r *Runtime) Dimensions() []string {
	set := r.current()
	if set == nil {
		return nil
	}
	ids := make([]string, 0, len(set.dimensions))
	for id := range set.dimensions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// NoiseParameters returns an independent copy of one registered noise definition.
func (r *Runtime) NoiseParameters(id string) (noise.Parameters, bool) {
	set := r.current()
	if set == nil {
		return noise.Parameters{}, false
	}
	parameters, ok := set.noise[id]
	if !ok {
		return noise.Parameters{}, false
	}
	parameters.Amplitudes = append([]float64(nil), parameters.Amplitudes...)
	return parameters, true
}

// NewDensityCompiler creates a seeded compiler using one generator's random source mode.
func (r *Runtime) NewDensityCompiler(settingsID string, seed int64) (*density.Compiler, error) {
	set := r.current()
	if set == nil {
		return nil, fmt.Errorf("create density compiler: worldgen resources are not loaded")
	}
	return newDensityCompiler(set, settingsID, seed)
}

func newDensityCompiler(set *ResourceSet, settingsID string, seed int64) (*density.Compiler, error) {
	settings, ok := set.noiseSettings[settingsID]
	if !ok {
		return nil, fmt.Errorf("create density compiler: unknown noise settings %s", settingsID)
	}
	definitions := make(map[string]json.RawMessage, len(set.resources["density_function"]))
	for id, data := range set.resources["density_function"] {
		definitions[id] = append(json.RawMessage(nil), data...)
	}
	noises := make(map[string]noise.Parameters, len(set.noise))
	for id, parameters := range set.noise {
		parameters.Amplitudes = append([]float64(nil), parameters.Amplitudes...)
		noises[id] = parameters
	}
	compiler, err := density.NewCompiler(density.Registry{
		Definitions: definitions,
		Noises:      noises,
		Seed:        seed,
		Legacy:      settings.LegacyRandomSource,
		CellWidth:   settings.Noise.SizeHorizontal * 4,
		CellHeight:  settings.Noise.SizeVertical * 4,
	})
	if err != nil {
		return nil, err
	}
	if settingsID == "minecraft:overworld" && set.standardTerrainSettings(settingsID) {
		if err := compiler.EnableStandardOverworld(); err != nil {
			return nil, err
		}
	}
	return compiler, nil
}

// NewRuntime registers worldgen resource compilation with the data pack manager.
func NewRuntime(resources *datapack.Manager) (*Runtime, error) {
	if resources == nil {
		return nil, fmt.Errorf("create worldgen runtime: nil resource manager")
	}
	runtime := &Runtime{resources: resources}
	if err := resources.RegisterCompiler(compiledWorldgenKey, compileResources); err != nil {
		return nil, err
	}
	return runtime, nil
}

// Count returns the number of active worldgen resources.
func (r *Runtime) Count() int {
	set := r.current()
	if set == nil {
		return 0
	}
	count := 0
	for _, registry := range set.resources {
		count += len(registry)
	}
	return count
}

// Resource returns a copy of one raw worldgen resource.
func (r *Runtime) Resource(kind, id string) (json.RawMessage, bool) {
	set := r.current()
	if set == nil {
		return nil, false
	}
	data, ok := set.resources[kind][id]
	return append(json.RawMessage(nil), data...), ok
}

// NoiseSettings returns decoded settings for a named generator preset.
func (r *Runtime) NoiseSettings(id string) (NoiseSettings, bool) {
	set := r.current()
	if set == nil {
		return NoiseSettings{}, false
	}
	settings, ok := set.noiseSettings[id]
	return settings, ok
}

func (r *Runtime) current() *ResourceSet {
	if r == nil || r.resources == nil {
		return nil
	}
	compiled, ok := r.resources.Current().Compiled(compiledWorldgenKey)
	if !ok {
		return nil
	}
	set, _ := compiled.(*ResourceSet)
	return set
}

func compileResources(snapshot *datapack.Snapshot) (any, error) {
	set := &ResourceSet{
		templates:      newStructureTemplates(snapshot),
		resources:      make(map[string]map[string]json.RawMessage),
		noiseSettings:  make(map[string]NoiseSettings),
		noise:          make(map[string]noise.Parameters),
		dimensionTypes: make(map[string]DimensionType),
		dimensions:     make(map[string]Dimension),
	}
	for _, name := range snapshot.Names() {
		kind, id, ok := registryIDFromPath(name)
		if !ok {
			continue
		}
		data, _ := snapshot.Get(name)
		registry := set.resources[kind]
		if registry == nil {
			registry = make(map[string]json.RawMessage)
			set.resources[kind] = registry
		}
		registry[id] = append(json.RawMessage(nil), data...)
		if kind == "noise_settings" {
			var settings NoiseSettings
			if err := json.Unmarshal(data, &settings); err != nil {
				return nil, fmt.Errorf("worldgen noise settings %s: %w", id, err)
			}
			if err := validateNoiseSettings(id, settings); err != nil {
				return nil, err
			}
			set.noiseSettings[id] = settings
		}
		if kind == "noise" {
			var parameters noise.Parameters
			if err := json.Unmarshal(data, &parameters); err != nil {
				return nil, fmt.Errorf("worldgen noise %s: %w", id, err)
			}
			if err := validateNoiseParameters(id, parameters); err != nil {
				return nil, err
			}
			parameters.Amplitudes = append([]float64(nil), parameters.Amplitudes...)
			set.noise[id] = parameters
		}
		if kind == "dimension_type" {
			var dimensionType DimensionType
			if err := json.Unmarshal(data, &dimensionType); err != nil {
				return nil, fmt.Errorf("dimension type %s: %w", id, err)
			}
			if err := validateDimensionType(id, dimensionType); err != nil {
				return nil, err
			}
			set.dimensionTypes[id] = dimensionType
		}
		if kind == "dimension" {
			var dimension Dimension
			if err := json.Unmarshal(data, &dimension); err != nil {
				return nil, fmt.Errorf("dimension %s: %w", id, err)
			}
			dimension.Generator.BiomeSource = append(json.RawMessage(nil), dimension.Generator.BiomeSource...)
			set.dimensions[id] = dimension
		}
	}
	set.standardDensity = standardDensityDefinitions(set.resources)
	for _, required := range []string{"minecraft:overworld", "minecraft:nether", "minecraft:end"} {
		if _, ok := set.noiseSettings[required]; !ok {
			return nil, fmt.Errorf("worldgen: missing required noise settings %s", required)
		}
	}
	for _, required := range []string{"minecraft:overworld", "minecraft:the_nether", "minecraft:the_end"} {
		if _, ok := set.dimensionTypes[required]; !ok {
			return nil, fmt.Errorf("worldgen: missing required dimension type %s", required)
		}
	}
	if err := compileCarverResources(snapshot, set); err != nil {
		return nil, err
	}
	if err := compileStructureSets(set); err != nil {
		return nil, err
	}
	if err := compileBuriedTreasureBiomes(snapshot, set); err != nil {
		return nil, err
	}
	if err := compileNetherFossils(snapshot, set); err != nil {
		return nil, err
	}
	features, err := compileFeatureResources(snapshot, set)
	if err != nil {
		return nil, err
	}
	set.features = features
	registerBuiltinDimensions(set)
	dimensionIDs := make([]string, 0, len(set.dimensions))
	for id := range set.dimensions {
		dimensionIDs = append(dimensionIDs, id)
	}
	sort.Strings(dimensionIDs)
	for _, id := range dimensionIDs {
		dimension := set.dimensions[id]
		if err := validateDimension(id, dimension, set); err != nil {
			return nil, err
		}
		if err := validateDimensionBiomeSource(id, dimension, set); err != nil {
			return nil, err
		}
	}
	validationCompiler, err := newDensityCompiler(set, "minecraft:overworld", 0)
	if err != nil {
		return nil, err
	}
	densityIDs := make([]string, 0, len(set.resources["density_function"]))
	for id := range set.resources["density_function"] {
		densityIDs = append(densityIDs, id)
	}
	sort.Strings(densityIDs)
	for _, id := range densityIDs {
		if _, err := validationCompiler.Compile(id); err != nil {
			return nil, err
		}
	}
	for _, settingsID := range []string{"minecraft:overworld", "minecraft:nether", "minecraft:end"} {
		compiler, err := newDensityCompiler(set, settingsID, 0)
		if err != nil {
			return nil, err
		}
		var router map[string]json.RawMessage
		if err := json.Unmarshal(set.noiseSettings[settingsID].NoiseRouter, &router); err != nil {
			return nil, fmt.Errorf("worldgen noise settings %s: invalid noise router: %w", settingsID, err)
		}
		names := make([]string, 0, len(router))
		for name := range router {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			data := router[name]
			if _, err := compiler.CompileInline(data); err != nil {
				return nil, fmt.Errorf("worldgen noise settings %s router %s: %w", settingsID, name, err)
			}
		}
	}
	return set, nil
}

func validateDimensionBiomeSource(id string, dimension Dimension, resources *ResourceSet) error {
	compiler, err := newDensityCompiler(resources, dimension.Generator.Settings, 0)
	if err != nil {
		return fmt.Errorf("dimension %s biome source: %w", id, err)
	}
	var router map[string]json.RawMessage
	settings := resources.noiseSettings[dimension.Generator.Settings]
	if err := json.Unmarshal(settings.NoiseRouter, &router); err != nil {
		return fmt.Errorf("dimension %s biome source: decode noise router: %w", id, err)
	}
	biomes, err := compileBiomeSource(resources, dimension.Generator.BiomeSource, router, compiler, 0)
	if err != nil {
		return fmt.Errorf("dimension %s biome source: %w", id, err)
	}
	initial, err := compiler.CompileInline(router["initial_density_without_jaggedness"])
	if err != nil {
		return fmt.Errorf("dimension %s surface: compile initial density: %w", id, err)
	}
	defaultBlock, err := block.BlockStateFromProperties(settings.DefaultBlock.Name, settings.DefaultBlock.Properties)
	if err != nil {
		return fmt.Errorf("dimension %s surface: default block: %w", id, err)
	}
	if _, err := compileSurfaceSystem(resources, settings, compiler, 0, defaultBlock.ID(), biomes, initial); err != nil {
		return fmt.Errorf("dimension %s surface: %w", id, err)
	}
	return nil
}

func validateNoiseParameters(id string, parameters noise.Parameters) error {
	if len(parameters.Amplitudes) == 0 {
		return fmt.Errorf("worldgen noise %s: no amplitudes", id)
	}
	for _, amplitude := range parameters.Amplitudes {
		if math.IsNaN(amplitude) || math.IsInf(amplitude, 0) {
			return fmt.Errorf("worldgen noise %s: non-finite amplitude", id)
		}
	}
	return nil
}

func validateNoiseSettings(id string, settings NoiseSettings) error {
	shape := settings.Noise
	if shape.MinY%16 != 0 || shape.Height <= 0 || shape.Height%16 != 0 || shape.MinY+shape.Height > 2032 || shape.MinY < -2032 {
		return fmt.Errorf("worldgen noise settings %s: invalid vertical shape", id)
	}
	if shape.SizeHorizontal < 1 || shape.SizeHorizontal > 4 || shape.SizeVertical < 1 || shape.SizeVertical > 4 {
		return fmt.Errorf("worldgen noise settings %s: invalid noise cell size", id)
	}
	if settings.DefaultBlock.Name == "" || settings.DefaultFluid.Name == "" || len(settings.NoiseRouter) == 0 || len(settings.SurfaceRule) == 0 {
		return fmt.Errorf("worldgen noise settings %s: incomplete settings", id)
	}
	return nil
}

func validateDimensionType(id string, dimensionType DimensionType) error {
	if dimensionType.MinY%16 != 0 || dimensionType.Height < 16 || dimensionType.Height%16 != 0 || dimensionType.LogicalHeight <= 0 || dimensionType.LogicalHeight > dimensionType.Height {
		return fmt.Errorf("dimension type %s: invalid vertical bounds", id)
	}
	if dimensionType.MinY < -2032 || dimensionType.MinY+dimensionType.Height > 2032 {
		return fmt.Errorf("dimension type %s: bounds outside supported range", id)
	}
	if dimensionType.CoordinateScale < 1e-5 || dimensionType.CoordinateScale > 3e7 || math.IsNaN(dimensionType.CoordinateScale) || math.IsInf(dimensionType.CoordinateScale, 0) {
		return fmt.Errorf("dimension type %s: invalid coordinate scale", id)
	}
	if dimensionType.Effects == "" || dimensionType.Infiniburn == "" {
		return fmt.Errorf("dimension type %s: incomplete definition", id)
	}
	return nil
}

func registerBuiltinDimensions(resources *ResourceSet) {
	builtins := map[string]Dimension{
		"minecraft:overworld": {
			Type:      "minecraft:overworld",
			Generator: DimensionGenerator{Type: "minecraft:noise", Settings: "minecraft:overworld", BiomeSource: json.RawMessage(`{"type":"minecraft:multi_noise","preset":"minecraft:overworld"}`)},
		},
		"minecraft:the_nether": {
			Type:      "minecraft:the_nether",
			Generator: DimensionGenerator{Type: "minecraft:noise", Settings: "minecraft:nether", BiomeSource: json.RawMessage(`{"type":"minecraft:multi_noise","preset":"minecraft:nether"}`)},
		},
		"minecraft:the_end": {
			Type:      "minecraft:the_end",
			Generator: DimensionGenerator{Type: "minecraft:noise", Settings: "minecraft:end", BiomeSource: json.RawMessage(`{"type":"minecraft:the_end"}`)},
		},
	}
	for id, definition := range builtins {
		if _, exists := resources.dimensions[id]; !exists {
			resources.dimensions[id] = definition
		}
	}
}

func validateDimension(id string, dimension Dimension, resources *ResourceSet) error {
	if _, ok := resources.dimensionTypes[dimension.Type]; !ok {
		return fmt.Errorf("dimension %s: unknown dimension type %s", id, dimension.Type)
	}
	if dimension.Generator.Type != "minecraft:noise" {
		return fmt.Errorf("dimension %s: unsupported generator %s", id, dimension.Generator.Type)
	}
	if _, ok := resources.noiseSettings[dimension.Generator.Settings]; !ok {
		return fmt.Errorf("dimension %s: unknown noise settings %s", id, dimension.Generator.Settings)
	}
	if len(dimension.Generator.BiomeSource) == 0 || !json.Valid(dimension.Generator.BiomeSource) {
		return fmt.Errorf("dimension %s: invalid biome source", id)
	}
	return nil
}

func registryIDFromPath(name string) (string, string, bool) {
	parts := strings.Split(name, "/")
	if len(parts) < 4 || parts[0] != "data" || !strings.HasSuffix(parts[len(parts)-1], ".json") {
		return "", "", false
	}
	if parts[2] == "worldgen" {
		if len(parts) < 5 {
			return "", "", false
		}
		local := strings.TrimSuffix(strings.Join(parts[4:], "/"), ".json")
		if parts[1] == "" || parts[3] == "" || local == "" {
			return "", "", false
		}
		return parts[3], parts[1] + ":" + local, true
	}
	if parts[2] != "dimension" && parts[2] != "dimension_type" {
		return "", "", false
	}
	local := strings.TrimSuffix(strings.Join(parts[3:], "/"), ".json")
	if parts[1] == "" || local == "" {
		return "", "", false
	}
	return parts[2], parts[1] + ":" + local, true
}
