package worldgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	generatedbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/world/worldgen/climate"
	"github.com/vitismc/vitis/internal/world/worldgen/density"
)

type biomeSource interface {
	sample(quartX, quartY, quartZ int) int32
}

type cachedBiomeSource struct {
	source                 biomeSource
	evaluation             *density.Evaluation
	values                 map[biomeQuartPosition]int32
	climateCursor          climate.Cursor
	quartX, quartY, quartZ int
	quartHeight            int
	grid                   []int32
	climateSamples         uint64
	climateComputations    uint64
	climateColumns         map[[2]int]struct{}
	columns                [36]biomeClimateColumn
}

// biomeClimateColumn owns horizontal samples and search state for one quart column.
type biomeClimateColumn struct {
	target climate.Target
	cursor climate.Cursor
	set    bool
}

func newCachedBiomeSource(source biomeSource) *cachedBiomeSource {
	return &cachedBiomeSource{
		source: source, evaluation: density.NewEvaluation(),
		values: make(map[biomeQuartPosition]int32, 1024),
	}
}

// reset clears per-chunk climate and biome results before pool reuse.
func (s *cachedBiomeSource) reset(source biomeSource) {
	s.source = source
	s.evaluation.ResetChunk(0, 0, 0, 0)
	clear(s.values)
	s.climateCursor.Reset()
	for index := range s.columns {
		s.columns[index].cursor.Reset()
		s.columns[index].set = false
	}
	s.quartHeight = 0
	s.climateSamples = 0
	s.climateComputations = 0
	clear(s.climateColumns)
}

// beginChunk prepares dense biome and horizontal climate caches for a chunk column.
func (s *cachedBiomeSource) beginChunk(baseX, baseZ, minY, height int) {
	s.evaluation.ResetChunk(baseX, minY, baseZ, height)
	s.quartX, s.quartY, s.quartZ = baseX>>2, minY>>2, baseZ>>2
	s.quartHeight = height >> 2
	length := 4 * 4 * s.quartHeight
	if cap(s.grid) < length {
		s.grid = make([]int32, length)
	} else {
		s.grid = s.grid[:length]
	}
	for index := range s.grid {
		s.grid[index] = -1
	}
}

// gridIndex maps a chunk-local quart position into dense storage.
func (s *cachedBiomeSource) gridIndex(x, y, z int) (int, bool) {
	dx, dy, dz := x-s.quartX, y-s.quartY, z-s.quartZ
	return (dx*4+dz)*s.quartHeight + dy, s.quartHeight > 0 && dx >= 0 && dx < 4 && dz >= 0 && dz < 4 && dy >= 0 && dy < s.quartHeight
}

func (s *cachedBiomeSource) sample(quartX, quartY, quartZ int) int32 {
	index, dense := s.gridIndex(quartX, quartY, quartZ)
	if dense && s.grid[index] >= 0 {
		return s.grid[index]
	}
	position := biomeQuartPosition{x: quartX, y: quartY, z: quartZ}
	if !dense {
		if biome, ok := s.values[position]; ok {
			return biome
		}
	}
	var biome int32
	if climateSource, ok := s.source.(*climateBiomeSource); ok {
		if s.climateColumns != nil {
			s.climateSamples++
			s.climateColumns[[2]int{quartX, quartZ}] = struct{}{}
		}
		dx, dz := quartX-s.quartX, quartZ-s.quartZ
		if s.quartHeight > 0 && dx >= -1 && dx <= 4 && dz >= -1 && dz <= 4 {
			column := &s.columns[(dx+1)*6+dz+1]
			position := density.Position{X: quartX * 4, Y: quartY * 4, Z: quartZ * 4, Evaluation: s.evaluation}
			if climateSource.horizontal && column.set {
				column.target[4] = climate.Quantize(float32(climateSource.functions[4].Sample(position)))
			} else {
				if s.climateColumns != nil {
					s.climateComputations++
				}
				column.target = climateSource.target(position)
				column.set = true
			}
			biome = climateSource.index.FindBiomeID(column.target, &column.cursor)
		} else {
			if s.climateColumns != nil {
				s.climateComputations++
			}
			biome = climateSource.sampleWithCursor(quartX, quartY, quartZ, s.evaluation, &s.climateCursor)
		}
	} else {
		biome = s.source.sample(quartX, quartY, quartZ)
	}
	if dense {
		s.grid[index] = biome
	} else {
		s.values[position] = biome
	}
	return biome
}

