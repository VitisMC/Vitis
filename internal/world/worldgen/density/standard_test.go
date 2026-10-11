package density

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitismc/vitis/internal/world/worldgen/noise"
)

// TestGeneratedStandardMatchesUniversal protects direct evaluation and marker arithmetic.
func TestGeneratedStandardMatchesUniversal(t *testing.T) {
	directory := "../../../../.mcdata/1.21.4/vanilla/data/minecraft/worldgen"
	registry := Registry{Definitions: make(map[string]json.RawMessage), Noises: make(map[string]noise.Parameters), CellWidth: 4, CellHeight: 8}
	for _, kind := range []string{"density_function", "noise"} {
		err := filepath.WalkDir(filepath.Join(directory, kind), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".json") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(filepath.Join(directory, kind), path)
			if err != nil {
				return err
			}
			id := "minecraft:" + strings.TrimSuffix(filepath.ToSlash(relative), ".json")
			if kind == "density_function" {
				registry.Definitions[id] = data
			} else {
				var parameters noise.Parameters
				if err := json.Unmarshal(data, &parameters); err != nil {
					return err
				}
				registry.Noises[id] = parameters
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(directory, "noise_settings", "overworld.json"))
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Router map[string]json.RawMessage `json:"noise_router"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	heights := []int{-65, -64, -63, -41, -40, -39, 239, 240, 241, 255, 256, 257, 319, 320, 321}
	for y := -64; y < 320; y += 17 {
		heights = append(heights, y)
	}
	for _, seed := range []int64{0, 123456789, -987654321} {
		registry.Seed = seed
		fast, err := NewCompiler(registry)
		if err != nil {
			t.Fatal(err)
		}
		if err := fast.EnableStandardOverworld(); err != nil {
			t.Fatal(err)
		}
		general, err := NewCompiler(registry)
		if err != nil {
			t.Fatal(err)
		}
		for name, raw := range settings.Router {
			got, err := fast.CompileInline(raw)
			if err != nil {
				t.Fatal(err)
			}
			want, err := general.CompileInline(raw)
			if err != nil {
				t.Fatal(err)
			}
			for _, origin := range [][2]int{{-32, -16}, {0, 0}, {29999968, -29999968}} {
				ge, we := NewChunkEvaluation(origin[0], -64, origin[1], 384), NewChunkEvaluation(origin[0], -64, origin[1], 384)
				for x := 0; x < 16; x += 3 {
					for z := 0; z < 16; z += 5 {
						for _, y := range heights {
							p := Position{X: origin[0] + x, Y: y, Z: origin[1] + z, Evaluation: ge}
							actual := got.Sample(p)
							p.Evaluation = we
							expected := want.Sample(p)
							if math.Float64bits(actual) != math.Float64bits(expected) {
								t.Fatalf("seed=%d router=%s at %d,%d,%d: %.17g != %.17g", seed, name, p.X, p.Y, p.Z, actual, expected)
							}
						}
					}
				}
			}
		}
		actual, err := fast.CompileInline(settings.Router["final_density"])
		if err != nil {
			t.Fatal(err)
		}
		expected, err := general.CompileInline(settings.Router["final_density"])
		if err != nil {
			t.Fatal(err)
		}
		var samples [16 * 16 * 64]float64
		fastPlan, referencePlan := fast.ChunkInterpolation(), general.ChunkInterpolation()
		for _, origin := range [][2]int{{-16, -32}, {0, 0}, {29999968, -29999968}} {
			ge, we := NewChunkEvaluation(origin[0], -64, origin[1], 64), NewChunkEvaluation(origin[0], -64, origin[1], 64)
			fastPlan.Fill(ge, func(p Position) { samples[(p.Y+64)*256+(p.Z-origin[1])*16+p.X-origin[0]] = actual.Sample(p) })
			referencePlan.Fill(we, func(p Position) {
				got := samples[(p.Y+64)*256+(p.Z-origin[1])*16+p.X-origin[0]]
				if want := expected.Sample(p); got != want {
					t.Fatalf("seed %d batched position %v: %.17g != %.17g", seed, p, got, want)
				}
			})
		}
	}
}
