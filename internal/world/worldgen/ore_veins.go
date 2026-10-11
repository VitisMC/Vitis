package worldgen

import (
	"math"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/world/worldgen/density"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type oreVeinifier struct {
	enabled bool
	toggle  density.Function
	ridged  density.Function
	gap     density.Function
	random  worldrandom.PositionalFactory
	copper  oreVeinType
	iron    oreVeinType
}

type oreVeinType struct {
	ore, raw, filler int32
	minY, maxY       int
}

func newOreVeinifier(enabled bool, seed int64, legacy bool, toggle, ridged, gap density.Function) *oreVeinifier {
	if !enabled {
		return &oreVeinifier{}
	}
	var source worldrandom.Source = worldrandom.NewXoroshiro(seed)
	if legacy {
		source = worldrandom.NewLegacy(seed)
	}
	factory := source.ForkPositional().FromHashOf("minecraft:ore").ForkPositional()
	return &oreVeinifier{
		enabled: true, toggle: toggle, ridged: ridged, gap: gap, random: factory,
		copper: oreVeinType{
			ore: mustDefaultBlock("minecraft:copper_ore"), raw: mustDefaultBlock("minecraft:raw_copper_block"),
			filler: mustDefaultBlock("minecraft:granite"), minY: 0, maxY: 50,
		},
		iron: oreVeinType{
			ore: mustDefaultBlock("minecraft:deepslate_iron_ore"), raw: mustDefaultBlock("minecraft:raw_iron_block"),
			filler: mustDefaultBlock("minecraft:tuff"), minY: -60, maxY: -8,
		},
	}
}

func (v *oreVeinifier) state(position density.Position) (int32, bool) {
	return v.stateWithScratch(position, nil)
}

// stateWithScratch preserves the vein RNG sequence while reusing caller-owned sources.
func (v *oreVeinifier) stateWithScratch(position density.Position, scratch *worldrandom.PositionalScratch) (int32, bool) {
	if v == nil || !v.enabled {
		return 0, false
	}
	veininess := v.toggle.Sample(position)
	vein := v.iron
	if veininess > 0 {
		vein = v.copper
	}
	if position.Y < vein.minY || position.Y > vein.maxY {
		return 0, false
	}
	distanceToEdge := min(vein.maxY-position.Y, position.Y-vein.minY)
	roundoff := clampedMap(float64(distanceToEdge), 0, 20, -0.2, 0)
	absolute := math.Abs(veininess)
	if absolute+roundoff < float64(float32(0.4)) {
		return 0, false
	}
	var random worldrandom.Source
	if scratch != nil {
		random = scratch.At(v.random, int32(position.X), int32(position.Y), int32(position.Z))
	} else {
		random = v.random.At(int32(position.X), int32(position.Y), int32(position.Z))
	}
	if random.NextFloat() > float32(0.7) || v.ridged.Sample(position) >= 0 {
		return 0, false
	}
	richness := clampedMap(absolute, float64(float32(0.4)), float64(float32(0.6)), float64(float32(0.1)), float64(float32(0.3)))
	if float64(random.NextFloat()) < richness && v.gap.Sample(position) > float64(float32(-0.3)) {
		if random.NextFloat() < float32(0.02) {
			return vein.raw, true
		}
		return vein.ore, true
	}
	return vein.filler, true
}

func clampedMap(value, fromMin, fromMax, toMin, toMax float64) float64 {
	if value <= fromMin {
		return toMin
	}
	if value >= fromMax {
		return toMax
	}
	return mapRange(value, fromMin, fromMax, toMin, toMax)
}

func mustDefaultBlock(name string) int32 {
	state, err := block.DefaultBlockState(name)
	if err != nil {
		panic(err)
	}
	return state.ID()
}
