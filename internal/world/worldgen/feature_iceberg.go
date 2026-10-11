package worldgen

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// icebergFeature places either configured ice state at the dimension sea level.
type icebergFeature struct{ state int32 }

// decodeIcebergFeature resolves the configured state once during resource loading.
func decodeIcebergFeature(raw json.RawMessage) (*icebergFeature, error) {
	var definition struct {
		Config struct {
			State BlockState `json:"state"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &definition); err != nil {
		return nil, err
	}
	state, err := block.BlockStateFromProperties(definition.Config.State.Name, definition.Config.State.Properties)
	if err != nil {
		return nil, fmt.Errorf("iceberg state: %w", err)
	}
	return &icebergFeature{state: state.ID()}, nil
}

// icebergShape stores the sampled dimensions and material for one placement.
type icebergShape struct {
	origin              blockPosition
	state               int32
	above, below, width int
	longAxis, shortAxis int
	angle               float64
	elliptical, snow    bool
}

// placeFeature samples the shape before generating, smoothing, and carving it.
func (f *icebergFeature) placeFeature(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	if system == nil || chunk == nil {
		return false
	}
	shape := icebergShape{origin: blockPosition{origin.x, system.seaLevel, origin.z}, state: f.state}
	shape.snow = random.NextDouble() > .7
	shape.angle = random.NextDouble() * 2 * math.Pi
	shape.longAxis = 11 - int(random.NextIntBound(5))
	shape.shortAxis = 3 + int(random.NextIntBound(3))
	shape.elliptical = random.NextDouble() > .7
	if shape.elliptical {
		shape.above = int(random.NextIntBound(6)) + 6
	} else {
		shape.above = int(random.NextIntBound(15)) + 3
		if random.NextDouble() > .9 {
			shape.above += int(random.NextIntBound(19)) + 7
		}
	}
	shape.below = min(shape.above+int(random.NextIntBound(11)), 18)
	shape.width = min(shape.above+int(random.NextIntBound(7))-int(random.NextIntBound(5)), 11)
	if !shape.elliptical {
		shape.longAxis = 11
	}

	for x := -shape.longAxis; x < shape.longAxis; x++ {
		for z := -shape.longAxis; z < shape.longAxis; z++ {
			for y := 0; y < shape.above; y++ {
				var radius int
				if shape.elliptical {
					radius = icebergEllipseRadius(y, shape.above, shape.width)
				} else {
					radius = icebergRoundRadius(random, y, shape.above, shape.width)
				}
				if shape.elliptical || x < radius {
					shape.placeBlock(chunk, random, x, y, z, radius, shape.longAxis, shape.above)
				}
			}
		}
	}
	shape.smooth(chunk)
	for x := -shape.longAxis; x < shape.longAxis; x++ {
		for z := -shape.longAxis; z < shape.longAxis; z++ {
			for y := -1; y > -shape.below; y-- {
				axis := shape.longAxis
				if shape.elliptical {
					axis = int(math.Ceil(float64(float32(shape.longAxis) *
						(1 - float32(math.Pow(float64(y), 2))/(float32(shape.below)*8)))))
				}
				radius := icebergSteepRadius(random, -y, shape.below, shape.width)
				if x < radius {
					shape.placeBlock(chunk, random, x, y, z, radius, axis, shape.below)
				}
			}
		}
	}
	if shape.elliptical && random.NextDouble() > .1 || !shape.elliptical && random.NextDouble() > .7 {
		shape.carve(chunk, random)
	}
	return true
}

// icebergRoundRadius retains the float rounding and random draw order of the noise-free shape.
func icebergRoundRadius(random worldrandom.Source, y, height, width int) int {
	slope := float32(3.5) - random.NextFloat()
	scale := (1 - float32(math.Pow(float64(y), 2))/(float32(height)*slope)) * float32(width)
	if height > 15+int(random.NextIntBound(5)) {
		adjusted := y
		if y < 3+int(random.NextIntBound(6)) {
			adjusted = y / 2
		}
		scale = (1 - float32(adjusted)/(float32(height)*slope*float32(.4))) * float32(width)
	}
	return int(math.Ceil(float64(scale / 2)))
}

// icebergEllipseRadius computes the deterministic radius of an elliptical slice.
func icebergEllipseRadius(y, height, width int) int {
	scale := (1 - float32(math.Pow(float64(y), 2))/float32(height)) * float32(width)
	return int(math.Ceil(float64(scale / 2)))
}

// icebergSteepRadius computes the taper below sea level and consumes one random float.
func icebergSteepRadius(random worldrandom.Source, y, height, width int) int {
	slope := 1 + random.NextFloat()/2
	scale := (1 - float32(y)/(float32(height)*slope)) * float32(width)
	return int(math.Ceil(float64(scale / 2)))
}

// icebergEllipseDistance tests a rotated horizontal ellipse without changing evaluation order.
func icebergEllipseDistance(x, z, centerX, centerZ, longAxis, shortAxis int, angle float64) float64 {
	u := (float64(x-centerX)*math.Cos(angle) - float64(z-centerZ)*math.Sin(angle)) / float64(longAxis)
	v := (float64(x-centerX)*math.Sin(angle) + float64(z-centerZ)*math.Cos(angle)) / float64(shortAxis)
	return math.Pow(u, 2) + math.Pow(v, 2) - 1
}

// placeBlock decides a single shape cell and applies the optional snow cap.
func (s *icebergShape) placeBlock(chunk *section.Chunk, random worldrandom.Source, x, y, z, radius, axis, height int) {
	var distance float64
	if s.elliptical {
		minor := s.shortAxis
		if y > 0 && height-y <= 3 {
			minor -= 4 - (height - y)
		}
		distance = icebergEllipseDistance(x, z, 0, 0, axis, minor, s.angle)
	} else {
		perturb := float32(10) * max(float32(.2), min(random.NextFloat(), float32(.8))) / float32(radius)
		distance = float64(perturb) + float64(x*x) + float64(z*z) - float64(radius*radius)
	}
	if !(distance < 0) {
		return
	}
	boundary := -.5
	if !s.elliptical {
		boundary = float64(-6 - random.NextIntBound(3))
	}
	if distance > boundary && random.NextDouble() > .9 {
		return
	}
	position := blockPosition{s.origin.x + x, s.origin.y + y, s.origin.z + z}
	current := getChunkBlock(chunk, position)
	name := block.NameFromState(current)
	if !block.IsAir(current) && name != "minecraft:snow_block" && name != "minecraft:ice" && name != "minecraft:water" {
		return
	}
	allowSnow := !s.elliptical || random.NextDouble() > .05
	divisor := 2
	if s.elliptical {
		divisor = 3
	}
	if s.snow && name != "minecraft:water" &&
		float64(height-y) <= float64(random.NextIntBound(int32(max(1, height/divisor))))+float64(height)*.6 && allowSnow {
		setChunkBlock(chunk, position, block.DefaultStateID("minecraft:snow_block"))
		return
	}
	setChunkBlock(chunk, position, s.state)
}

// icebergMaterial reports whether smoothing and carving may replace the state.
func icebergMaterial(state int32) bool {
	switch block.NameFromState(state) {
	case "minecraft:packed_ice", "minecraft:blue_ice", "minecraft:snow_block":
		return true
	default:
		return false
	}
}

// smooth removes unsupported caps and single-cell projections above the waterline.
func (s *icebergShape) smooth(chunk *section.Chunk) {
	axis := s.width / 2
	if s.elliptical {
		axis = s.longAxis
	}
	for x := -axis; x <= axis; x++ {
		for z := -axis; z <= axis; z++ {
			for y := 0; y <= s.above; y++ {
				position := blockPosition{s.origin.x + x, s.origin.y + y, s.origin.z + z}
				state := getChunkBlock(chunk, position)
				if !icebergMaterial(state) && block.NameFromState(state) != "minecraft:snow" {
					continue
				}
				if block.IsAir(getChunkBlock(chunk, blockPosition{position.x, position.y - 1, position.z})) {
					setChunkBlock(chunk, position, 0)
					setChunkBlock(chunk, blockPosition{position.x, position.y + 1, position.z}, 0)
					continue
				}
				if !icebergMaterial(state) {
					continue
				}
				exposed := 0
				for _, neighbor := range [...]blockPosition{
					{position.x - 1, position.y, position.z}, {position.x + 1, position.y, position.z},
					{position.x, position.y, position.z - 1}, {position.x, position.y, position.z + 1},
				} {
					if !icebergMaterial(getChunkBlock(chunk, neighbor)) {
						exposed++
					}
				}
				if exposed >= 3 {
					setChunkBlock(chunk, position, 0)
				}
			}
		}
	}
}

// carve cuts an opening through the iceberg and refills submerged cells with water.
func (s *icebergShape) carve(chunk *section.Chunk, random worldrandom.Source) {
	signX, signZ := 1, 1
	if random.NextBoolean() {
		signX = -1
	}
	if random.NextBoolean() {
		signZ = -1
	}
	x := int(random.NextIntBound(int32(max(s.width/2-2, 1))))
	if random.NextBoolean() {
		x = s.width/2 + 1 - int(random.NextIntBound(int32(max(s.width-s.width/2-1, 1))))
	}
	z := int(random.NextIntBound(int32(max(s.width/2-2, 1))))
	if random.NextBoolean() {
		z = s.width/2 + 1 - int(random.NextIntBound(int32(max(s.width-s.width/2-1, 1))))
	}
	if s.elliptical {
		x = int(random.NextIntBound(int32(max(s.longAxis-5, 1))))
		z = x
	}
	x, z = signX*x, signZ*z
	angle := s.angle + math.Pi/2
	if !s.elliptical {
		angle = random.NextDouble() * 2 * math.Pi
	}
	for y := 0; y < s.above-3; y++ {
		s.carveSlice(chunk, icebergRoundRadius(random, y, s.above, s.width), y, x, z, angle)
	}
	for y := -1; y > -s.above+int(random.NextIntBound(5)); y-- {
		s.carveSlice(chunk, icebergSteepRadius(random, -y, s.above, s.width), y, x, z, angle)
	}
}

// carveSlice replaces only ice and snow cells inside the cut-out ellipse.
func (s *icebergShape) carveSlice(chunk *section.Chunk, radius, y, centerX, centerZ int, angle float64) {
	longAxis := radius + 1 + s.longAxis/3
	shortAxis := min(radius-3, 3) + s.shortAxis/2 - 1
	for x := -longAxis; x < longAxis; x++ {
		for z := -longAxis; z < longAxis; z++ {
			if !(icebergEllipseDistance(x, z, centerX, centerZ, longAxis, shortAxis, angle) < 0) {
				continue
			}
			position := blockPosition{s.origin.x + x, s.origin.y + y, s.origin.z + z}
			state := getChunkBlock(chunk, position)
			if !icebergMaterial(state) {
				continue
			}
			if y < 0 {
				setChunkBlock(chunk, position, block.DefaultStateID("minecraft:water"))
			} else {
				setChunkBlock(chunk, position, 0)
				above := blockPosition{position.x, position.y + 1, position.z}
				if block.NameFromState(getChunkBlock(chunk, above)) == "minecraft:snow" {
					setChunkBlock(chunk, above, 0)
				}
			}
		}
	}
}
