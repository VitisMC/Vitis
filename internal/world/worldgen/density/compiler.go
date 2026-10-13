package density

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/vitismc/vitis/internal/world/worldgen/noise"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// Registry contains data-driven density and noise definitions for one seed.
type Registry struct {
	Definitions map[string]json.RawMessage
	Noises      map[string]noise.Parameters
	Seed        int64
	Legacy      bool
	CellWidth   int
	CellHeight  int
}

// Compiler resolves and validates immutable density graphs.
type Compiler struct {
	registry      Registry
	random        worldrandom.PositionalFactory
	mu            sync.Mutex
	compiled      map[string]Function
	compiling     map[string]bool
	noiseSamples  map[string]*noise.Normal
	nextCacheID   int
	interpolators []*interpolated
	standard      *standardDensity
}

// NewCompiler creates a density graph compiler for one world seed.
func NewCompiler(registry Registry) (*Compiler, error) {
	if len(registry.Definitions) == 0 {
		return nil, fmt.Errorf("create density compiler: no definitions")
	}
	if len(registry.Noises) == 0 {
		return nil, fmt.Errorf("create density compiler: no noise definitions")
	}
	if registry.CellWidth <= 0 || registry.CellHeight <= 0 {
		return nil, fmt.Errorf("create density compiler: invalid noise cell size")
	}
	definitions := make(map[string]json.RawMessage, len(registry.Definitions))
	for id, data := range registry.Definitions {
		definitions[normalizeID(id)] = append(json.RawMessage(nil), data...)
	}
	noises := make(map[string]noise.Parameters, len(registry.Noises))
	for id, parameters := range registry.Noises {
		parameters.Amplitudes = append([]float64(nil), parameters.Amplitudes...)
		noises[normalizeID(id)] = parameters
	}
	registry.Definitions = definitions
	registry.Noises = noises
	var source worldrandom.Source = worldrandom.NewXoroshiro(registry.Seed)
	if registry.Legacy {
		source = worldrandom.NewLegacy(registry.Seed)
	}
	return &Compiler{
		registry: registry, random: source.ForkPositional(),
		compiled: make(map[string]Function), compiling: make(map[string]bool), noiseSamples: make(map[string]*noise.Normal),
	}, nil
}

// Compile resolves a named density function and all transitive references.
func (c *Compiler) Compile(id string) (Function, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.compileReference(normalizeID(id))
}

// CompileInline compiles an inline density function using the same registry.
func (c *Compiler) CompileInline(data json.RawMessage) (Function, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.compile(data)
}

func (c *Compiler) compileReference(id string) (Function, error) {
	if function := c.compiled[id]; function != nil {
		return function, nil
	}
	if c.compiling[id] {
		return nil, fmt.Errorf("density function reference cycle at %s", id)
	}
	data, ok := c.registry.Definitions[id]
	if !ok {
		return nil, fmt.Errorf("unknown density function %s", id)
	}
	c.compiling[id] = true
	function, err := c.compile(data)
	delete(c.compiling, id)
	if err != nil {
		return nil, fmt.Errorf("density function %s: %w", id, err)
	}
	c.compiled[id] = function
	return function, nil
}

func (c *Compiler) compile(data json.RawMessage) (Function, error) {
	data = trimJSON(data)
	if c.standard != nil {
		if function := c.standard.roots[standardJSONKey(data)]; function != nil {
			return function, nil
		}
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty density function")
	}
	switch data[0] {
	case '"':
		var id string
		if err := json.Unmarshal(data, &id); err != nil {
			return nil, err
		}
		return c.compileReference(normalizeID(id))
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		var value float64
		if err := json.Unmarshal(data, &value); err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("invalid density constant")
		}
		return constant(value), nil
	case '{':
		return c.compileObject(data)
	default:
		return nil, fmt.Errorf("invalid density function JSON")
	}
}

