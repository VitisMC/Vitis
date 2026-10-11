package worldgen

import (
	"strings"
	"sync"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	generatedbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
)

// TestGeneratedChunkIncludesNeighborTree verifies deterministic border assembly.
func TestGeneratedChunkIncludesNeighborTree(t *testing.T) {
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	generator, err := runtime.NewTerrainGenerator("minecraft:overworld", 123456789, 0)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := generator.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	for x := int32(-3); x <= 3; x++ {
		for z := int32(-3); z <= 3; z++ {
			entry := generator.rawChunk(snapshot, x, z)
			if entry.err != nil {
				t.Fatal(entry.err)
			}
			border := entry.chunk.BorderBlocks()
			for position, state := range border {
				name := block.NameFromState(state)
				if !strings.HasSuffix(name, "_log") && !strings.HasSuffix(name, "_leaves") {
					continue
				}
				targetX, targetZ := int32(position.X>>4), int32(position.Z>>4)
				if targetX == x && targetZ == z {
					continue
				}
				rawTarget, err := generator.generateRaw(snapshot, targetX, targetZ)
				if err != nil {
					t.Fatal(err)
				}
				if !block.IsAir(rawTarget.GetBlock(position.X, position.Y, position.Z)) {
					continue
				}
				target, err := generator.Generate(targetX, targetZ)
				if err != nil {
					t.Fatal(err)
				}
				if target.GetBlock(position.X, position.Y, position.Z) == state {
					return
				}
			}
		}
	}
	t.Fatal("no neighboring tree block was assembled")
}

func TestOverworldGenerationIndependentOfConcurrentOrder(t *testing.T) {
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	generator, err := runtime.NewTerrainGenerator("minecraft:overworld", 123456789, 0)
	if err != nil {
		t.Fatal(err)
	}
	coordinates := [][2]int32{{0, 0}, {1, 0}, {-1, 2}, {3, -2}}
	want := make([]*section.Chunk, len(coordinates))
	for i, coordinate := range coordinates {
		want[i], err = generator.Generate(coordinate[0], coordinate[1])
		if err != nil {
			t.Fatal(err)
		}
	}
	got := make([]*section.Chunk, len(coordinates))
	errors := make([]error, len(coordinates))
	var workers sync.WaitGroup
	for i, coordinate := range coordinates {
		workers.Add(1)
		go func() {
			defer workers.Done()
			got[i], errors[i] = generator.Generate(coordinate[0], coordinate[1])
		}()
	}
	workers.Wait()
	for i := range coordinates {
		if errors[i] != nil {
			t.Fatal(errors[i])
		}
		for y := want[i].MinY(); y <= want[i].MaxY(); y++ {
			for z := 0; z < 16; z++ {
				for x := 0; x < 16; x++ {
					if want[i].GetBlock(x, y, z) != got[i].GetBlock(x, y, z) {
						t.Fatalf("chunk %v differs at %d,%d,%d", coordinates[i], x, y, z)
					}
				}
			}
		}
		for s := range want[i].Sections {
			for y := 0; y < 4; y++ {
				for z := 0; z < 4; z++ {
					for x := 0; x < 4; x++ {
						if want[i].Sections[s].GetBiome(x, y, z) != got[i].Sections[s].GetBiome(x, y, z) {
							t.Fatalf("chunk %v biome differs in section %d at %d,%d,%d", coordinates[i], s, x, y, z)
						}
					}
				}
			}
		}
	}
}

func TestTerrainGeneratorUsesDimensionBoundsAndIsDeterministic(t *testing.T) {
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	generator, err := runtime.NewTerrainGenerator("minecraft:end", 123456789, 0)
	if err != nil {
		t.Fatal(err)
	}
	first, err := generator.Generate(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	second, err := generator.Generate(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if first.MinY() != 0 || first.MaxY() != 255 || len(first.Sections) != 16 {
		t.Fatalf("End bounds = %d..%d (%d sections)", first.MinY(), first.MaxY(), len(first.Sections))
	}
	for y := first.MinY(); y <= first.MaxY(); y++ {
		for x := 0; x < 16; x++ {
			for z := 0; z < 16; z++ {
				if first.GetBlock(x, y, z) != second.GetBlock(x, y, z) {
					t.Fatalf("non-deterministic state at %d,%d,%d", x, y, z)
				}
			}
		}
	}
	for sectionIndex, current := range first.Sections {
		for y := 0; y < 4; y++ {
			for z := 0; z < 4; z++ {
				for x := 0; x < 4; x++ {
					got := current.GetBiome(x, y, z)
					if got != generatedbiome.BiomeTheEnd {
						t.Fatalf("End biome in section %d at %d,%d,%d = %d", sectionIndex, x, y, z, got)
					}
					if got != second.Sections[sectionIndex].GetBiome(x, y, z) {
						t.Fatal("non-deterministic biome")
					}
				}
			}
		}
	}
}

// TestConcurrentDuplicateGenerationOwnsSeparateChunks exercises consumption of cached terrain.
func TestConcurrentDuplicateGenerationOwnsSeparateChunks(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	for _, dimension := range []string{"minecraft:overworld", "minecraft:nether", "minecraft:end"} {
		t.Run(dimension, func(t *testing.T) {
			generator, err := runtime.NewTerrainGenerator(dimension, 42, 0)
			if err != nil {
				t.Fatal(err)
			}
			var results [4]*section.Chunk
			var errors [4]error
			var workers sync.WaitGroup
			for index := range results {
				workers.Add(1)
				go func() { defer workers.Done(); results[index], errors[index] = generator.Generate(-2, -3) }()
			}
			workers.Wait()
			for index := range results {
				if errors[index] != nil {
					t.Fatal(errors[index])
				}
				if index > 0 && results[index] == results[0] {
					t.Fatal("mutable terrain shared between callers")
				}
				for y := results[0].MinY(); y <= results[0].MaxY(); y++ {
					for z := 0; z < 16; z++ {
						for x := 0; x < 16; x++ {
							if results[index].GetBlock(x, y, z) != results[0].GetBlock(x, y, z) {
								t.Fatalf("duplicate differs at %d,%d,%d", x, y, z)
							}
						}
					}
				}
			}
			x, y, z := -32, results[0].MinY(), -48
			original := results[1].GetBlock(x, y, z)
			results[0].SetBlock(x, y, z, original+1)
			if got := results[1].GetBlock(x, y, z); got != original {
				t.Fatal("mutation changed another caller's chunk")
			}
			snapshot, err := generator.snapshot()
			if err != nil {
				t.Fatal(err)
			}
			generator.rawMu.Lock()
			entry := generator.rawCache[rawChunkKey{snapshot: snapshot, x: -2, z: -3}]
			generator.rawMu.Unlock()
			if entry == nil || !entry.complete || entry.chunk != nil {
				t.Fatal("consumed terrain retained instead of border writes")
			}
		})
	}
}
