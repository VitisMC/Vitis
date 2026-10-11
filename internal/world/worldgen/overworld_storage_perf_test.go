package worldgen

import (
	"testing"

	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/persistence"
)

// BenchmarkOverworldChunkStorage measures encoding and region writing after generation.
func BenchmarkOverworldChunkStorage(b *testing.B) {
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		b.Fatal(err)
	}
	if err := resources.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		b.Fatal(err)
	}
	terrain, err := runtime.NewTerrainGenerator("minecraft:overworld", 0, 0)
	if err != nil {
		b.Fatal(err)
	}
	generator, err := chunk.NewWorldgenGenerator(terrain)
	if err != nil {
		b.Fatal(err)
	}
	generated, err := generator.Generate(0, 0)
	if err != nil {
		b.Fatal(err)
	}
	store := persistence.NewChunkStore(b.TempDir())
	defer store.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := chunk.EncodeChunkNBT(generated)
		if err != nil {
			b.Fatal(err)
		}
		if err := store.WriteChunkNBT(i%32, (i/32)%32, data); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "chunks/s")
}
