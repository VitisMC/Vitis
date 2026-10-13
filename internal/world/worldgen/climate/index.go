// Package climate implements deterministic multi-noise biome selection.
package climate

import (
	"fmt"
	"math"
	"sort"

	generatedbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	generated "github.com/vitismc/vitis/internal/data/generated/climate"
)

const parameterCount = 7

// Parameter is an inclusive interval in quantized climate space.
type Parameter struct {
	Min int64
	Max int64
}

// Span creates a validated parameter interval from float coordinates.
func Span(minimum, maximum float32) (Parameter, error) {
	if minimum > maximum || minimum < -2 || maximum > 2 || float32IsInvalid(minimum) || float32IsInvalid(maximum) {
		return Parameter{}, fmt.Errorf("invalid climate interval [%v,%v]", minimum, maximum)
	}
	return Parameter{Min: Quantize(minimum), Max: Quantize(maximum)}, nil
}

// Quantize converts a climate coordinate to its fixed point representation.
func Quantize(value float32) int64 {
	return int64(value * 10000)
}

func float32IsInvalid(value float32) bool {
	return math.IsNaN(float64(value)) || math.IsInf(float64(value), 0)
}

func (p Parameter) distance(value int64) int64 {
	if value > p.Max {
		return value - p.Max
	}
	if value < p.Min {
		return p.Min - value
	}
	return 0
}

func (p Parameter) span(other Parameter) Parameter {
	return Parameter{Min: min(p.Min, other.Min), Max: max(p.Max, other.Max)}
}

// Target is one sampled point in six dimensional climate space.
type Target [6]int64

// NewTarget quantizes six sampled climate values with float precision.
func NewTarget(temperature, humidity, continentalness, erosion, depth, weirdness float64) Target {
	return Target{
		Quantize(float32(temperature)), Quantize(float32(humidity)),
		Quantize(float32(continentalness)), Quantize(float32(erosion)),
		Quantize(float32(depth)), Quantize(float32(weirdness)),
	}
}

// Point maps one climate volume to a biome registry name.
type Point struct {
	Parameters [parameterCount]Parameter
	Biome      string
}

// Index is an immutable spatial index for climate parameter points.
type Index struct {
	root   *node
	len    int
	biomes []string
	flat   []flatNode
	groups []depthGroup
}

// NewIndex builds an immutable climate lookup index.
func NewIndex(points []Point) (*Index, error) {
	if len(points) == 0 {
		return nil, fmt.Errorf("climate index requires at least one point")
	}
	nodes := make([]*node, len(points))
	biomes := make([]string, 0, len(points))
	seenBiomes := make(map[string]struct{}, len(points))
	for i, point := range points {
		if point.Biome == "" {
			return nil, fmt.Errorf("climate point %d has no biome", i)
		}
		for dimension, parameter := range point.Parameters {
			if parameter.Min > parameter.Max {
				return nil, fmt.Errorf("climate point %d dimension %d has reversed interval", i, dimension)
			}
		}
		nodes[i] = &node{parameters: point.Parameters, biome: point.Biome, leaf: true}
		if _, seen := seenBiomes[point.Biome]; !seen {
			biomes = append(biomes, point.Biome)
			seenBiomes[point.Biome] = struct{}{}
		}
	}
	index := &Index{root: build(nodes), len: len(points), biomes: biomes}
	index.flatten(index.root)
	index.buildDepthGroups()
	return index, nil
}

// Biomes returns biome names in climate parameter order.
func (i *Index) Biomes() []string {
	if i == nil {
		return nil
	}
	return append([]string(nil), i.biomes...)
}

// NewPreset builds a built in multi-noise preset.
func NewPreset(id string) (*Index, error) {
	var source []generated.Point
	switch id {
	case "minecraft:overworld":
		source = generated.Overworld[:]
	case "minecraft:nether":
		source = generated.Nether[:]
	default:
		return nil, fmt.Errorf("unknown climate preset %s", id)
	}
	points := make([]Point, len(source))
	for i, raw := range source {
		points[i].Biome = raw.Biome
		for dimension, parameter := range raw.Parameters {
			points[i].Parameters[dimension] = Parameter{Min: parameter.Min, Max: parameter.Max}
		}
	}
	return NewIndex(points)
}

// Len returns the number of indexed parameter points.
func (i *Index) Len() int {
	if i == nil {
		return 0
	}
	return i.len
}

// Find returns the closest biome for a climate target.
func (i *Index) Find(target Target) string {
	return i.FindWithCursor(target, nil)
}

// Cursor holds the previous result for a sequence of nearby climate queries.
type Cursor struct {
	last             *node
	index            *Index
	flat             int32
	set              bool
	horizontalTarget [5]int64
	horizontal       []int64
	stamps           []uint32
	generation       uint32
	groupTarget      [5]int64
	groupCandidates  [8]depthCandidate
	groupsSet        bool
}