type fixedBiomeSource struct{ biome int32 }

func (s fixedBiomeSource) sample(_, _, _ int) int32 { return s.biome }

type climateBiomeSource struct {
	index      *climate.Index
	functions  [6]density.Function
	horizontal bool
}

// possibleBiomeNames returns the source's distinct biomes in encounter order.
func possibleBiomeNames(source biomeSource) []string {
	switch source := source.(type) {
	case fixedBiomeSource:
		if biome := generatedbiome.BiomeByID(source.biome); biome != nil {
			return []string{biome.Name}
		}
	case *climateBiomeSource:
		return source.index.Biomes()
	case endBiomeSource:
		return []string{"minecraft:the_end", "minecraft:end_highlands", "minecraft:end_midlands", "minecraft:small_end_islands", "minecraft:end_barrens"}
	case checkerboardBiomeSource:
		result := make([]string, 0, len(source.biomes))
		seen := make(map[string]struct{}, len(source.biomes))
		for _, id := range source.biomes {
			biome := generatedbiome.BiomeByID(id)
			if biome == nil {
				continue
			}
			if _, ok := seen[biome.Name]; !ok {
				result = append(result, biome.Name)
				seen[biome.Name] = struct{}{}
			}
		}
		return result
	}
	return nil
}

func (s *climateBiomeSource) sample(quartX, quartY, quartZ int) int32 {
	return s.sampleWithEvaluation(quartX, quartY, quartZ, nil)
}

func (s *climateBiomeSource) sampleWithEvaluation(quartX, quartY, quartZ int, evaluation *density.Evaluation) int32 {
	return s.sampleWithCursor(quartX, quartY, quartZ, evaluation, nil)
}

// sampleWithCursor evaluates a climate position with reusable search state.
func (s *climateBiomeSource) sampleWithCursor(quartX, quartY, quartZ int, evaluation *density.Evaluation, cursor *climate.Cursor) int32 {
	position := density.Position{X: quartX * 4, Y: quartY * 4, Z: quartZ * 4, Evaluation: evaluation}
	return s.index.FindBiomeID(s.target(position), cursor)
}

// target samples the complete climate vector for the general biome-source path.
func (s *climateBiomeSource) target(position density.Position) climate.Target {
	return climate.NewTarget(
		s.functions[0].Sample(position), s.functions[1].Sample(position),
		s.functions[2].Sample(position), s.functions[3].Sample(position),
		s.functions[4].Sample(position), s.functions[5].Sample(position),
	)
}

type endBiomeSource struct{ erosion density.Function }

func (s endBiomeSource) sample(quartX, quartY, quartZ int) int32 {
	sectionX, sectionZ := floorDiv(quartX, 4), floorDiv(quartZ, 4)
	if int64(sectionX)*int64(sectionX)+int64(sectionZ)*int64(sectionZ) <= 4096 {
		return generatedbiome.BiomeTheEnd
	}
	position := density.Position{X: (sectionX*2 + 1) * 8, Y: quartY * 4, Z: (sectionZ*2 + 1) * 8}
	erosion := s.erosion.Sample(position)
	switch {
	case erosion > 0.25:
		return generatedbiome.BiomeEndHighlands
	case erosion >= -0.0625:
		return generatedbiome.BiomeEndMidlands
	case erosion < -0.21875:
		return generatedbiome.BiomeSmallEndIslands
	default:
		return generatedbiome.BiomeEndBarrens
	}
}

type checkerboardBiomeSource struct {
	biomes   []int32
	bitShift uint
}

