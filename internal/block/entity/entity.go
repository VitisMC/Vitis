package entity

import (
	genbe "github.com/vitismc/vitis/internal/data/generated/block_entity"
	"github.com/vitismc/vitis/internal/nbt"
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
	root := nbt.NewCompound().
		PutString("id", b.typeName).
		PutInt("x", b.x).
		PutInt("y", b.y).
		PutInt("z", b.z)
	encoder := nbt.NewEncoder(128)
	if err := encoder.WriteRootCompound(root); err != nil {
		return nil
	}
	return encoder.Bytes()
}

// RawBlockEntity preserves a block entity's complete NBT while exposing its
// protocol identity and position through BlockEntity.
type RawBlockEntity struct {
	*BaseBlockEntity
	raw *nbt.Compound
}

// NewRawBlockEntity creates a lossless block entity from chunk NBT.
func NewRawBlockEntity(raw *nbt.Compound) *RawBlockEntity {
	if raw == nil {
		return nil
	}
	typeName, ok := raw.GetString("id")
	if !ok || typeName == "" {
		return nil
	}
	x, xOK := raw.GetInt("x")
	y, yOK := raw.GetInt("y")
	z, zOK := raw.GetInt("z")
	if !xOK || !yOK || !zOK {
		return nil
	}
	return &RawBlockEntity{
		BaseBlockEntity: NewBaseBlockEntity(typeName, x, y, z),
		raw:             raw.Clone(),
	}
}

// PersistentNBT returns a lossless copy of the block entity NBT.
func (b *RawBlockEntity) PersistentNBT() *nbt.Compound {
	if b == nil || b.raw == nil {
		return nil
	}
	root := b.raw.Clone()
	x, y, z := b.Position()
	root.PutString("id", b.TypeName()).PutInt("x", x).PutInt("y", y).PutInt("z", z)
	return root
}

// ChunkDataNBT returns unnamed network NBT for chunk and block-entity packets.
func (b *RawBlockEntity) ChunkDataNBT() []byte {
	root := b.PersistentNBT()
	if root == nil {
		return nil
	}
	encoder := nbt.NewEncoder(256)
	if err := encoder.WriteRootCompound(root); err != nil {
		return nil
	}
	return encoder.Bytes()
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
