package worldgen

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

func TestDeltaFeatureRequiresEnclosedFloorAndOpenTop(t *testing.T) {
	chunk := section.NewChunkWithBounds(0, 0, 0, 128, 0)
	netherrack := block.DefaultStateID("minecraft:netherrack")
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			for y := 0; y <= 31; y++ {
				chunk.SetBlock(x, y, z, netherrack)
			}
		}
	}
	for x := 4; x <= 12; x++ {
		for z := 4; z <= 12; z++ {
			chunk.SetBlock(x, 31, z, 0)
		}
	}
	feature := &deltaFeature{
		contents: block.DefaultStateID("minecraft:lava"),
		rim:      block.DefaultStateID("minecraft:magma_block"),
		rimSize:  constantInt(0),
		size:     constantInt(2),
	}
	if !feature.placeFeature(nil, chunk, worldrandom.NewLegacy(1), blockPosition{x: 8, y: 30, z: 8}) {
		t.Fatal("delta was not placed")
	}
	if got := chunk.GetBlock(8, 30, 8); got != feature.contents {
		t.Fatalf("delta center = %d, want %d", got, feature.contents)
	}
}

func TestBasaltPillarDescendsFromCeiling(t *testing.T) {
	chunk := section.NewChunkWithBounds(0, 0, 0, 128, 0)
	netherrack := block.DefaultStateID("minecraft:netherrack")
	chunk.SetBlock(8, 20, 8, netherrack)
	chunk.SetBlock(8, 10, 8, netherrack)
	feature := &basaltPillarFeature{basalt: block.DefaultStateID("minecraft:basalt")}
	if !feature.placeFeature(nil, chunk, worldrandom.NewLegacy(2), blockPosition{x: 8, y: 19, z: 8}) {
		t.Fatal("basalt pillar was not placed")
	}
	for y := 11; y <= 19; y++ {
		if got := chunk.GetBlock(8, y, 8); got != feature.basalt {
			t.Fatalf("pillar block at y=%d is %d, want %d", y, got, feature.basalt)
		}
	}
}

func TestBasaltColumnsGrowFromValidFloor(t *testing.T) {
	chunk := section.NewChunkWithBounds(0, 0, 0, 128, 0)
	netherrack := block.DefaultStateID("minecraft:netherrack")
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			chunk.SetBlock(x, 10, z, netherrack)
		}
	}
	feature := &basaltColumnsFeature{
		basalt: block.DefaultStateID("minecraft:basalt"),
		reach:  constantInt(1),
		height: constantInt(4),
	}
	if !feature.placeFeature(&featureSystem{seaLevel: 32}, chunk, worldrandom.NewLegacy(3), blockPosition{x: 8, y: 11, z: 8}) {
		t.Fatal("basalt columns were not placed")
	}
	placed := 0
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			for y := 11; y < 20; y++ {
				if chunk.GetBlock(x, y, z) == feature.basalt {
					placed++
				}
			}
		}
	}
	if placed == 0 {
		t.Fatal("basalt column feature reported success without placing basalt")
	}
}

func TestNetherForestVegetationRequiresNyliumAndSpreads(t *testing.T) {
	chunk := section.NewChunkWithBounds(0, 0, 0, 128, 0)
	nylium := block.DefaultStateID("minecraft:warped_nylium")
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			chunk.SetBlock(x, 10, z, nylium)
		}
	}
	feature := &netherForestVegetationFeature{
		provider:     simpleStateProvider(block.DefaultStateID("minecraft:nether_sprouts")),
		spreadWidth:  8,
		spreadHeight: 4,
	}
	if !feature.placeFeature(nil, chunk, worldrandom.NewLegacy(4), blockPosition{x: 8, y: 11, z: 8}) {
		t.Fatal("nether vegetation was not placed")
	}
	placed := 0
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			for y := 11; y < 15; y++ {
				if block.NameFromState(chunk.GetBlock(x, y, z)) == "minecraft:nether_sprouts" {
					placed++
				}
			}
		}
	}
	if placed == 0 {
		t.Fatal("nether vegetation reported success without placing blocks")
	}
}

func TestVineColumnsUsePlantBodiesAndAgedHeads(t *testing.T) {
	chunk := section.NewChunkWithBounds(0, 0, 0, 128, 0)
	random := worldrandom.NewLegacy(5)

	placeVinesColumn(
		chunk, random, blockPosition{x: 4, y: 11, z: 4}, 3, 1,
		block.DefaultStateID("minecraft:twisting_vines"), block.DefaultStateID("minecraft:twisting_vines_plant"),
	)
	if got := block.NameFromState(chunk.GetBlock(4, 11, 4)); got != "minecraft:twisting_vines_plant" {
		t.Fatalf("twisting vine body = %s", got)
	}
	assertVineHeadAge(t, chunk.GetBlock(4, 13, 4), "minecraft:twisting_vines")

	placeVinesColumn(
		chunk, random, blockPosition{x: 8, y: 19, z: 8}, 3, -1,
		block.DefaultStateID("minecraft:weeping_vines"), block.DefaultStateID("minecraft:weeping_vines_plant"),
	)
	if got := block.NameFromState(chunk.GetBlock(8, 19, 8)); got != "minecraft:weeping_vines_plant" {
		t.Fatalf("weeping vine body = %s", got)
	}
	assertVineHeadAge(t, chunk.GetBlock(8, 16, 8), "minecraft:weeping_vines")
}

func assertVineHeadAge(t *testing.T, stateID int32, name string) {
	t.Helper()
	state, err := block.BlockStateFromID(stateID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Name() != name {
		t.Fatalf("vine head = %s, want %s", state.Name(), name)
	}
	age := state.Properties()["age"]
	if age < "17" || age > "25" {
		t.Fatalf("vine head age = %s", age)
	}
}