func (s checkerboardBiomeSource) sample(quartX, _, quartZ int) int32 {
	index := floorMod((quartX>>s.bitShift)+(quartZ>>s.bitShift), len(s.biomes))
	return s.biomes[index]
}

type biomeSourceDefinition struct {
	Type   string            `json:"type"`
	Biome  string            `json:"biome"`
	Preset string            `json:"preset"`
	Biomes []json.RawMessage `json:"biomes"`
	Scale  *int              `json:"scale"`
}

type directBiomeEntry struct {
	Parameters directClimateParameters `json:"parameters"`
	Biome      string                  `json:"biome"`
}

type directClimateParameters struct {
	Temperature     json.RawMessage `json:"temperature"`
	Humidity        json.RawMessage `json:"humidity"`
	Continentalness json.RawMessage `json:"continentalness"`
	Erosion         json.RawMessage `json:"erosion"`
	Depth           json.RawMessage `json:"depth"`
	Weirdness       json.RawMessage `json:"weirdness"`
	Offset          float32         `json:"offset"`
}

func compileBiomeSource(resources *ResourceSet, definition json.RawMessage, router map[string]json.RawMessage, compiler *density.Compiler, fallback int32) (biomeSource, error) {
	if len(definition) == 0 {
		return fixedBiomeSource{biome: fallback}, nil
	}
	var decoded biomeSourceDefinition
	if err := json.Unmarshal(definition, &decoded); err != nil {
		return nil, fmt.Errorf("decode biome source: %w", err)
	}
	switch decoded.Type {
	case "minecraft:fixed":
		id, err := biomeID(decoded.Biome)
		if err != nil {
			return nil, err
		}
		return fixedBiomeSource{biome: id}, nil
	case "minecraft:the_end":
		erosion, err := compiler.CompileInline(router["erosion"])
		if err != nil {
			return nil, fmt.Errorf("compile end biome erosion: %w", err)
		}
		return endBiomeSource{erosion: erosion}, nil
	case "minecraft:multi_noise":
		return compileMultiNoiseBiomeSource(resources, decoded, router, compiler)
	case "minecraft:checkerboard":
		return compileCheckerboardBiomeSource(decoded)
	default:
		return nil, fmt.Errorf("unsupported biome source %q", decoded.Type)
	}
}

func compileMultiNoiseBiomeSource(resources *ResourceSet, decoded biomeSourceDefinition, router map[string]json.RawMessage, compiler *density.Compiler) (biomeSource, error) {
	var index *climate.Index
	var err error
	if decoded.Preset != "" {
		preset := decoded.Preset
		if raw, ok := resources.resources["multi_noise_biome_source_parameter_list"][preset]; ok {
			var reference struct {
				Preset string `json:"preset"`
			}
			if decodeErr := json.Unmarshal(raw, &reference); decodeErr != nil || reference.Preset == "" {
				return nil, fmt.Errorf("decode biome preset %s", preset)
			}
			preset = reference.Preset
		}
		index, err = climate.NewPreset(preset)
	} else {
		index, err = compileDirectClimateIndex(decoded.Biomes)
	}
	if err != nil {
		return nil, err
	}
	names := [...]string{"temperature", "vegetation", "continents", "erosion", "depth", "ridges"}
	source := &climateBiomeSource{index: index}
	if resources.standardTerrainSettings("minecraft:overworld") {
		var standardRouter map[string]json.RawMessage
		if err := json.Unmarshal(resources.noiseSettings["minecraft:overworld"].NoiseRouter, &standardRouter); err != nil {
			return nil, err
		}
		source.horizontal = true
		for _, dimension := range []int{0, 1, 2, 3, 5} {
			source.horizontal = source.horizontal && bytes.Equal(router[names[dimension]], standardRouter[names[dimension]])
		}
	}
	for i, name := range names {
		function, compileErr := compiler.CompileInline(router[name])
		if compileErr != nil {
			return nil, fmt.Errorf("compile biome router %s: %w", name, compileErr)
		}
		source.functions[i] = function
	}
	return source, nil
}

