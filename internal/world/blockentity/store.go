package blockentity

import (
	"encoding/json"
	"math/rand"

	blockentity "github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/loot"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/chunk"
)

// BlockEntityType IDs matching the Minecraft protocol registry.
const (
	TypeSign     int32 = 7
	TypeHangSign int32 = 35
)

// SignData holds the text lines for both sides of a sign.
type SignData struct {
	FrontLines [4]string
	BackLines  [4]string
}

// ChunkLookup resolves a loaded authoritative chunk.
type ChunkLookup func(x, z int32) (*chunk.Chunk, bool)

// Store exposes typed block-entity operations backed by chunk ownership.
type Store struct {
	lookup ChunkLookup
	loot   *loot.Runtime
}

// NewStore creates a chunk-backed block entity store.
func NewStore(lookup ...ChunkLookup) *Store {
	store := &Store{}
	if len(lookup) > 0 {
		store.lookup = lookup[0]
	}
	return store
}

// SetLootRuntime configures deferred container loot generation.
func (s *Store) SetLootRuntime(runtime *loot.Runtime) {
	if s != nil {
		s.loot = runtime
	}
}

// SetSign stores sign data at the given position.
func (s *Store) SetSign(x, y, z int, data *SignData) {
	owner := s.owner(x, z)
	if owner == nil || owner.Data() == nil || data == nil {
		return
	}
	raw := signCompound(data).PutInt("x", int32(x)).PutInt("y", int32(y)).PutInt("z", int32(z))
	owner.Data().AddBlockEntity(blockentity.NewRawBlockEntity(raw))
	owner.MarkDirty()
}

// GetSign returns sign data at the given position, or nil.
func (s *Store) GetSign(x, y, z int) *SignData {
	owner := s.owner(x, z)
	if owner == nil || owner.Data() == nil {
		return nil
	}
	blockEntity := owner.Data().GetBlockEntity(int32(x), int32(y), int32(z))
	persistent, ok := blockEntity.(interface{ PersistentNBT() *nbt.Compound })
	if !ok {
		return nil
	}
	root := persistent.PersistentNBT()
	if root == nil {
		return nil
	}
	return decodeSign(root)
}

// RemoveSign removes sign data at the given position.
func (s *Store) RemoveSign(x, y, z int) {
	owner := s.owner(x, z)
	if owner == nil || owner.Data() == nil {
		return
	}
	owner.Data().RemoveBlockEntity(int32(x), int32(y), int32(z))
	owner.MarkDirty()
}

// Container returns the chunk-owned inventory at a block position, upgrading raw NBT on demand.
func (s *Store) Container(x, y, z int, typeName string, slots int) *inventory.Container {
	if typeName == "minecraft:furnace" || typeName == "minecraft:blast_furnace" || typeName == "minecraft:smoker" {
		if furnace := s.Furnace(x, y, z, typeName); furnace != nil && furnace.InventoryValid() {
			return furnace.Container()
		}
		return nil
	}
	owner := s.owner(x, z)
	if owner == nil || owner.Data() == nil {
		return nil
	}
	current := owner.Data().GetBlockEntity(int32(x), int32(y), int32(z))
	if current != nil && current.TypeName() != typeName {
		return nil
	}
	if typed, ok := current.(*blockentity.ContainerBlockEntity); ok && typed.TypeName() == typeName && typed.Container().Size() == slots {
		if !typed.InventoryValid() {
			return nil
		}
		if raw := typed.PersistentNBT(); raw != nil {
			if _, deferred := raw.GetString("LootTable"); !deferred {
				return typed.Container()
			}
		}
	}
	var raw *nbt.Compound
	if persistent, ok := current.(interface{ PersistentNBT() *nbt.Compound }); ok && current.TypeName() == typeName {
		raw = persistent.PersistentNBT()
	}
	var generated []inventory.Slot
	if raw != nil {
		if tableID, hasTable := raw.GetString("LootTable"); hasTable && tableID != "" {
			if s.loot == nil || s.loot.Table(tableID) == nil {
				return nil
			}
			seed, _ := raw.GetLong("LootTableSeed")
			generated = rollContainerLoot(s.loot, tableID, seed, slots)
		}
	}
	typed := blockentity.NewContainerBlockEntity(typeName, int32(x), int32(y), int32(z), slots, raw)
	if typed == nil || !typed.InventoryValid() {
		return nil
	}
	if generated != nil {
		typed.ApplyLoot(generated)
	}
	owner.Data().AddBlockEntity(typed)
	owner.MarkDirty()
	return typed.Container()
}

// rollContainerLoot maps evaluated stacks into stable, randomized container slots.
func rollContainerLoot(runtime *loot.Runtime, tableID string, seed int64, slots int) []inventory.Slot {
	rng := rand.New(rand.NewSource(seed))
	drops := runtime.Roll(tableID, loot.NewContext(rng))
	items := make([]inventory.Slot, 0, len(drops))
	for _, drop := range drops {
		itemID := item.IDByName(drop.ItemID)
		stackSize := int(item.StackSize(itemID))
		if itemID < 0 || drop.Count <= 0 || stackSize <= 0 {
			continue
		}
		for remaining := drop.Count; remaining > 0 && len(items) < slots; {
			count := min(remaining, stackSize)
			items = append(items, inventory.NewSlot(itemID, int32(count)))
			remaining -= count
		}
	}
	rng.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
	result := make([]inventory.Slot, slots)
	positions := rng.Perm(slots)
	for index, stack := range items {
		if index >= slots {
			break
		}
		result[positions[index]] = stack
	}
	return result
}