func (c *Compiler) compileObject(data json.RawMessage) (Function, error) {
	var header struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, err
	}
	kind := strings.TrimPrefix(header.Type, "minecraft:")
	switch kind {
	case "add", "mul", "min", "max":
		return c.compileBinary(kind, data)
	case "abs", "square", "cube", "half_negative", "quarter_negative", "squeeze":
		return c.compileMapped(kind, data)
	case "clamp":
		return c.compileClamp(data)
	case "range_choice":
		return c.compileRangeChoice(data)
	case "noise":
		return c.compileNoise(data)
	case "shift_a", "shift_b":
		return c.compileShift(kind, data)
	case "shifted_noise":
		return c.compileShiftedNoise(data)
	case "weird_scaled_sampler":
		return c.compileWeird(data)
	case "y_clamped_gradient":
		return compileYGradient(data)
	case "old_blended_noise":
		return c.compileBlended(data)
	case "end_islands":
		islands := NewEndIslands(c.registry.Seed)
		return functionFunc(func(position Position) float64 { return islands.Sample(position.X, position.Z) }), nil
	case "spline":
		return c.compileSplineFunction(data)
	case "interpolated", "flat_cache", "cache_2d", "cache_once", "cache_all_in_cell":
		return c.compileMarker(kind, data)
	case "blend_alpha":
		return constant(1), nil
	case "blend_offset":
		return constant(0), nil
	case "blend_density":
		return c.compileArgument(data)
	default:
		return nil, fmt.Errorf("unsupported density function type %q", header.Type)
	}
}

