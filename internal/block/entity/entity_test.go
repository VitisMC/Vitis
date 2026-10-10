package entity

import (
	"testing"

	genbe "github.com/vitismc/vitis/internal/data/generated/block_entity"
	"github.com/vitismc/vitis/internal/nbt"
)

func TestHasBlockEntity(t *testing.T) {
	tests := []struct {
		blockName string
		want      bool
	}{
		{"minecraft:chest", true},
		{"minecraft:furnace", true},
		{"minecraft:stone", false},
		{"minecraft:air", false},
		{"minecraft:sign", true},
		{"minecraft:beacon", true},
	}

	for _, tt := range tests {
		got := HasBlockEntity(tt.blockName)
		if got != tt.want {
			t.Errorf("HasBlockEntity(%q) = %v, want %v", tt.blockName, got, tt.want)
		}
	}
}

func TestBlockEntityTypeID(t *testing.T) {
	tests := []struct {
		blockName string
		wantID    int32
	}{
		{"minecraft:furnace", genbe.BlockEntityFurnace},
		{"minecraft:chest", genbe.BlockEntityChest},
		{"minecraft:trapped_chest", genbe.BlockEntityTrappedChest},
		{"minecraft:stone", -1},
		{"minecraft:sign", genbe.BlockEntitySign},
	}

	for _, tt := range tests {
		got := BlockEntityTypeID(tt.blockName)
		if got != tt.wantID {
			t.Errorf("BlockEntityTypeID(%q) = %d, want %d", tt.blockName, got, tt.wantID)
		}
	}
}

func TestBaseBlockEntity(t *testing.T) {
	be := NewBaseBlockEntity("minecraft:chest", 10, 64, 20)

	if be.TypeID() != genbe.BlockEntityChest {
		t.Errorf("TypeID() = %d, want %d", be.TypeID(), genbe.BlockEntityChest)
	}

	if be.TypeName() != "minecraft:chest" {
		t.Errorf("TypeName() = %q, want %q", be.TypeName(), "minecraft:chest")
	}

	x, y, z := be.Position()
	if x != 10 || y != 64 || z != 20 {
		t.Errorf("Position() = (%d, %d, %d), want (10, 64, 20)", x, y, z)
	}

	nbt := be.WriteNBT()
	if nbt["id"] != "minecraft:chest" {
		t.Errorf("WriteNBT()[id] = %v, want minecraft:chest", nbt["id"])
	}
	if nbt["x"] != int32(10) {
		t.Errorf("WriteNBT()[x] = %v, want 10", nbt["x"])
	}
}

func TestRawBlockEntityPreservesUnknownNBT(t *testing.T) {
	raw := nbt.NewCompound().
		PutString("id", "minecraft:chest").
		PutInt("x", 1).
		PutInt("y", 64).
		PutInt("z", 2).
		PutString("Custom", "preserve")
	blockEntity := NewRawBlockEntity(raw)
	if blockEntity == nil {
		t.Fatal("NewRawBlockEntity returned nil")
	}
	blockEntity.SetPosition(3, 65, 4)
	persistent := blockEntity.PersistentNBT()
	if value, _ := persistent.GetString("Custom"); value != "preserve" {
		t.Fatalf("custom tag = %q", value)
	}
	if x, _ := persistent.GetInt("x"); x != 3 {
		t.Fatalf("x = %d, want 3", x)
	}
	if len(blockEntity.ChunkDataNBT()) == 0 {
		t.Fatal("network NBT is empty")
	}
}

func TestGeneratedConstants(t *testing.T) {
	if genbe.BlockEntityFurnace != 0 {
		t.Errorf("TypeFurnace = %d, want 0", genbe.BlockEntityFurnace)
	}
	if genbe.BlockEntityChest != 1 {
		t.Errorf("TypeChest = %d, want 1", genbe.BlockEntityChest)
	}
	if genbe.BlockEntityCount != 45 {
		t.Errorf("BlockEntityCount = %d, want 45", genbe.BlockEntityCount)
	}
}
