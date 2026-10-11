package section

import (
	"testing"

	blockentity "github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/nbt"
)

// TestGeneratedBlockEntityBoundaryRouting keeps neighboring entities out of persisted source chunks.
func TestGeneratedBlockEntityBoundaryRouting(t *testing.T) {
	for _, coordinate := range []int32{-1, 16} {
		target := NewChunkWithBounds(0, 0, -64, 384, 0)
		target.EnableBorderBlocks()
		be := blockentity.NewRawBlockEntity(nbt.NewCompound().PutString("id", "minecraft:beehive").PutInt("x", coordinate).PutInt("y", 70).PutInt("z", 8))
		target.AddBlockEntity(be)
		if len(target.BlockEntities) != 0 || len(target.BorderBlockEntities()) != 1 || target.GetBlockEntity(coordinate, 70, 8) != be {
			t.Fatal("neighbor entity stored in source chunk")
		}
		if target.CloneRaw().BorderBlockEntities() != nil {
			t.Fatal("raw clone retained transient entities")
		}
		target.RemoveBlockEntity(coordinate, 70, 8)
		if target.GetBlockEntity(coordinate, 70, 8) != nil {
			t.Fatal("neighbor entity removal failed")
		}
		target.AddBlockEntity(be)
		target.DropBorderBlocks()
		if target.BorderBlockEntities() != nil || target.GetBlockEntity(coordinate, 70, 8) != nil {
			t.Fatal("returned chunk retained transient entities")
		}
	}
}

// TestGeneratedBlockEntityVerticalBounds rejects neighboring entities outside the dimension.
func TestGeneratedBlockEntityVerticalBounds(t *testing.T) {
	target := NewChunkWithBounds(0, 0, 0, 128, 0)
	target.EnableBorderBlocks()
	for _, y := range []int32{-1, 128} {
		target.AddBlockEntity(blockentity.NewRawBlockEntity(nbt.NewCompound().PutString("id", "minecraft:chest").PutInt("x", 16).PutInt("y", y).PutInt("z", 8)))
	}
	if len(target.BlockEntities) != 0 || len(target.BorderBlockEntities()) != 0 {
		t.Fatal("out-of-bounds entity accepted")
	}
}