func (c *Compiler) compileBinary(kind string, data json.RawMessage) (Function, error) {
	var value struct {
		First  json.RawMessage `json:"argument1"`
		Second json.RawMessage `json:"argument2"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	first, err := c.compile(value.First)
	if err != nil {
		return nil, err
	}
	second, err := c.compile(value.Second)
	if err != nil {
		return nil, err
	}
	first, firstMin, firstMax := unwrapBounds(first)
	second, secondMin, secondMax := unwrapBounds(second)
	switch kind {
	case "add":
		if left, ok := first.(constant); ok {
			if right, ok := second.(constant); ok {
				return constant(float64(left) + float64(right)), nil
			}
			return withBounds(functionFunc(func(position Position) float64 { return float64(left) + second.Sample(position) }), firstMin+secondMin, firstMax+secondMax), nil
		}
		if right, ok := second.(constant); ok {
			return withBounds(functionFunc(func(position Position) float64 { return first.Sample(position) + float64(right) }), firstMin+secondMin, firstMax+secondMax), nil
		}
		return withBounds(functionFunc(func(position Position) float64 { return first.Sample(position) + second.Sample(position) }), firstMin+secondMin, firstMax+secondMax), nil
	case "mul":
		products := [...]float64{firstMin * secondMin, firstMin * secondMax, firstMax * secondMin, firstMax * secondMax}
		minimum, maximum := products[0], products[0]
		for _, product := range products[1:] {
			minimum, maximum = math.Min(minimum, product), math.Max(maximum, product)
		}
		if left, ok := first.(constant); ok {
			if right, ok := second.(constant); ok {
				return constant(float64(left) * float64(right)), nil
			}
			return withBounds(functionFunc(func(position Position) float64 { return float64(left) * second.Sample(position) }), minimum, maximum), nil
		}
		if right, ok := second.(constant); ok {
			return withBounds(functionFunc(func(position Position) float64 { return first.Sample(position) * float64(right) }), minimum, maximum), nil
		}
		return withBounds(functionFunc(func(position Position) float64 { return first.Sample(position) * second.Sample(position) }), minimum, maximum), nil
	case "min":
		return withBounds(functionFunc(func(position Position) float64 {
			value := first.Sample(position)
			if value < secondMin {
				return value
			}
			return math.Min(value, second.Sample(position))
		}), math.Min(firstMin, secondMin), math.Min(firstMax, secondMax)), nil
	default:
		return withBounds(functionFunc(func(position Position) float64 {
			value := first.Sample(position)
			if value > secondMax {
				return value
			}
			return math.Max(value, second.Sample(position))
		}), math.Max(firstMin, secondMin), math.Max(firstMax, secondMax)), nil
	}
}

func (c *Compiler) compileMapped(kind string, data json.RawMessage) (Function, error) {
	input, err := c.compileArgument(data)
	if err != nil {
		return nil, err
	}
	input, inputMin, inputMax := unwrapBounds(input)
	switch kind {
	case "abs":
		minimum := 0.0
		if inputMin > 0 {
			minimum = inputMin
		} else if inputMax < 0 {
			minimum = -inputMax
		}
		return withBounds(functionFunc(func(position Position) float64 { return math.Abs(input.Sample(position)) }), minimum, math.Max(math.Abs(inputMin), math.Abs(inputMax))), nil
	case "square":
		maximum := math.Max(inputMin*inputMin, inputMax*inputMax)
		minimum := 0.0
		if inputMin > 0 {
			minimum = inputMin * inputMin
		} else if inputMax < 0 {
			minimum = inputMax * inputMax
		}
		return withBounds(functionFunc(func(position Position) float64 {
			value := input.Sample(position)
			return value * value
		}), minimum, maximum), nil
	case "cube":
		return withBounds(functionFunc(func(position Position) float64 {
			value := input.Sample(position)
			return value * value * value
		}), inputMin*inputMin*inputMin, inputMax*inputMax*inputMax), nil
	case "half_negative":
		minimum, maximum := inputMin, inputMax
		if minimum < 0 {
			minimum *= 0.5
		}
		if maximum < 0 {
			maximum *= 0.5
		}
		return withBounds(functionFunc(func(position Position) float64 {
			value := input.Sample(position)
			if value <= 0 {
				return value * 0.5
			}
			return value
		}), minimum, maximum), nil
	case "quarter_negative":
		minimum, maximum := inputMin, inputMax
		if minimum < 0 {
			minimum *= 0.25
		}
		if maximum < 0 {
			maximum *= 0.25
		}
		return withBounds(functionFunc(func(position Position) float64 {
			value := input.Sample(position)
			if value <= 0 {
				return value * 0.25
			}
			return value
		}), minimum, maximum), nil
	default:
		squeeze := func(value float64) float64 {
			value = math.Max(-1, math.Min(1, value))
			return value/2 - value*value*value/24
		}
		return withBounds(functionFunc(func(position Position) float64 {
			value := math.Max(-1, math.Min(1, input.Sample(position)))
			return value/2 - value*value*value/24
		}), squeeze(inputMin), squeeze(inputMax)), nil
	}
}

func (c *Compiler) compileArgument(data json.RawMessage) (Function, error) {
	var value struct {
		Argument json.RawMessage `json:"argument"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return c.compile(value.Argument)
}

func (c *Compiler) compileClamp(data json.RawMessage) (Function, error) {
	var value struct {
		Input json.RawMessage `json:"input"`
		Min   float64         `json:"min"`
		Max   float64         `json:"max"`
	}
	if err := json.Unmarshal(data, &value); err != nil || value.Min > value.Max {
		return nil, fmt.Errorf("invalid clamp")
	}
	input, err := c.compile(value.Input)
	if err != nil {
		return nil, err
	}
	input, inputMin, inputMax := unwrapBounds(input)
	return withBounds(functionFunc(func(position Position) float64 {
		return math.Max(value.Min, math.Min(value.Max, input.Sample(position)))
	}), math.Max(value.Min, math.Min(value.Max, inputMin)), math.Max(value.Min, math.Min(value.Max, inputMax))), nil
}

func (c *Compiler) compileRangeChoice(data json.RawMessage) (Function, error) {
	var value struct {
		Input json.RawMessage `json:"input"`
		Min   float64         `json:"min_inclusive"`
		Max   float64         `json:"max_exclusive"`
		In    json.RawMessage `json:"when_in_range"`
		Out   json.RawMessage `json:"when_out_of_range"`
	}
	if err := json.Unmarshal(data, &value); err != nil || value.Min > value.Max {
		return nil, fmt.Errorf("invalid range choice")
	}
	input, err := c.compile(value.Input)
	if err != nil {
		return nil, err
	}
	inRange, err := c.compile(value.In)
	if err != nil {
		return nil, err
	}
	outOfRange, err := c.compile(value.Out)
	if err != nil {
		return nil, err
	}
	input, _, _ = unwrapBounds(input)
	inRange, inMin, inMax := unwrapBounds(inRange)
	outOfRange, outMin, outMax := unwrapBounds(outOfRange)
	return withBounds(functionFunc(func(position Position) float64 {
		result := input.Sample(position)
		if result >= value.Min && result < value.Max {
			return inRange.Sample(position)
		}
		return outOfRange.Sample(position)
	}), math.Min(inMin, outMin), math.Max(inMax, outMax)), nil
}

func (c *Compiler) compileNoise(data json.RawMessage) (Function, error) {
	var value struct {
		Noise string `json:"noise"`
		XZ, Y float64
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(fields["noise"], &value.Noise); err != nil {
		return nil, fmt.Errorf("invalid noise reference")
	}
	if err := json.Unmarshal(fields["xz_scale"], &value.XZ); err != nil {
		return nil, fmt.Errorf("invalid xz_scale")
	}
	if err := json.Unmarshal(fields["y_scale"], &value.Y); err != nil {
		return nil, fmt.Errorf("invalid y_scale")
	}
	sampler, err := c.resolveNoise(value.Noise)
	if err != nil {
		return nil, err
	}
	maximum := sampler.MaxValue()
	if value.Y == 0 {
		function := functionFunc(func(position Position) float64 {
			return sampler.SampleXZ(float64(position.X)*value.XZ, float64(position.Z)*value.XZ)
		})
		id := c.nextCacheID
		c.nextCacheID++
		return withBounds(&Marker{ID: id, Kind: "cache_2d", Function: function}, -maximum, maximum), nil
	}
	return withBounds(functionFunc(func(position Position) float64 {
		return sampler.Sample(float64(position.X)*value.XZ, float64(position.Y)*value.Y, float64(position.Z)*value.XZ)
	}), -maximum, maximum), nil
}

func (c *Compiler) compileShift(kind string, data json.RawMessage) (Function, error) {
	var value struct {
		Noise string `json:"argument"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	sampler, err := c.resolveNoise(value.Noise)
	if err != nil {
		return nil, err
	}
	maximum := sampler.MaxValue() * 4
	if kind == "shift_a" {
		return withBounds(functionFunc(func(position Position) float64 {
			return sampler.SampleXZ(float64(position.X)*0.25, float64(position.Z)*0.25) * 4
		}), -maximum, maximum), nil
	}
	return withBounds(functionFunc(func(position Position) float64 {
		return sampler.Sample(float64(position.Z)*0.25, float64(position.X)*0.25, 0) * 4
	}), -maximum, maximum), nil
}

func (c *Compiler) compileShiftedNoise(data json.RawMessage) (Function, error) {
	var value struct {
		ShiftX, ShiftY, ShiftZ json.RawMessage
		Noise                  string
		XZ, Y                  float64
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	value.ShiftX, value.ShiftY, value.ShiftZ = fields["shift_x"], fields["shift_y"], fields["shift_z"]
	if err := json.Unmarshal(fields["noise"], &value.Noise); err != nil {
		return nil, fmt.Errorf("invalid noise reference")
	}
	if err := json.Unmarshal(fields["xz_scale"], &value.XZ); err != nil {
		return nil, fmt.Errorf("invalid xz_scale")
	}
	if err := json.Unmarshal(fields["y_scale"], &value.Y); err != nil {
		return nil, fmt.Errorf("invalid y_scale")
	}
	sx, err := c.compile(value.ShiftX)
	if err != nil {
		return nil, err
	}
	sy, err := c.compile(value.ShiftY)
	if err != nil {
		return nil, err
	}
	sz, err := c.compile(value.ShiftZ)
	if err != nil {
		return nil, err
	}
	sampler, err := c.resolveNoise(value.Noise)
	if err != nil {
		return nil, err
	}
	maximum := sampler.MaxValue()
	if value.Y == 0 {
		vertical, _, _ := unwrapBounds(sy)
		if zero, ok := vertical.(constant); ok && zero == 0 {
			return withBounds(functionFunc(func(position Position) float64 {
				x := float64(position.X)*value.XZ + sx.Sample(position)
				z := float64(position.Z)*value.XZ + sz.Sample(position)
				return sampler.SampleXZ(x, z)
			}), -maximum, maximum), nil
		}
	}
	return withBounds(functionFunc(func(position Position) float64 {
		return sampler.Sample(float64(position.X)*value.XZ+sx.Sample(position), float64(position.Y)*value.Y+sy.Sample(position), float64(position.Z)*value.XZ+sz.Sample(position))
	}), -maximum, maximum), nil
}

func (c *Compiler) compileWeird(data json.RawMessage) (Function, error) {
	var value struct {
		Input  json.RawMessage `json:"input"`
		Noise  string          `json:"noise"`
		Mapper string          `json:"rarity_value_mapper"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	input, err := c.compile(value.Input)
	if err != nil {
		return nil, err
	}
	sampler, err := c.resolveNoise(value.Noise)
	if err != nil {
		return nil, err
	}
	if value.Mapper != "type_1" && value.Mapper != "type_2" {
		return nil, fmt.Errorf("unknown rarity mapper %q", value.Mapper)
	}
	return withBounds(functionFunc(func(position Position) float64 {
		rarity := spaghettiRarity(input.Sample(position), value.Mapper)
		return rarity * math.Abs(sampler.Sample(float64(position.X)/rarity, float64(position.Y)/rarity, float64(position.Z)/rarity))
	}), 0, math.Inf(1)), nil
}

func compileYGradient(data json.RawMessage) (Function, error) {
	var value struct {
		FromY int     `json:"from_y"`
		ToY   int     `json:"to_y"`
		From  float64 `json:"from_value"`
		To    float64 `json:"to_value"`
	}
	if err := json.Unmarshal(data, &value); err != nil || value.FromY >= value.ToY {
		return nil, fmt.Errorf("invalid y gradient")
	}
	minimum, maximum := math.Min(value.From, value.To), math.Max(value.From, value.To)
	return withBounds(functionFunc(func(position Position) float64 {
		delta := (float64(position.Y) - float64(value.FromY)) / float64(value.ToY-value.FromY)
		if delta < 0 {
			delta = 0
		} else if delta > 1 {
			delta = 1
		}
		return value.From + delta*(value.To-value.From)
	}), minimum, maximum), nil
}

func (c *Compiler) compileBlended(data json.RawMessage) (Function, error) {
	var parameters noise.BlendedParameters
	if err := json.Unmarshal(data, &parameters); err != nil {
		return nil, err
	}
	var source worldrandom.Source
	if c.registry.Legacy {
		source = worldrandom.NewLegacy(c.registry.Seed)
	} else {
		source = c.random.FromHashOf("minecraft:terrain")
	}
	sampler, err := noise.NewBlended(source, parameters)
	if err != nil {
		return nil, err
	}
	maximum := sampler.MaxValue()
	id := c.nextCacheID
	c.nextCacheID++
	return withBounds(&blendedFunction{id: id, sampler: sampler}, -maximum, maximum), nil
}

func (c *Compiler) compileMarker(kind string, data json.RawMessage) (Function, error) {
	input, err := c.compileArgument(data)
	if err != nil {
		return nil, err
	}
	cacheID := c.nextCacheID
	c.nextCacheID++
	input, minimum, maximum := unwrapBounds(input)
	if kind == "interpolated" {
		width, height := c.registry.CellWidth, c.registry.CellHeight
		powerOfTwo := width&(width-1) == 0 && height&(height-1) == 0
		channel := &interpolated{
			id: cacheID, function: input, width: width, height: height,
			widthMask: width - 1, heightMask: height - 1,
			widthInverse: 1 / float64(width), heightInverse: 1 / float64(height), powerOfTwo: powerOfTwo,
		}
		c.interpolators = append(c.interpolators, channel)
		return withBounds(channel, minimum, maximum), nil
	}
	return withBounds(&Marker{ID: cacheID, Kind: kind, Function: input}, minimum, maximum), nil
}

func (c *Compiler) resolveNoise(id string) (*noise.Normal, error) {
	id = normalizeID(id)
	if sampler := c.noiseSamples[id]; sampler != nil {
		return sampler, nil
	}
	parameters, ok := c.registry.Noises[id]
	if !ok {
		return nil, fmt.Errorf("unknown noise %s", id)
	}
	var sampler *noise.Normal
	var err error
	if c.registry.Legacy && id == "minecraft:temperature" {
		sampler, err = noise.NewLegacyNormal(worldrandom.NewLegacy(c.registry.Seed), noise.Parameters{FirstOctave: -7, Amplitudes: []float64{1, 1}})
	} else if c.registry.Legacy && id == "minecraft:vegetation" {
		sampler, err = noise.NewLegacyNormal(worldrandom.NewLegacy(c.registry.Seed+1), noise.Parameters{FirstOctave: -7, Amplitudes: []float64{1, 1}})
	} else {
		sampler, err = noise.NewNormal(c.random.FromHashOf(id), parameters)
	}
	if err != nil {
		return nil, err
	}
	c.noiseSamples[id] = sampler
	return sampler, nil
}

func spaghettiRarity(value float64, kind string) float64 {
	if kind == "type_1" {
		if value < -0.5 {
			return 0.75
		}
		if value < 0 {
			return 1
		}
		if value < 0.5 {
			return 1.5
		}
		return 2
	}
	if value < -0.75 {
		return 0.5
	}
	if value < -0.5 {
		return 0.75
	}
	if value < 0.5 {
		return 1
	}
	if value < 0.75 {
		return 2
	}
	return 3
}

func normalizeID(id string) string {
	if !strings.Contains(id, ":") {
		return "minecraft:" + id
	}
	return id
}

func trimJSON(data []byte) []byte { return []byte(strings.TrimSpace(string(data))) }

func floorToMultiple(value, size int) int {
	remainder := value % size
	if remainder < 0 {
		remainder += size
	}
	return value - remainder
}