// Reset clears search order state while retaining independently owned column buffers.
func (c *Cursor) Reset() {
	c.last = nil
	c.index = nil
	c.set = false
	c.groupsSet = false
}

// FindWithCursor returns the closest biome and uses a cursor to bound the search.
func (i *Index) FindWithCursor(target Target, cursor *Cursor) string {
	if i != nil && len(i.flat) != 0 {
		if cursor != nil && len(i.groups) > 0 {
			return i.flat[i.findDepthGroups(target, cursor)].biome
		}
		return i.flat[i.findFlat(target, cursor)].biome
	}
	if i == nil || i.root == nil {
		return ""
	}
	values := [parameterCount]int64{target[0], target[1], target[2], target[3], target[4], target[5], 0}
	limit := int64(math.MaxInt64)
	if cursor != nil && cursor.index == i && cursor.last != nil {
		distance := cursor.last.distance(values, math.MaxInt64)
		if distance < math.MaxInt64 {
			limit = distance + 1
		}
	}
	result, _ := i.root.search(values, nil, limit)
	if result == nil {
		return ""
	}
	if cursor != nil {
		cursor.last = result
		cursor.index = i
	}
	return result.biome
}

// FindBiomeID returns the registry ID directly for a validated biome source.
func (i *Index) FindBiomeID(target Target, cursor *Cursor) int32 {
	if i == nil || len(i.flat) == 0 {
		return -1
	}
	if cursor != nil && len(i.groups) > 0 {
		return i.flat[i.findDepthGroups(target, cursor)].biomeID
	}
	return i.flat[i.findFlat(target, cursor)].biomeID
}

// flatNode keeps climate bounds and ordered children together in contiguous storage.
type flatNode struct {
	parameters [parameterCount]Parameter
	children   [6]int32
	count      uint8
	biome      string
	biomeID    int32
}

// flatten preserves the original tree's child order and biome tie resolution.
func (i *Index) flatten(n *node) int32 {
	index := int32(len(i.flat))
	if n.leaf {
		n.rank = index
	}
	i.flat = append(i.flat, flatNode{parameters: n.parameters, biome: n.biome, biomeID: generatedbiome.BiomeIDByName(n.biome)})
	for offset, child := range n.children {
		childIndex := i.flatten(child)
		i.flat[index].children[offset] = childIndex
	}
	i.flat[index].count = uint8(len(n.children))
	return index
}

// distance rejects a node as soon as its lower distance bound cannot improve the result.
func (n *flatNode) distance(target *[parameterCount]int64, limit int64) int64 {
	var total int64
	for dimension := range n.parameters {
		parameter, value := n.parameters[dimension], target[dimension]
		var distance int64
		if value > parameter.Max {
			distance = value - parameter.Max
		} else if value < parameter.Min {
			distance = parameter.Min - value
		}
		total += distance * distance
		if total >= limit {
			return total
		}
	}
	return total
}

// findFlat traverses ordered compact nodes with a per-query stack and cursor bound.
func (i *Index) findFlat(target Target, cursor *Cursor) int32 {
	values := [parameterCount]int64{target[0], target[1], target[2], target[3], target[4], target[5], 0}
	if cursor != nil {
		horizontal := [5]int64{target[0], target[1], target[2], target[3], target[5]}
		if cursor.index != i || cursor.horizontalTarget != horizontal || cursor.generation == 0 {
			cursor.horizontalTarget = horizontal
			cursor.generation++
			if len(cursor.horizontal) != len(i.flat) {
				cursor.horizontal = make([]int64, len(i.flat))
				cursor.stamps = make([]uint32, len(i.flat))
			}
			if cursor.generation == 0 {
				clear(cursor.stamps)
				cursor.generation = 1
			}
		}
	}
	limit := int64(math.MaxInt64)
	if cursor != nil && cursor.index == i && cursor.set {
		distance := i.columnDistance(cursor.flat, &values, limit, cursor)
		if distance < limit {
			limit = distance + 1
		}
	}
	best := int32(0)
	var stack [64]int32
	stack[0] = 0
	length := 1
	for length > 0 {
		length--
		index := stack[length]
		n := &i.flat[index]
		distance := i.columnDistance(index, &values, limit, cursor)
		if distance >= limit {
			continue
		}
		if n.count == 0 {
			best, limit = index, distance
			continue
		}
		for offset := int(n.count) - 1; offset >= 0; offset-- {
			stack[length] = n.children[offset]
			length++
		}
	}
	if cursor != nil {
		cursor.index, cursor.flat, cursor.set = i, best, true
	}
	return best
}

