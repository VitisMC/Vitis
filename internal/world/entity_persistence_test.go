package world

import (
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/projectile"
	"github.com/vitismc/vitis/internal/inventory"
	itemregistry "github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/persistence"
)

// persistentTestWorld constructs an isolated world without spawn residency.
func persistentTestWorld(t *testing.T, path string, c *chunk.Chunk) (*World, *chunk.Storage) {
	t.Helper()
	store := persistence.NewChunkStore(path)
	storage := chunk.NewStorage(1)
	if c != nil {
		storage.Set(c)
	}
	manager := chunk.NewManager(chunk.ManagerConfig{Storage: storage, Generator: chunk.NewPersistentGenerator(store, chunk.NewTerrainGenerator(0, 0), 0), OnUnload: makeChunkSaveFunc(store)})
	w, err := New(Config{Name: "entity-test", Dimension: "minecraft:overworld", ChunkStore: store, ChunkManager: manager, AutosaveIntervalTicks: 1200})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.SetGameRule("spawnChunkRadius", "0"); err != nil {
		t.Fatal(err)
	}
	if err := w.SetGameRule("doMobSpawning", "false"); err != nil {
		t.Fatal(err)
	}
	return w, storage
}

// awaitEntityLoad applies asynchronous worker results through the connected world tick.
func awaitEntityLoad(t *testing.T, w *World, x, z int32) *chunk.Chunk {
	t.Helper()
	w.chunks.RequestLoad(x, z)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		w.Tick()
		if c, ok := w.chunks.PeekLoaded(x, z); ok {
			return c
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("entity-bearing chunk did not load")
	return nil
}

// TestRuntimeEntitiesSurviveUnloadReloadAndRestart verifies the complete connected storage lifecycle.
func TestRuntimeEntitiesSurviveUnloadReloadAndRestart(t *testing.T) {
	path := t.TempDir()
	c := chunk.New(-1, 2)
	w, _ := persistentTestWorld(t, path, c)
	pos := entity.Vec3{X: -8, Y: 64, Z: 40}
	item := entity.NewItemEntity(41, protocol.UUID{1, 1}, pos, inventory.NewSlot(1, 17))
	item.SetPickupDelay(29)
	orb := entity.NewXPOrb(42, protocol.UUID{1, 2}, pos, 13)
	tnt := entity.NewTNTEntityWithFuse(43, protocol.UUID{1, 3}, pos, -1, 37)
	mob := entity.NewMobEntityByName(44, protocol.UUID{1, 4}, "cow", pos)
	mob.SetHealth(7)
	brokenHelmet := inventory.NewSlot(itemregistry.IDByName("minecraft:diamond_helmet"), 1)
	brokenHelmet.SetDamage(itemregistry.MaxDurability(brokenHelmet.ItemID))
	if err := mob.SetEquipment(entity.EquipmentHead, brokenHelmet); err != nil {
		t.Fatal(err)
	}
	if protected, err := mob.WearSunHelmet(); err != nil || !protected || !mob.Equipment(entity.EquipmentHead).Empty() {
		t.Fatal("sunlight helmet did not break before its snapshot")
	}
	mob.SetNoAI(true)
	gear := inventory.NewSlot(itemregistry.IDByName("minecraft:diamond_chestplate"), 1)
	gear.SetDamage(7)
	mob.SetCanPickUpLoot(true)
	if picked, _, err := mob.EquipPickedItem(gear, nil, rand.New(rand.NewSource(1))); err != nil || picked.Empty() {
		t.Fatal(err)
	}
	arrow := projectile.NewArrow(45, protocol.UUID{1, 5}, pos, entity.Vec3{X: 0.25}, -1, protocol.UUID{7, 8})
	arrow.SetDamage(4)
	if err := arrow.AddEffect(effect.Instance{ID: effectdata.EffectSlowness, Duration: 600, Flags: effect.DefaultFlags()}); err != nil {
		t.Fatal(err)
	}
	w.SpawnItemEntity(item)
	w.SpawnXPOrb(orb)
	w.SpawnTNT(tnt)
	w.mobs.Add(mob)
	w.SpawnArrow(arrow)
	w.Tick()
	if w.IsEntityTicking(-1, 2) {
		t.Fatal("test chunk unexpectedly active")
	}
	if !w.chunks.MarkForUnload(-1, 2) {
		t.Fatal("unload rejected")
	}
	w.Tick()
	if w.entities.Count() != 5 {
		t.Fatal("entities evicted before durable acknowledgement")
	}
	deadline := time.Now().Add(3 * time.Second)
	for w.chunks.IsTracked(-1, 2) && time.Now().Before(deadline) {
		w.Tick()
		time.Sleep(time.Millisecond)
	}
	if w.chunks.IsTracked(-1, 2) || w.entities.Count() != 0 || w.items.Count() != 0 || w.mobs.Count() != 0 {
		t.Fatal("durable eviction retained runtime entities")
	}
	if _, err := os.Stat(filepath.Join(path, "entities", "r.-1.0.mca")); err != nil {
		t.Fatal(err)
	}
	awaitEntityLoad(t, w, -1, 2)
	if w.entities.Count() != 5 {
		t.Fatalf("reload entity count = %d", w.entities.Count())
	}
	for range 3 {
		w.Tick()
	}
	if w.entities.Count() != 5 {
		t.Fatal("repeated bind duplicated entities")
	}
	assert := func(w *World) {
		t.Helper()
		found := make(map[protocol.UUID]bool)
		for id, e := range w.entities.Entities() {
			if found[e.UUID()] {
				t.Fatal("duplicate UUID")
			}
			found[e.UUID()] = true
			if e.Position() != pos {
				t.Fatal("inactive entity position changed")
			}
			switch e.UUID() {
			case item.UUID():
				restored := w.items.Get(id)
				if restored == nil || restored.Stack().ItemCount != 17 || restored.PickupDelay() != 29 {
					t.Fatal("item state lost")
				}
			case orb.UUID():
				restored := w.xpOrbs.Get(id)
				if restored == nil || restored.XPValue() != 13 {
					t.Fatal("XP state lost")
				}
			case tnt.UUID():
				restored := w.tnts.Get(id)
				if restored == nil || restored.Fuse() != 37 {
					t.Fatal("TNT state lost")
				}
			case mob.UUID():
				restored := w.mobs.Get(id)
				if restored == nil || restored.Health() != 7 || !restored.NoAI() {
					t.Fatal("mob state lost")
				}
				if !restored.CanPickUpLoot() || !restored.PersistenceRequired() || !restored.Equipment(entity.EquipmentChest).StackableWith(gear) || restored.EquipmentDropChance(entity.EquipmentChest) != 2 || restored.ArmorDefense() != 8 {
					t.Fatal("equipment state lost across eviction or restart")
				}
				if !restored.Equipment(entity.EquipmentHead).Empty() || restored.ConsumeEquipmentBroken() != 0 {
					t.Fatal("restart restored a broken sunlight helmet or replayed its break event")
				}
			case arrow.UUID():
				restored := w.projectiles.GetArrow(id)
				if restored == nil || restored.Damage() != 4 || restored.OwnerUUID() != (protocol.UUID{7, 8}) {
					t.Fatal("arrow state lost")
				}
				if effects := restored.HitEffects(); len(effects) != 1 || effects[0].ID != effectdata.EffectSlowness || effects[0].Duration != 600 {
					t.Fatal("arrow effect lost during unload/restart")
				}
			default:
				t.Fatal("unexpected entity")
			}
		}
		if len(found) != 5 {
			t.Fatalf("missing entities: %d", len(found))
		}
	}
	assert(w)
	if err := w.Close(nil); err != nil {
		t.Fatal(err)
	}
	restarted, _ := persistentTestWorld(t, path, nil)
	defer func() {
		if err := restarted.Close(nil); err != nil {
			t.Error(err)
		}
	}()
	awaitEntityLoad(t, restarted, -1, 2)
	assert(restarted)
}

// TestEntityWriteFailureRetainsRuntimeAndRetries verifies independent entity-region failures.
func TestEntityWriteFailureRetainsRuntimeAndRetries(t *testing.T) {
	path := t.TempDir()
	blocker := filepath.Join(path, "entities")
	if err := os.WriteFile(blocker, []byte("block region directory"), 0600); err != nil {
		t.Fatal(err)
	}
	c := chunk.New(0, 0)
	w, _ := persistentTestWorld(t, path, c)
	defer func() {
		if err := w.Close(nil); err != nil {
			t.Error(err)
		}
	}()
	item := entity.NewItemEntity(1, protocol.UUID{2, 1}, entity.Vec3{X: 8, Y: 64, Z: 8}, inventory.NewSlot(1, 1))
	w.SpawnItemEntity(item)
	w.Tick()
	if !w.chunks.MarkForUnload(0, 0) {
		t.Fatal("unload rejected")
	}
	w.chunks.ProcessUnloads(1)
	select {
	case result := <-w.autosave.results:
		if result.err == nil {
			t.Fatal("entity write unexpectedly succeeded")
		}
		w.autosave.acknowledge(result)
	case <-time.After(3 * time.Second):
		t.Fatal("writer did not complete")
	}
	if !c.Dirty() || w.entities.Count() != 1 {
		t.Fatal("failed entity save released runtime state")
	}
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	w.chunks.ProcessUnloads(1)
	select {
	case result := <-w.autosave.results:
		if result.err != nil {
			t.Fatal(result.err)
		}
		w.autosave.acknowledge(result)
	case <-time.After(3 * time.Second):
		t.Fatal("retry did not complete")
	}
	w.chunks.ProcessUnloads(1)
	if w.chunks.IsTracked(0, 0) || w.entities.Count() != 0 {
		t.Fatal("successful retry did not evict saved state")
	}
}

// entityRegionNBT builds a compatible isolated region snapshot.
func entityRegionNBT(t *testing.T, x, z int32, entries ...*nbt.Compound) []byte {
	t.Helper()
	list := nbt.NewList(nbt.TagCompound)
	for _, raw := range entries {
		list.Add(raw)
	}
	root := nbt.NewCompound().PutInt("DataVersion", protocol.CurrentDataVersion).PutIntArray("Position", []int32{x, z}).PutList("Entities", list).PutString("example:root", "keep")
	encoder := nbt.NewEncoder(512)
	if err := encoder.WriteNamedRootCompound("", root); err != nil {
		t.Fatal(err)
	}
	return encoder.Bytes()
}

// TestOpaquePassengerGraphsPreserveAllNBT verifies unsupported mounts cannot be flattened or duplicated.
func TestOpaquePassengerGraphsPreserveAllNBT(t *testing.T) {
	path := t.TempDir()
	c := chunk.New(-1, 0)
	w, _ := persistentTestWorld(t, path, c)
	parent := entity.NewEntity(0, protocol.UUID{3, 1}, entity.EntityTypeMob, entity.Vec3{X: -8, Y: 64, Z: 8}, entity.Vec2{})
	child := entity.NewEntity(0, protocol.UUID{3, 2}, entity.EntityTypeMob, entity.Vec3{X: -8, Y: 65, Z: 8}, entity.Vec2{})
	parentRaw, _ := parent.PersistentBaseNBT("minecraft:boat")
	childRaw, _ := child.PersistentBaseNBT("minecraft:zombie")
	parentRaw.PutList("Passengers", nbt.NewList(nbt.TagCompound).Add(childRaw.PutCompound("example:unknown", nbt.NewCompound().PutString("keep", "nested"))))
	c.LoadEntityNBT(entityRegionNBT(t, -1, 0, parentRaw))
	w.Tick()
	if w.entities.Count() != 0 {
		t.Fatal("unsupported mount graph was partially instantiated")
	}
	if err := w.Close(nil); err != nil {
		t.Fatal(err)
	}
	store := persistence.NewChunkStore(path)
	defer store.Close()
	data, err := store.ReadEntityNBT(-1, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, root, err := nbt.NewDecoder(data).ReadNamedRootCompound()
	if err != nil {
		t.Fatal(err)
	}
	list, _ := root.GetList("Entities")
	if list.Len() != 1 || !reflect.DeepEqual(parentRaw, list.Elements()[0]) {
		t.Fatal("opaque passenger graph changed")
	}
	if value, _ := root.GetString("example:root"); value != "keep" {
		t.Fatal("unknown region fields lost")
	}
}

// TestMalformedEntityRestoreIsAtomic exercises validation before chunk publication.
func TestMalformedEntityRestoreIsAtomic(t *testing.T) {
	path := t.TempDir()
	store := persistence.NewChunkStore(path)
	c := chunk.New(0, 0)
	data, err := chunk.EncodeChunkNBT(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WriteChunkNBT(0, 0, data); err != nil {
		t.Fatal(err)
	}
	good := entity.NewItemEntity(1, protocol.UUID{4, 1}, entity.Vec3{X: 8, Y: 64, Z: 8}, inventory.NewSlot(1, 1))
	raw, _ := good.PersistentNBT()
	bad := raw.Clone().PutIntArray("UUID", entity.UUIDToIntArray(protocol.UUID{4, 2})).PutInt("Age", 7)
	if err := store.WriteEntityNBT(0, 0, entityRegionNBT(t, 0, 0, raw, bad)); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	w, _ := persistentTestWorld(t, path, nil)
	defer func() { _ = w.Close(nil) }()
	var loadErr error
	completed := false
	w.chunks.SetLoadObserver(func(x, z int32, err error) { completed, loadErr = true, err })
	w.chunks.RequestLoad(0, 0)
	deadline := time.Now().Add(3 * time.Second)
	for !completed && time.Now().Before(deadline) {
		w.Tick()
		time.Sleep(time.Millisecond)
	}
	if !completed || loadErr == nil || w.chunks.IsTracked(0, 0) || w.entities.Count() != 0 || w.tickScheduler.Pending() != 0 {
		t.Fatalf("partial malformed publication: completed=%v err=%v", completed, loadErr)
	}
}

// TestEntitySnapshotRevisionRejectsLiveMutations verifies immutable entity payload ownership.
func TestEntitySnapshotRevisionRejectsLiveMutations(t *testing.T) {
	c := chunk.New(0, 0)
	w, _ := persistentTestWorld(t, t.TempDir(), c)
	defer func() {
		if err := w.Close(nil); err != nil {
			t.Error(err)
		}
	}()
	item := entity.NewItemEntity(1, protocol.UUID{5, 1}, entity.Vec3{X: 8, Y: 64, Z: 8}, inventory.NewSlot(1, 3))
	w.SpawnItemEntity(item)
	if err := w.refreshEntitySnapshots(); err != nil {
		t.Fatal(err)
	}
	queued, err := w.autosave.enqueue(c)
	if err != nil || !queued {
		t.Fatalf("enqueue %v %v", queued, err)
	}
	item.SetStack(inventory.NewSlot(1, 9))
	if err := w.refreshEntitySnapshots(); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-w.autosave.results:
		if result.err != nil {
			t.Fatal(result.err)
		}
		w.autosave.acknowledge(result)
	case <-time.After(3 * time.Second):
		t.Fatal("save did not finish")
	}
	if !c.Dirty() {
		t.Fatal("stale entity revision was acknowledged")
	}
	data, err := w.chunkStore.ReadEntityNBT(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, root, err := nbt.NewDecoder(data).ReadNamedRootCompound()
	if err != nil {
		t.Fatal(err)
	}
	list, _ := root.GetList("Entities")
	saved, err := entity.DecodePersistentEntity(0, list.Elements()[0].(*nbt.Compound))
	if err != nil {
		t.Fatal(err)
	}
	if saved.(*entity.ItemEntity).Stack().ItemCount != 3 {
		t.Fatal("background writer observed live item mutation")
	}
}

// TestEntityMigrationClearsSourceSnapshot prevents duplication after moving across a region seam.
func TestEntityMigrationClearsSourceSnapshot(t *testing.T) {
	path := t.TempDir()
	source, target := chunk.New(31, -1), chunk.New(32, -1)
	w, storage := persistentTestWorld(t, path, source)
	storage.Set(target)
	item := entity.NewItemEntity(7, protocol.UUID{6, 1}, entity.Vec3{X: 31*16 + 8, Y: 64, Z: -8}, inventory.NewSlot(1, 5))
	w.SpawnItemEntity(item)
	if _, err := w.SaveAllDirty(); err != nil {
		t.Fatal(err)
	}
	item.SetPosition(entity.Vec3{X: 32*16 + 8, Y: 64, Z: -8})
	w.entities.Tick(1)
	if _, err := w.SaveAllDirty(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(nil); err != nil {
		t.Fatal(err)
	}
	restarted, _ := persistentTestWorld(t, path, nil)
	defer func() {
		if err := restarted.Close(nil); err != nil {
			t.Error(err)
		}
	}()
	awaitEntityLoad(t, restarted, 31, -1)
	if restarted.entities.Count() != 0 {
		t.Fatal("source retained migrated entity")
	}
	awaitEntityLoad(t, restarted, 32, -1)
	if restarted.entities.Count() != 1 {
		t.Fatal("destination lost or duplicated migrated entity")
	}
}

// TestDuplicateEntityUUIDRejectsPublication reserves opaque passengers as well as live entities.
func TestDuplicateEntityUUIDRejectsPublication(t *testing.T) {
	w, storage := persistentTestWorld(t, t.TempDir(), chunk.New(0, 0))
	defer func() { _ = w.Close(nil) }()
	item := entity.NewItemEntity(1, protocol.UUID{7, 1}, entity.Vec3{X: 8, Y: 64, Z: 8}, inventory.NewSlot(1, 1))
	w.SpawnItemEntity(item)
	other := chunk.New(1, 0)
	raw, _ := item.PersistentNBT()
	raw.PutList("Pos", nbt.NewList(nbt.TagDouble).Add(float64(24)).Add(float64(64)).Add(float64(8)))
	other.LoadEntityNBT(entityRegionNBT(t, 1, 0, raw))
	if _, err := w.prepareEntityChunk(other); err == nil || w.entities.Count() != 1 {
		t.Fatal("duplicate UUID was accepted")
	}
	opaque := entity.NewEntity(0, protocol.UUID{7, 2}, entity.EntityTypeMob, entity.Vec3{X: 24, Y: 64, Z: 8}, entity.Vec2{})
	parent, _ := opaque.PersistentBaseNBT("minecraft:boat")
	child := entity.NewEntity(0, protocol.UUID{7, 3}, entity.EntityTypeMob, entity.Vec3{X: 24, Y: 65, Z: 8}, entity.Vec2{})
	passenger, _ := child.PersistentBaseNBT("minecraft:unknown")
	parent.PutList("Passengers", nbt.NewList(nbt.TagCompound).Add(passenger))
	other.LoadEntityNBT(entityRegionNBT(t, 1, 0, parent))
	storage.Set(other)
	if err := w.bindChunkEntities(other); err != nil {
		t.Fatal(err)
	}
	duplicate := chunk.New(2, 0)
	passenger.PutList("Pos", nbt.NewList(nbt.TagDouble).Add(float64(40)).Add(float64(64)).Add(float64(8)))
	duplicate.LoadEntityNBT(entityRegionNBT(t, 2, 0, passenger))
	if _, err := w.prepareEntityChunk(duplicate); err == nil {
		t.Fatal("opaque passenger UUID was not reserved")
	}
}

// TestMissingEntityDestinationBlocksSaveAndRecovers retains the last complete source snapshot.
func TestMissingEntityDestinationBlocksSaveAndRecovers(t *testing.T) {
	w, storage := persistentTestWorld(t, t.TempDir(), chunk.New(0, 0))
	defer func() {
		if err := w.Close(nil); err != nil {
			t.Error(err)
		}
	}()
	item := entity.NewItemEntity(1, protocol.UUID{8, 1}, entity.Vec3{X: 8, Y: 64, Z: 8}, inventory.NewSlot(1, 1))
	w.SpawnItemEntity(item)
	if _, err := w.SaveAllDirty(); err != nil {
		t.Fatal(err)
	}
	item.SetPosition(entity.Vec3{X: 24, Y: 64, Z: 8})
	if _, err := w.SaveAllDirty(); err == nil {
		t.Fatal("incomplete migration snapshot was saved")
	}
	source, ok := w.chunks.PeekLoaded(0, 0)
	if !ok || !source.Dirty() {
		t.Fatal("source was not retained for recovery")
	}
	storage.Set(chunk.New(1, 0))
	if _, err := w.SaveAllDirty(); err != nil {
		t.Fatalf("snapshot error poisoned retry: %v", err)
	}
}

// FuzzEntityRegionRestore verifies bounded malformed snapshots cannot partially publish entities.
func FuzzEntityRegionRestore(f *testing.F) {
	root := nbt.NewCompound().PutInt("DataVersion", protocol.CurrentDataVersion).PutIntArray("Position", []int32{0, 0}).PutList("Entities", nbt.NewList(nbt.TagCompound))
	encoder := nbt.NewEncoder(256)
	_ = encoder.WriteNamedRootCompound("", root)
	f.Add(encoder.Bytes())
	dropped := entity.NewItemEntity(1, protocol.UUID{1, 2}, entity.Vec3{X: 8, Y: 64, Z: 8}, inventory.NewSlot(1, 1))
	raw, err := dropped.PersistentNBT()
	if err != nil {
		f.Fatal(err)
	}
	root.PutList("Entities", nbt.NewList(nbt.TagCompound).Add(raw))
	seeded := nbt.NewEncoder(512)
	if err := seeded.WriteNamedRootCompound("", root); err != nil {
		f.Fatal(err)
	}
	f.Add(seeded.Bytes())
	f.Add([]byte{10, 0, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			return
		}
		w := &World{entities: entity.NewManager()}
		c := chunk.NewLoading(0, 0)
		c.LoadEntityNBT(data)
		_, _ = w.prepareEntityChunk(c)
		if w.entities.Count() != 0 {
			t.Fatal("validation published runtime state")
		}
	})
}