// Furnace returns a chunk-owned furnace, restoring raw NBT on first use.
func (s *Store) Furnace(x, y, z int, typeName string) *blockentity.FurnaceLikeBlockEntity {
	owner := s.owner(x, z)
	if owner == nil || owner.Data() == nil {
		return nil
	}
	current := owner.Data().GetBlockEntity(int32(x), int32(y), int32(z))
	if current != nil && current.TypeName() != typeName {
		return nil
	}
	if typed, ok := current.(*blockentity.FurnaceLikeBlockEntity); ok && typed.TypeName() == typeName {
		return typed
	}
	var raw *nbt.Compound
	if persistent, ok := current.(interface{ PersistentNBT() *nbt.Compound }); ok && current.TypeName() == typeName {
		raw = persistent.PersistentNBT()
	}
	var furnace *blockentity.FurnaceLikeBlockEntity
	switch typeName {
	case "minecraft:furnace":
		furnace = blockentity.NewFurnaceBlockEntity(int32(x), int32(y), int32(z))
	case "minecraft:blast_furnace":
		furnace = blockentity.NewBlastFurnaceBlockEntity(int32(x), int32(y), int32(z))
	case "minecraft:smoker":
		furnace = blockentity.NewSmokerBlockEntity(int32(x), int32(y), int32(z))
	default:
		return nil
	}
	furnace.LoadPersistentNBT(raw)
	owner.Data().AddBlockEntity(furnace)
	owner.MarkDirty()
	return furnace
}

// MarkContainerDirty records a committed inventory mutation for persistence.
func (s *Store) MarkContainerDirty(x, z int) {
	if owner := s.owner(x, z); owner != nil {
		owner.MarkDirty()
	}
}

// RemoveContainer removes a broken inventory block entity and returns its contents.
func (s *Store) RemoveContainer(x, y, z int) []inventory.Slot {
	owner := s.owner(x, z)
	if owner == nil || owner.Data() == nil {
		return nil
	}
	current := owner.Data().GetBlockEntity(int32(x), int32(y), int32(z))
	if current != nil && current.TypeName() == "minecraft:chest" {
		if root, ok := current.(interface{ PersistentNBT() *nbt.Compound }); ok {
			if raw := root.PersistentNBT(); raw != nil {
				if _, hasLoot := raw.GetString("LootTable"); hasLoot {
					if s.Container(x, y, z, current.TypeName(), 27) != nil {
						current = owner.Data().GetBlockEntity(int32(x), int32(y), int32(z))
					}
				}
			}
		}
	}
	var slots []inventory.Slot
	switch typed := current.(type) {
	case *blockentity.ContainerBlockEntity:
		slots = typed.Container().Slots()
	case *blockentity.FurnaceLikeBlockEntity:
		slots = typed.Container().Slots()
	case interface{ PersistentNBT() *nbt.Compound }:
		root := typed.PersistentNBT()
		if root != nil {
			if decoded := blockentity.NewContainerBlockEntity(current.TypeName(), int32(x), int32(y), int32(z), 54, root); decoded != nil {
				slots = decoded.Container().Slots()
			}
		}
	}
	owner.Data().RemoveBlockEntity(int32(x), int32(y), int32(z))
	owner.MarkDirty()
	return slots
}

func (s *Store) owner(x, z int) *chunk.Chunk {
	if s == nil || s.lookup == nil {
		return nil
	}
	owner, ok := s.lookup(int32(x>>4), int32(z>>4))
	if !ok {
		return nil
	}
	return owner
}

// EncodeSignNBT encodes sign data to NBT bytes for the BlockEntityData packet.
func EncodeSignNBT(sd *SignData) []byte {
	if sd == nil {
		sd = &SignData{}
	}
	root := signCompound(sd)
	enc := nbt.NewEncoder(256)
	_ = enc.WriteRootCompound(root)
	return enc.Bytes()
}

func signCompound(sd *SignData) *nbt.Compound {
	root := nbt.NewCompound().PutString("id", "minecraft:sign")

	front := nbt.NewCompound()
	frontMsgs := nbt.NewList(nbt.TagString)
	for _, line := range sd.FrontLines {
		if line == "" {
			frontMsgs.Add("{\"text\":\"\"}")
		} else {
			frontMsgs.Add("{\"text\":\"" + escapeJSON(line) + "\"}")
		}
	}
	front.PutList("messages", frontMsgs)
	root.PutCompound("front_text", front)

	back := nbt.NewCompound()
	backMsgs := nbt.NewList(nbt.TagString)
	for _, line := range sd.BackLines {
		if line == "" {
			backMsgs.Add("{\"text\":\"\"}")
		} else {
			backMsgs.Add("{\"text\":\"" + escapeJSON(line) + "\"}")
		}
	}
	back.PutList("messages", backMsgs)
	root.PutCompound("back_text", back)

	return root
}

func decodeSign(root *nbt.Compound) *SignData {
	result := &SignData{}
	decodeSignSide(root, "front_text", &result.FrontLines)
	decodeSignSide(root, "back_text", &result.BackLines)
	return result
}

func decodeSignSide(root *nbt.Compound, name string, target *[4]string) {
	side, ok := root.GetCompound(name)
	if !ok {
		return
	}
	messages, ok := side.GetList("messages")
	if !ok {
		return
	}
	for index, value := range messages.Elements() {
		if index >= len(target) {
			break
		}
		raw, ok := value.(string)
		if !ok {
			continue
		}
		var component struct {
			Text string `json:"text"`
		}
		if json.Unmarshal([]byte(raw), &component) == nil {
			target[index] = component.Text
		}
	}
}

func escapeJSON(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			out = append(out, '\\', '"')
		case '\\':
			out = append(out, '\\', '\\')
		case '\n':
			out = append(out, '\\', 'n')
		default:
			out = append(out, c)
		}
	}
	return string(out)
}
