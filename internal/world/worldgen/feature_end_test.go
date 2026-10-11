package worldgen

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	blockentity "github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

func TestEndGatewayBuildsFrameAndPersistentExit(t *testing.T) {
	chunk := section.NewChunkWithBounds(0, 0, 0, 128, 0)
	feature := &endGatewayFeature{
		hasExit: true,
		exit:    [3]int32{100, 50, 0},
		exact:   true,
		gateway: block.DefaultStateID("minecraft:end_gateway"),
		bedrock: block.DefaultStateID("minecraft:bedrock"),
	}
	origin := blockPosition{x: 8, y: 64, z: 8}
	if !feature.placeFeature(nil, chunk, worldrandom.NewLegacy(0), origin) {
		t.Fatal("end gateway was not placed")
	}
	if got := block.NameFromState(chunk.GetBlock(8, 64, 8)); got != "minecraft:end_gateway" {
		t.Fatalf("gateway center = %s", got)
	}
	for _, y := range []int{62, 66} {
		if got := block.NameFromState(chunk.GetBlock(8, y, 8)); got != "minecraft:bedrock" {
			t.Fatalf("gateway cap at y=%d = %s", y, got)
		}
	}
	entity, ok := chunk.GetBlockEntity(8, 64, 8).(*blockentity.RawBlockEntity)
	if !ok || entity == nil {
		t.Fatal("end gateway block entity is missing")
	}
	root := entity.PersistentNBT()
	value, tagID, ok := root.Get("exit_portal")
	exit, valid := value.([]int32)
	if !ok || tagID != nbt.TagIntArray || !valid || len(exit) != 3 || exit[0] != 100 || exit[1] != 50 || exit[2] != 0 {
		t.Fatalf("end gateway exit = %#v", value)
	}
	if exact, ok := root.GetBool("ExactTeleport"); !ok || !exact {
		t.Fatal("end gateway exact teleport flag is missing")
	}
}

func TestChorusPlantGrowsConnectedBranchesAndFlowers(t *testing.T) {
	chunk := section.NewChunkWithBounds(0, 0, 0, 128, 0)
	chunk.SetBlock(8, 20, 8, block.DefaultStateID("minecraft:end_stone"))
	flower, err := block.BlockStateFromProperties("minecraft:chorus_flower", map[string]string{"age": "5"})
	if err != nil {
		t.Fatal(err)
	}
	feature := &chorusPlantFeature{plant: block.DefaultStateID("minecraft:chorus_plant"), flower: flower.ID()}
	if !feature.placeFeature(nil, chunk, worldrandom.NewLegacy(7), blockPosition{x: 8, y: 21, z: 8}) {
		t.Fatal("chorus plant was not placed")
	}
	plants, flowers := 0, 0
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			for y := 21; y < 64; y++ {
				switch block.NameFromState(chunk.GetBlock(x, y, z)) {
				case "minecraft:chorus_plant":
					plants++
				case "minecraft:chorus_flower":
					flowers++
				}
			}
		}
	}
	if plants == 0 || flowers == 0 {
		t.Fatalf("chorus blocks: plants=%d flowers=%d", plants, flowers)
	}
	base, err := block.BlockStateFromID(chunk.GetBlock(8, 21, 8))
	if err != nil {
		t.Fatal(err)
	}
	if down, ok := base.Property("down"); !ok || down != "true" {
		t.Fatalf("chorus base down connection = %q", down)
	}
}
