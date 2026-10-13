package worldgen

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	blockfluid "github.com/vitismc/vitis/internal/block/fluid"
	generatedbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

func TestEndPlatformPlacement(t *testing.T) {
	chunk := section.NewChunkWithBounds(6, 0, 0, 256, generatedbiome.BiomeTheEnd)
	feature := &endPlatformFeature{obsidian: block.DefaultStateID("minecraft:obsidian")}
	feature.placeFeature(nil, chunk, worldrandom.NewLegacy(1), blockPosition{x: 100, y: 49, z: 0})
	if got := chunk.GetBlock(100, 48, 0); got != feature.obsidian {
		t.Fatalf("platform floor = %d, want obsidian %d", got, feature.obsidian)
	}
	if got := chunk.GetBlock(100, 49, 0); !block.IsAir(got) {
		t.Fatalf("platform interior = %d, want air", got)
	}
}

func TestFreezeTopLayerUsesColdBiome(t *testing.T) {
	chunk := section.NewChunk(0, 0, section.OverworldSections, generatedbiome.BiomeSnowyPlains)
	stone := block.DefaultStateID("minecraft:stone")
	water := blockfluid.FlowingStateID(blockfluid.FluidTypeWater, 8, true)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			chunk.SetBlock(x, 62, z, stone)
			chunk.SetBlock(x, 63, z, water)
		}
	}
	ice := block.DefaultStateID("minecraft:ice")
	feature := &freezeTopLayerFeature{ice: ice, snow: block.DefaultStateID("minecraft:snow")}
	system := &featureSystem{biomes: fixedBiomeSource{biome: generatedbiome.BiomeSnowyPlains}}
	feature.placeFeature(system, chunk, worldrandom.NewLegacy(1), blockPosition{})
	if got := chunk.GetBlock(0, 63, 0); got != ice {
		t.Fatalf("surface water = %d, want ice %d", got, ice)
	}
}

func TestUnderwaterMagmaPlacement(t *testing.T) {
	chunk := section.NewChunk(0, 0, section.OverworldSections, 0)
	stone := block.DefaultStateID("minecraft:stone")
	water := blockfluid.FlowingStateID(blockfluid.FluidTypeWater, 8, true)
	for x := 6; x <= 10; x++ {
		for z := 6; z <= 10; z++ {
			chunk.SetBlock(x, 3, z, stone)
			chunk.SetBlock(x, 4, z, stone)
			chunk.SetBlock(x, 5, z, water)
		}
	}
	magma := block.DefaultStateID("minecraft:magma_block")
	feature := &underwaterMagmaFeature{floorSearchRange: 5, probability: 1, radius: 1, magma: magma}
	if !feature.placeFeature(nil, chunk, worldrandom.NewLegacy(1), blockPosition{x: 8, y: 5, z: 8}) {
		t.Fatal("underwater magma was not placed")
	}
	if got := chunk.GetBlock(8, 4, 8); got != magma {
		t.Fatalf("floor center = %d, want magma %d", got, magma)
	}
}