// columnDistance reuses six invariant climate dimensions for nearby height queries.
func (i *Index) columnDistance(index int32, target *[parameterCount]int64, limit int64, cursor *Cursor) int64 {
	n := &i.flat[index]
	if cursor == nil {
		return n.distance(target, limit)
	}
	var horizontal int64
	if cursor.stamps[index] == cursor.generation {
		horizontal = cursor.horizontal[index]
	} else {
		for _, dimension := range [...]uint8{0, 1, 2, 3, 5, 6} {
			parameter, value := n.parameters[dimension], target[dimension]
			var distance int64
			if value > parameter.Max {
				distance = value - parameter.Max
			} else if value < parameter.Min {
				distance = parameter.Min - value
			}
			horizontal += distance * distance
		}
		cursor.horizontal[index], cursor.stamps[index] = horizontal, cursor.generation
	}
	if horizontal >= limit {
		return horizontal
	}
	parameter, value := n.parameters[4], target[4]
	var distance int64
	if value > parameter.Max {
		distance = value - parameter.Max
	} else if value < parameter.Min {
		distance = parameter.Min - value
	}
	return horizontal + distance*distance
}

type node struct {
	parameters [parameterCount]Parameter
	children   []*node
	biome      string
	leaf       bool
	rank       int32
}

func build(nodes []*node) *node {
	if len(nodes) == 1 {
		return nodes[0]
	}
	if len(nodes) <= 6 {
		sort.SliceStable(nodes, func(i, j int) bool {
			return centerMagnitude(nodes[i]) < centerMagnitude(nodes[j])
		})
		return subtree(nodes)
	}
	bestCost := int64(math.MaxInt64)
	bestDimension := -1
	var bestBuckets []*node
	for dimension := 0; dimension < parameterCount; dimension++ {
		sortNodes(nodes, dimension, false)
		buckets := bucketize(nodes)
		var cost int64
		for _, bucket := range buckets {
			for _, parameter := range bucket.parameters {
				cost += abs(parameter.Max - parameter.Min)
			}
		}
		if cost < bestCost {
			bestCost = cost
			bestDimension = dimension
			bestBuckets = buckets
		}
	}
	sortNodes(bestBuckets, bestDimension, true)
	children := make([]*node, len(bestBuckets))
	for i, bucket := range bestBuckets {
		children[i] = build(append([]*node(nil), bucket.children...))
	}
	return subtree(children)
}

func subtree(children []*node) *node {
	result := &node{children: append([]*node(nil), children...)}
	result.parameters = children[0].parameters
	for _, child := range children[1:] {
		for dimension := range result.parameters {
			result.parameters[dimension] = result.parameters[dimension].span(child.parameters[dimension])
		}
	}
	return result
}

func bucketize(nodes []*node) []*node {
	bucketSize := int(math.Pow(6, math.Floor(math.Log(float64(len(nodes))-0.01)/math.Log(6))))
	if bucketSize < 1 {
		bucketSize = 1
	}
	result := make([]*node, 0, (len(nodes)+bucketSize-1)/bucketSize)
	for start := 0; start < len(nodes); start += bucketSize {
		end := min(start+bucketSize, len(nodes))
		result = append(result, subtree(nodes[start:end]))
	}
	return result
}

func sortNodes(nodes []*node, firstDimension int, absolute bool) {
	sort.SliceStable(nodes, func(i, j int) bool {
		for offset := 0; offset < parameterCount; offset++ {
			dimension := (firstDimension + offset) % parameterCount
			left := (nodes[i].parameters[dimension].Min + nodes[i].parameters[dimension].Max) / 2
			right := (nodes[j].parameters[dimension].Min + nodes[j].parameters[dimension].Max) / 2
			if absolute {
				left, right = abs(left), abs(right)
			}
			if left != right {
				return left < right
			}
		}
		return false
	})
}

func centerMagnitude(value *node) int64 {
	var total int64
	for _, parameter := range value.parameters {
		total += abs((parameter.Min + parameter.Max) / 2)
	}
	return total
}

func (n *node) distance(target [parameterCount]int64, limit int64) int64 {
	var total int64
	for dimension := 0; dimension < parameterCount; dimension++ {
		parameter, value := n.parameters[dimension], target[dimension]
		var distance int64
		if value > parameter.Max {
			distance = value - parameter.Max
		} else if value < parameter.Min {
			distance = parameter.Min - value
		}
		total += distance * distance
		if total >= limit {
			return total
		}
	}
	return total
}

func (n *node) search(target [parameterCount]int64, closest *node, closestDistance int64) (*node, int64) {
	if n.leaf {
		return n, n.distance(target, closestDistance)
	}
	for _, child := range n.children {
		distance := child.distance(target, closestDistance)
		if closestDistance <= distance {
			continue
		}
		candidate, candidateDistance := child, distance
		if !child.leaf {
			candidate, candidateDistance = child.search(target, closest, closestDistance)
		}
		if candidateDistance < closestDistance {
			closest = candidate
			closestDistance = candidateDistance
		}
	}
	return closest, closestDistance
}

func abs(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
