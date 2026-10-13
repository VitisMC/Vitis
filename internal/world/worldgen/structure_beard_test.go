package worldgen

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"
)

// TestStructureBeardReference checks every adaptation mode against Minecraft 1.21.4 samples.
func TestStructureBeardReference(t *testing.T) {
	data, err := os.ReadFile("testdata/structure_beard_1_21_4.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Mode   string `json:"mode"`
		Ground int    `json:"ground"`
		Point  [3]int `json:"point"`
		Extra  bool   `json:"extra"`
		Bits   string `json:"bits"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 176 || len(structureBeardKernel) != 13824 {
		t.Fatalf("reference sizes: %d cases, %d kernel coefficients", len(cases), len(structureBeardKernel))
	}
	for index, example := range cases {
		mode, err := decodeTerrainAdjustment(map[string]string{
			"NONE": "none", "BURY": "bury", "BEARD_THIN": "beard_thin", "BEARD_BOX": "beard_box", "ENCAPSULATE": "encapsulate",
		}[example.Mode])
		if err != nil {
			t.Fatal(err)
		}
		input := structureDensity{pieces: []structureRigid{{box: structureBox{low: blockPosition{10, -30, -20}, high: blockPosition{23, -18, -7}}, adjustment: mode, groundDelta: example.Ground}}}
		if example.Extra {
			input.pieces = append(input.pieces, structureRigid{box: structureBox{low: blockPosition{-1, -31, -11}, high: blockPosition{5, -25, -2}}, adjustment: terrainAdjustmentBury})
			input.junctions = []structureJunction{{source: blockPosition{7, -27, -15}}, {source: blockPosition{10, -30, -20}}}
		}
		want, err := strconv.ParseUint(example.Bits, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		got := math.Float64bits(input.sample(example.Point[0], example.Point[1], example.Point[2]))
		if got != want {
			t.Errorf("case %d %+v: got %.17g (0x%x), want %.17g (0x%x)", index, example, math.Float64frombits(got), got, math.Float64frombits(want), want)
		}
	}
}

// TestStructureDensityVerticalBoundsPreserveContributions checks the terrain fast rejection window.
func TestStructureDensityVerticalBoundsPreserveContributions(t *testing.T) {
	box := structureBox{low: blockPosition{10, -30, -20}, high: blockPosition{23, -18, -7}}
	for _, mode := range []terrainAdjustment{terrainAdjustmentBury, terrainAdjustmentThin, terrainAdjustmentBox, terrainAdjustmentEncapsulate} {
		start := &structureStartPlan{adjustment: mode, pieces: []structurePiecePlan{{box: box}}}
		bounded := densityForStructures([]*structureStartPlan{start})
		plain := structureDensity{pieces: []structureRigid{{box: box, adjustment: mode}}}
		for x := 0; x <= 35; x += 5 {
			for y := -55; y <= 5; y++ {
				for z := -30; z <= 0; z += 5 {
					if got, want := math.Float64bits(bounded.sample(x, y, z)), math.Float64bits(plain.sample(x, y, z)); got != want {
						t.Fatalf("mode=%d at (%d,%d,%d): bounded=%g plain=%g", mode, x, y, z, math.Float64frombits(got), math.Float64frombits(want))
					}
				}
			}
		}
	}
}
