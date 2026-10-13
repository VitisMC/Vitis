package world

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/lighting"
	"github.com/vitismc/vitis/internal/world/worldgen"
)

// TestWorldgenNBTDigest prints stable chunk and lighting digests for cross-build comparisons.
func TestWorldgenNBTDigest(t *testing.T) {
	if os.Getenv("VITIS_BENCH_DIGEST") != "1" {
		t.Skip("set VITIS_BENCH_DIGEST=1 to compare generated contents")
	}
	resources := &datapack.Manager{}
	runtime, err := worldgen.NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{"../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	for _, dimension := range []string{"minecraft:overworld", "minecraft:nether", "minecraft:end"} {
		for _, seed := range []int64{0, 345678765456} {
			terrain, err := runtime.NewTerrainGenerator(dimension, seed, 0)
			if err != nil {
				t.Fatal(err)
			}
			generator, err := chunk.NewWorldgenGenerator(terrain)
			if err != nil {
				t.Fatal(err)
			}
			for _, coordinate := range [][2]int32{{-37, -41}, {0, 0}, {618, -625}} {
				generated, err := generator.Generate(coordinate[0], coordinate[1])
				if err != nil {
					t.Fatal(err)
				}
				encoded, err := chunk.EncodeChunkNBT(generated)
				if err != nil {
					t.Fatal(err)
				}
				if directory := os.Getenv("VITIS_BENCH_NBT_DIR"); directory != "" {
					name := fmt.Sprintf("%s-%d-%d-%d.nbt", strings.ReplaceAll(dimension, ":", "_"), seed, coordinate[0], coordinate[1])
					if err := os.WriteFile(filepath.Join(directory, name), encoded, 0600); err != nil {
						t.Fatal(err)
					}
				}
				data := generated.Data()
				states := make([]byte, len(data.Sections)*(4096+64)*4)
				offset := 0
				for sectionIndex, sec := range data.Sections {
					for y := 0; y < 16; y++ {
						for z := 0; z < 16; z++ {
							for x := 0; x < 16; x++ {
								binary.LittleEndian.PutUint32(states[offset:], uint32(data.GetBlock(x, data.MinY()+sectionIndex*16+y, z)))
								offset += 4
							}
						}
					}
					for y := 0; y < 4; y++ {
						for z := 0; z < 4; z++ {
							for x := 0; x < 4; x++ {
								binary.LittleEndian.PutUint32(states[offset:], uint32(sec.GetBiome(x, y, z)))
								offset += 4
							}
						}
					}
				}
				lightHash := sha256.New()
				for _, sections := range [][]*lighting.NibbleArray{generated.Data().LightEngine.BlockLightSections, generated.Data().LightEngine.SkyLightSections} {
					for _, light := range sections {
						if light == nil {
							var zero [lighting.NibbleArraySize]byte
							_, _ = lightHash.Write(zero[:])
						} else {
							_, _ = lightHash.Write(light.Bytes())
						}
					}
				}
				t.Logf("%s seed=%d chunk=%d,%d states=%x light=%x nbt=%x", dimension, seed, coordinate[0], coordinate[1], sha256.Sum256(states), lightHash.Sum(nil), sha256.Sum256(encoded))
			}
		}
	}
}
