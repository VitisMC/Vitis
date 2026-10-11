package blockentity

import (
	"testing"

	blockentity "github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/loot"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/chunk"
)

func TestStorePersistsSignsInOwningChunk(t *testing.T) {
	owner := chunk.New(-1, 2)
	store := NewStore(func(x, z int32) (*chunk.Chunk, bool) {
		if x != -1 || z != 2 {
			t.Fatalf("lookup coordinates = %d,%d", x, z)
		}
		return owner, true
	})
	sign := &SignData{FrontLines: [4]string{"one", "two"}, BackLines: [4]string{"back"}}
	store.SetSign(-1, 64, 32, sign)

	if owner.Data().GetBlockEntity(-1, 64, 32) == nil {
		t.Fatal("sign was not stored in chunk")
	}
	loaded := store.GetSign(-1, 64, 32)
	if loaded == nil || loaded.FrontLines[0] != "one" || loaded.BackLines[0] != "back" {
		t.Fatalf("loaded sign = %#v", loaded)
	}
	if !owner.Dirty() {
		t.Fatal("block entity mutation did not mark chunk dirty")
	}

	store.RemoveSign(-1, 64, 32)
	if owner.Data().GetBlockEntity(-1, 64, 32) != nil {
		t.Fatal("sign was not removed from chunk")
	}
}

// TestDeferredChestLootRollsOnce checks opening, persistence and missing table safety.
func TestDeferredChestLootRollsOnce(t *testing.T) {
	resources := &datapack.Manager{}
	runtime, err := loot.NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	owner := chunk.New(0, 0)
	owner.Data().AddBlockEntity(blockentity.NewRawBlockEntity(nbt.NewCompound().
		PutString("id", "minecraft:chest").PutInt("x", 1).PutInt("y", 64).PutInt("z", 2).
		PutString("LootTable", "minecraft:chests/buried_treasure").PutLong("LootTableSeed", 12345)))
	store := NewStore(func(x, z int32) (*chunk.Chunk, bool) { return owner, x == 0 && z == 0 })
	if got := store.Container(1, 64, 2, "minecraft:chest", 27); got != nil {
		t.Fatal("opened deferred chest without a loot runtime")
	}
	store.SetLootRuntime(runtime)
	contents := store.Container(1, 64, 2, "minecraft:chest", 27)
	if contents == nil {
		t.Fatal("loot chest did not open")
	}
	var itemCount int32
	for _, stack := range contents.Slots() {
		itemCount += stack.ItemCount
	}
	if itemCount == 0 {
		t.Fatal("loot chest remained empty")
	}
	if again := store.Container(1, 64, 2, "minecraft:chest", 27); again != contents {
		t.Fatal("chest inventory was replaced on second open")
	}
	encoded, err := chunk.EncodeChunkNBT(owner)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := chunk.DecodeChunkNBT(encoded, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	loadedStore := NewStore(func(x, z int32) (*chunk.Chunk, bool) { return loaded, x == 0 && z == 0 })
	loadedStore.SetLootRuntime(runtime)
	restored := loadedStore.Container(1, 64, 2, "minecraft:chest", 27)
	var restoredCount int32
	for _, stack := range restored.Slots() {
		restoredCount += stack.ItemCount
	}
	if restoredCount != itemCount {
		t.Fatalf("restored item count = %d, want %d", restoredCount, itemCount)
	}
	root := loaded.Data().GetBlockEntity(1, 64, 2).(*blockentity.ContainerBlockEntity).PersistentNBT()
	if _, present := root.GetString("LootTable"); present {
		t.Fatal("consumed loot table marker was persisted")
	}
	owner.Data().AddBlockEntity(blockentity.NewRawBlockEntity(nbt.NewCompound().
		PutString("id", "minecraft:chest").PutInt("x", 3).PutInt("y", 64).PutInt("z", 2).
		PutString("LootTable", "minecraft:chests/buried_treasure").PutLong("LootTableSeed", 54321)))
	var brokenCount int32
	for _, stack := range store.RemoveContainer(3, 64, 2) {
		brokenCount += stack.ItemCount
	}
	if brokenCount == 0 || owner.Data().GetBlockEntity(3, 64, 2) != nil {
		t.Fatalf("unopened chest break: drops=%d, entity=%v", brokenCount, owner.Data().GetBlockEntity(3, 64, 2))
	}
}

func TestContainerSurvivesChunkRoundTrip(t *testing.T) {
	owner := chunk.New(0, 0)
	store := NewStore(func(x, z int32) (*chunk.Chunk, bool) { return owner, x == 0 && z == 0 })
	contents := store.Container(1, 64, 2, "minecraft:chest", 27)
	if contents == nil {
		t.Fatal("container was not created")
	}
	contents.Set(4, inventory.NewSlot(item.IDByName("minecraft:stone"), 12))
	store.MarkContainerDirty(1, 2)
	encoded, err := chunk.EncodeChunkNBT(owner)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := chunk.DecodeChunkNBT(encoded, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	loadedStore := NewStore(func(x, z int32) (*chunk.Chunk, bool) { return loaded, x == 0 && z == 0 })
	got := loadedStore.Container(1, 64, 2, "minecraft:chest", 27).Get(4)
	if got.ItemCount != 12 || got.ItemID != item.IDByName("minecraft:stone") {
		t.Fatalf("round-trip item = %+v", got)
	}
}
