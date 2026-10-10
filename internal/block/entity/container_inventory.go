package entity

import (
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/logger"
	"github.com/vitismc/vitis/internal/nbt"
)

// ContainerBlockEntity owns the inventory of a simple block container.
type ContainerBlockEntity struct {
	*BaseBlockEntity
	container      *inventory.Container
	raw            *nbt.Compound
	rawItems       map[int]*nbt.Compound
	inventoryValid bool
}

// NewContainerBlockEntity creates a chunk-owned inventory block entity.
func NewContainerBlockEntity(typeName string, x, y, z int32, slots int, raw *nbt.Compound) *ContainerBlockEntity {
	if slots <= 0 || slots > 54 {
		return nil
	}
	b := &ContainerBlockEntity{
		BaseBlockEntity: NewBaseBlockEntity(typeName, x, y, z),
		container:       inventory.NewContainer(slots),
		raw:             raw.Clone(),
	}
	b.rawItems, b.inventoryValid = readContainerItems(b.container, raw)
	return b
}

// Container returns the authoritative slots owned by the block entity.
func (b *ContainerBlockEntity) Container() *inventory.Container { return b.container }

// InventoryValid reports whether all stored slots decoded without data loss.
func (b *ContainerBlockEntity) InventoryValid() bool { return b.inventoryValid }

// ApplyLoot stores generated contents and clears the deferred loot marker.
func (b *ContainerBlockEntity) ApplyLoot(slots []inventory.Slot) {
	if b == nil || !b.inventoryValid {
		return
	}
	for index, stack := range slots {
		if index >= b.container.Size() {
			break
		}
		b.container.Set(index, stack)
	}
	if b.raw != nil {
		b.raw.Delete("LootTable")
		b.raw.Delete("LootTableSeed")
	}
}

// PersistentNBT merges owned inventory fields with the original block entity NBT.
func (b *ContainerBlockEntity) PersistentNBT() *nbt.Compound {
	root := b.raw.Clone()
	if root == nil {
		root = nbt.NewCompound()
	}
	x, y, z := b.Position()
	root.PutString("id", b.TypeName()).PutInt("x", x).PutInt("y", y).PutInt("z", z)
	if b.inventoryValid {
		root.PutList("Items", writeContainerItems(b.container, b.rawItems))
	}
	return root
}

// ChunkDataNBT encodes the current block entity for chunk streaming.
func (b *ContainerBlockEntity) ChunkDataNBT() []byte {
	encoder := nbt.NewEncoder(256)
	if err := encoder.WriteRootCompound(b.PersistentNBT()); err != nil {
		return nil
	}
	return encoder.Bytes()
}

// WriteNBT returns the minimal typed representation required by BlockEntity.
func (b *ContainerBlockEntity) WriteNBT() map[string]any { return b.BaseBlockEntity.WriteNBT() }

// ReadNBT updates only the fields owned by the typed container.
func (b *ContainerBlockEntity) ReadNBT(data map[string]any) { b.BaseBlockEntity.ReadNBT(data) }

// readContainerItems restores complete stacks owned by this block entity.
func readContainerItems(container *inventory.Container, root *nbt.Compound) (map[int]*nbt.Compound, bool) {
	raw := make(map[int]*nbt.Compound)
	if root == nil {
		return raw, true
	}
	list, ok := root.GetList("Items")
	if !ok {
		_, _, present := root.Get("Items")
		return raw, !present
	}
	valid := (list.Len() == 0 || list.ElementType() == nbt.TagCompound) && list.Len() <= container.Size()
	seen := make(map[int8]bool)
	for _, value := range list.Elements() {
		entry, ok := value.(*nbt.Compound)
		if !ok || entry == nil {
			valid = false
			continue
		}
		slot, ok := entry.GetByte("Slot")
		if !ok || slot < 0 || int(slot) >= container.Size() || seen[slot] {
			valid = false
			continue
		}
		seen[slot] = true
		stack, err := inventory.ItemStackFromNBT(entry)
		if err != nil {
			valid = false
			logger.Error("container item decode failed", "slot", slot, "error", err)
			raw[int(slot)] = entry.Clone()
			continue
		}
		container.Set(int(slot), stack)
	}
	return raw, valid
}

// writeContainerItems serializes complete stacks rather than grafting slot metadata.
func writeContainerItems(container *inventory.Container, raw map[int]*nbt.Compound) *nbt.List {
	list := nbt.NewList(nbt.TagCompound)
	for index, stack := range container.Slots() {
		if stack.Empty() {
			if entry := raw[index]; entry != nil {
				list.Add(entry.Clone())
			}
			continue
		}
		entry, err := inventory.ItemStackToNBT(stack)
		if err != nil {
			logger.Error("container item encode failed", "slot", index, "error", err)
			entry = stack.PersistentData.Clone()
			if entry == nil {
				continue
			}
		}
		entry.PutByte("Slot", int8(index))
		list.Add(entry)
	}
	return list
}
