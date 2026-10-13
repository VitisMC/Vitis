package section

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/block/entity"
	genbe "github.com/vitismc/vitis/internal/data/generated/block_entity"
	"github.com/vitismc/vitis/internal/nbt"
)

// TestBedHalvesOwnRendererEntities verifies all colors create the protocol bed type.
func TestBedHalvesOwnRendererEntities(t *testing.T) {
	for _, color := range []string{"white", "orange", "magenta", "light_blue", "yellow", "lime", "pink", "gray", "light_gray", "cyan", "purple", "blue", "brown", "green", "red", "black"} {
		t.Run(color, func(t *testing.T) {
			c := NewChunk(-1, 2, OverworldSections, 0)
			for index, part := range []string{"head", "foot"} {
				x, y, z := -2, 64, 34+index
				state := block.StateID("minecraft:"+color+"_bed", map[string]string{"part": part, "facing": "north", "occupied": "false"})
				c.SetBlock(x, y, z, state)
				be := c.GetBlockEntity(int32(x), int32(y), int32(z))
				if be == nil || be.TypeID() != genbe.BlockEntityBed {
					t.Fatal("bed half lacks its renderer entity")
				}
				root, err := nbt.NewDecoder(be.ChunkDataNBT()).ReadRootCompound()
				if err != nil {
					t.Fatal(err)
				}
				if id, _ := root.GetString("id"); id != "minecraft:bed" {
					t.Fatalf("bed entity identifier = %q", id)
				}
				c.SetBlock(x, y, z, 0)
				if c.GetBlockEntity(int32(x), int32(y), int32(z)) != nil {
					t.Fatal("removed bed left its renderer entity")
				}
			}
		})
	}
}

// TestBedStateChangesPreservePersistentEntity keeps custom data across sleep/wake.
func TestBedStateChangesPreservePersistentEntity(t *testing.T) {
	c := NewChunk(0, 0, OverworldSections, 0)
	state := block.StateID("minecraft:red_bed", map[string]string{"part": "head", "facing": "north", "occupied": "false"})
	c.SetBlock(1, 64, 1, state)
	be := entity.NewRawBlockEntity(nbt.NewCompound().PutString("id", "minecraft:bed").PutInt("x", 1).PutInt("y", 64).PutInt("z", 1).PutString("FutureField", "keep"))
	c.AddBlockEntity(be)
	c.SetBlock(1, 64, 1, block.StateID("minecraft:red_bed", map[string]string{"part": "head", "facing": "north", "occupied": "true"}))
	if c.GetBlockEntity(1, 64, 1) != be || c.RepairBedBlockEntities() != 0 {
		t.Fatal("bed state update replaced its persistent entity")
	}
}
