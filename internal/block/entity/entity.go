package entity

import (
	genbe "github.com/vitismc/vitis/internal/data/generated/block_entity"
)

type BlockEntity interface {
	TypeID() int32
	TypeName() string
	Position() (x, y, z int32)
	SetPosition(x, y, z int32)

	WriteNBT() map[string]any
	ReadNBT(data map[string]any)

	ChunkDataNBT() []byte
}

type BaseBlockEntity struct {
	typeID   int32
	typeName string
	x, y, z  int32
}

func NewBaseBlockEntity(typeName string, x, y, z int32) *BaseBlockEntity {
	return &BaseBlockEntity{
		typeID:   genbe.BlockEntityIDByName(typeName),
		typeName: typeName,
		x:        x,
		y:        y,
		z:        z,
	}
}

func (b *BaseBlockEntity) TypeID() int32 {
	return b.typeID
}

func (b *BaseBlockEntity) TypeName() string {
	return b.typeName
}

func (b *BaseBlockEntity) Position() (x, y, z int32) {
	return b.x, b.y, b.z
}

func (b *BaseBlockEntity) SetPosition(x, y, z int32) {
	b.x = x
	b.y = y
	b.z = z
}

func (b *BaseBlockEntity) WriteNBT() map[string]any {
	return map[string]any{
		"id": b.typeName,
		"x":  b.x,
		"y":  b.y,
		"z":  b.z,
	}
}

func (b *BaseBlockEntity) ReadNBT(data map[string]any) {
	if id, ok := data["id"].(string); ok {
		b.typeName = id
		b.typeID = genbe.BlockEntityIDByName(id)
	}
	if x, ok := data["x"].(int32); ok {
		b.x = x
	}
	if y, ok := data["y"].(int32); ok {
		b.y = y
	}
	if z, ok := data["z"].(int32); ok {
		b.z = z
	}
}

func (b *BaseBlockEntity) ChunkDataNBT() []byte {
	return nil
}

func HasBlockEntity(blockName string) bool {
	return genbe.BlockEntityIDByName(blockName) >= 0
}

func BlockEntityTypeID(blockName string) int32 {
	return genbe.BlockEntityIDByName(blockName)
}

func BlockEntityTypeName(blockName string) string {
	return genbe.BlockEntityNameByID(genbe.BlockEntityIDByName(blockName))
}