func compileDirectClimateIndex(rawEntries []json.RawMessage) (*climate.Index, error) {
	points := make([]climate.Point, len(rawEntries))
	for i, raw := range rawEntries {
		var entry directBiomeEntry
		if err := json.Unmarshal(raw, &entry); err != nil {
			return nil, fmt.Errorf("decode biome entry %d: %w", i, err)
		}
		if _, err := biomeID(entry.Biome); err != nil {
			return nil, err
		}
		parameters := [...]json.RawMessage{
			entry.Parameters.Temperature, entry.Parameters.Humidity, entry.Parameters.Continentalness,
			entry.Parameters.Erosion, entry.Parameters.Depth, entry.Parameters.Weirdness,
		}
		for dimension, rawParameter := range parameters {
			parameter, err := decodeClimateParameter(rawParameter)
			if err != nil {
				return nil, fmt.Errorf("biome entry %d parameter %d: %w", i, dimension, err)
			}
			points[i].Parameters[dimension] = parameter
		}
		if entry.Parameters.Offset < 0 || entry.Parameters.Offset > 1 || math.IsNaN(float64(entry.Parameters.Offset)) {
			return nil, fmt.Errorf("biome entry %d: invalid offset", i)
		}
		offset := climate.Quantize(entry.Parameters.Offset)
		points[i].Parameters[6] = climate.Parameter{Min: offset, Max: offset}
		points[i].Biome = entry.Biome
	}
	return climate.NewIndex(points)
}

func decodeClimateParameter(raw json.RawMessage) (climate.Parameter, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return climate.Parameter{}, err
	}
	switch value := value.(type) {
	case json.Number:
		parsed, err := strconvFloat32(value.String())
		if err != nil {
			return climate.Parameter{}, err
		}
		return climate.Span(parsed, parsed)
	case map[string]any:
		minimum, minOK := value["min"].(json.Number)
		maximum, maxOK := value["max"].(json.Number)
		if !minOK || !maxOK {
			return climate.Parameter{}, fmt.Errorf("interval requires min and max")
		}
		minValue, err := strconvFloat32(minimum.String())
		if err != nil {
			return climate.Parameter{}, err
		}
		maxValue, err := strconvFloat32(maximum.String())
		if err != nil {
			return climate.Parameter{}, err
		}
		return climate.Span(minValue, maxValue)
	default:
		return climate.Parameter{}, fmt.Errorf("expected number or interval")
	}
}

func compileCheckerboardBiomeSource(decoded biomeSourceDefinition) (biomeSource, error) {
	if len(decoded.Biomes) == 0 {
		return nil, fmt.Errorf("checkerboard biome source has no biomes")
	}
	ids := make([]int32, len(decoded.Biomes))
	for i, raw := range decoded.Biomes {
		var name string
		if err := json.Unmarshal(raw, &name); err != nil {
			return nil, fmt.Errorf("checkerboard biome %d: %w", i, err)
		}
		id, err := biomeID(name)
		if err != nil {
			return nil, err
		}
		ids[i] = id
	}
	scale := 2
	if decoded.Scale != nil {
		scale = *decoded.Scale
	}
	if scale < 0 || scale > 62 {
		return nil, fmt.Errorf("checkerboard scale %d outside 0..62", scale)
	}
	return checkerboardBiomeSource{biomes: ids, bitShift: uint(scale + 2)}, nil
}

func biomeID(name string) (int32, error) {
	id := generatedbiome.BiomeIDByName(name)
	if id < 0 {
		return 0, fmt.Errorf("unknown biome %s", name)
	}
	return id, nil
}

func strconvFloat32(value string) (float32, error) {
	parsed, err := strconv.ParseFloat(value, 32)
	return float32(parsed), err
}

func floorMod(value, divisor int) int {
	result := value % divisor
	if result < 0 {
		result += divisor
	}
	return result
}
