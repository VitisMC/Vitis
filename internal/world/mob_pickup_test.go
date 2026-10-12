package world

import (
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// dropPickupStack registers a resident source item with deterministic position and pickup timing.
func dropPickupStack(w *World, pos entity.Vec3, stack inventory.ItemStack, delay int32) *entity.ItemEntity {
	dropped := entity.NewItemEntityWithVelocity(w.entities.AllocateID(), protocol.GenerateUUID(), pos, stack, entity.Vec3{}, delay)
	w.SpawnItemEntity(dropped)
	return dropped
}

// TestRuntimeMobPickupTransfersOnceAndSynchronizes checks real simulation, full stacks and competing mobs.
func TestRuntimeMobPickupTransfersOnceAndSynchronizes(t *testing.T) {
	w, player, _, sender := survivalTestWorld(t, nil)
	mob := w.SpawnMob("husk", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
	mob.SetNoAI(true)
	mob.SetCanPickUpLoot(true)
	second := w.SpawnMob("husk", mob.Position())
	second.SetNoAI(true)
	second.SetCanPickUpLoot(true)
	stack := inventory.NewSlot(item.IDByName("minecraft:diamond_sword"), 1)
	stack.SetDamage(7)
	stack.SetComponent(dc.ComponentCustomData, inventory.NBTComponent{Value: nbt.NewCompound().PutString("identity", "runtime")})
	dropped := dropPickupStack(w, mob.Position(), stack, 0)
	player.TrackedEntities()[dropped.ID()] = struct{}{}
	player.TrackedEntities()[mob.ID()] = struct{}{}
	player.TrackedEntities()[second.ID()] = struct{}{}
	w.tickMobs()
	w.tickMobs()
	if w.items.Count() != 0 || !dropped.Removed() || !mob.Equipment(entity.EquipmentMainHand).StackableWith(stack) || !second.Equipment(entity.EquipmentMainHand).Empty() || !mob.PersistenceRequired() {
		t.Fatal("source was lost, duplicated or equipped by both mobs")
	}
	collected, equipped := 0, 0
	for _, packet := range sender.packets {
		switch value := packet.(type) {
		case *play.CollectItem:
			if value.CollectedEntityID == dropped.ID() {
				if value.CollectorEntityID != mob.ID() || value.PickupItemCount != 1 {
					t.Fatal("pickup animation has the wrong collector or count")
				}
				collected++
			}
		case *play.EntityEquipment:
			for _, slot := range value.Slots {
				if value.EntityID == mob.ID() && slot.SlotID == 0 && slot.Stack.StackableWith(stack) {
					buffer := protocol.NewBuffer(128)
					if err := value.Encode(buffer); err != nil {
						t.Fatal(err)
					}
					decoded := &play.EntityEquipment{}
					if err := decoded.Decode(protocol.WrapBuffer(buffer.Bytes())); err != nil || !decoded.Slots[0].Stack.StackableWith(stack) {
						t.Fatal("equipment wire encoding lost pickup components")
					}
					equipped++
				}
			}
		}
	}
	if collected != 1 || equipped != 1 {
		t.Fatalf("collected=%d equipment=%d", collected, equipped)
	}
}

// TestRuntimeMobPickupPreservesRemaindersAndDropDelays verifies limited armor and old-equipment ownership.
func TestRuntimeMobPickupPreservesRemaindersAndDropDelays(t *testing.T) {
	w, player, _, sender := survivalTestWorld(t, nil)
	mob := w.SpawnMob("husk", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
	mob.SetCanPickUpLoot(true)
	mob.SetNoAI(true)
	stack := inventory.NewSlot(item.IDByName("minecraft:iron_helmet"), 5)
	stack.RemoveComponent(dc.ComponentMaxDamage)
	stack.RemoveComponent(dc.ComponentDamage)
	stack.SetComponent(dc.ComponentMaxStackSize, inventory.VarIntComponent(16))
	dropped := dropPickupStack(w, mob.Position(), stack, 0)
	player.TrackedEntities()[dropped.ID()] = struct{}{}
	w.pickupMobEquipment()
	w.syncMobStates()
	if dropped.Stack().ItemCount != 4 || mob.Equipment(entity.EquipmentHead).ItemCount != 1 {
		t.Fatal("limited armor pickup consumed the complete source stack")
	}
	metadataSent := false
	for _, packet := range sender.packets {
		if value, ok := packet.(*play.SetEntityMetadata); ok && value.EntityID == dropped.ID() {
			metadataSent = true
		}
	}
	if !metadataSent {
		t.Fatal("source remainder was not synchronized")
	}
	old := inventory.NewSlot(item.IDByName("minecraft:iron_sword"), 1)
	old.SetDamage(7)
	if err := mob.SetEquipment(entity.EquipmentMainHand, old); err != nil {
		t.Fatal(err)
	}
	if err := mob.SetEquipmentDropChance(entity.EquipmentMainHand, 2); err != nil {
		t.Fatal(err)
	}
	newStack := inventory.NewSlot(item.IDByName("minecraft:diamond_sword"), 1)
	source := dropPickupStack(w, mob.Position(), newStack, 0)
	w.pickupMobEquipment()
	if !source.Removed() || !mob.Equipment(entity.EquipmentMainHand).StackableWith(newStack) {
		t.Fatal("weapon upgrade failed")
	}
	oldDrops := 0
	for _, dropped := range w.items.All() {
		if dropped.Stack().StackableWith(old) {
			if dropped.PickupDelay() != 10 || dropped.Stack().Damage != 7 {
				t.Fatal("replaced equipment lost durability or pickup delay")
			}
			oldDrops++
		}
	}
	if oldDrops != 1 {
		t.Fatal("replaced equipment was lost or issued twice")
	}
}

// TestMobPickupRespectsRulesActivityAndVerticalReach checks ignored requests without consuming sources.
func TestMobPickupRespectsRulesActivityAndVerticalReach(t *testing.T) {
	for _, reason := range []string{"rule", "disabled", "delay", "dead", "removed", "inactive", "above", "far"} {
		w, _, _, _ := survivalTestWorld(t, nil)
		mob := w.SpawnMob("husk", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
		mob.SetCanPickUpLoot(true)
		pos := mob.Position()
		delay := int32(0)
		switch reason {
		case "rule":
			if err := w.SetGameRule("mobGriefing", "false"); err != nil {
				t.Fatal(err)
			}
		case "disabled":
			mob.SetCanPickUpLoot(false)
		case "delay":
			delay = 1
		case "dead":
			mob.ApplyDamage(entity.NewDamageSource(entity.DamageTypeGeneric, 100), 0, 0, 0)
		case "removed":
			mob.Remove()
		case "inactive":
			pos = entity.Vec3{X: 80.5, Y: 64, Z: 80.5}
			mob.SetPosition(pos)
		case "above":
			pos.Y += 2.5
		case "far":
			pos.X += 2.5
		}
		stack := inventory.NewSlot(item.IDByName("minecraft:diamond_sword"), 1)
		dropped := dropPickupStack(w, pos, stack, delay)
		w.pickupMobEquipment()
		if dropped.Removed() || !dropped.Stack().StackableWith(stack) || !mob.Equipment(entity.EquipmentMainHand).Empty() || mob.PersistenceRequired() {
			t.Fatalf("%s performed a forbidden pickup", reason)
		}
	}
}

// TestMobPickupBudgetResumesBeyondRejectedItems checks predictable work without prefix starvation.
func TestMobPickupBudgetResumesBeyondRejectedItems(t *testing.T) {
	w, _, _, _ := survivalTestWorld(t, nil)
	mob := w.SpawnMob("husk", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
	mob.SetCanPickUpLoot(true)
	if err := mob.SetEquipment(entity.EquipmentMainHand, inventory.NewSlot(item.IDByName("minecraft:diamond_sword"), 1)); err != nil {
		t.Fatal(err)
	}
	for range maxMobPickupChecks {
		dropPickupStack(w, mob.Position(), inventory.NewSlot(item.IDByName("minecraft:stone"), 1), 0)
	}
	better := dropPickupStack(w, mob.Position(), inventory.NewSlot(item.IDByName("minecraft:netherite_sword"), 1), 0)
	w.pickupMobEquipment()
	if better.Removed() {
		t.Fatal("pickup exceeded its per-tick candidate budget")
	}
	w.pickupMobEquipment()
	if !better.Removed() || mob.Equipment(entity.EquipmentMainHand).ItemID != item.IDByName("minecraft:netherite_sword") {
		t.Fatal("candidate cursor repeatedly scanned the rejected prefix")
	}
}

// TestNaturalPickupCapabilityUsesRegionalDifficulty checks fresh-world and late-game spawn settings.
func TestNaturalPickupCapabilityUsesRegionalDifficulty(t *testing.T) {
	w, _, _, _ := survivalTestWorld(t, nil)
	mob := w.SpawnMob("husk", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
	for _, difficulty := range []int32{0, 1, 2} {
		w.SetDifficulty(difficulty)
		for range 100 {
			w.initializeMobPickup(mob)
			if mob.CanPickUpLoot() {
				t.Fatal("fresh low regional difficulty enabled random pickup")
			}
		}
	}
	w.SetDifficulty(3)
	w.worldAge.Store(2_000_000)
	enabled := 0
	for range 1000 {
		w.initializeMobPickup(mob)
		if mob.CanPickUpLoot() {
			enabled++
		}
	}
	if enabled < 300 || enabled > 650 || mob.PersistenceRequired() {
		t.Fatalf("regional pickup initialization enabled=%d or incorrectly marked persistence", enabled)
	}
}

// TestPickedGearDropsOnceWithoutPlayerCredit checks environmental death and removal after pickup.
func TestPickedGearDropsOnceWithoutPlayerCredit(t *testing.T) {
	for _, cause := range []string{"death", "loot_disabled", "removed"} {
		w, _, _, _ := survivalTestWorld(t, nil)
		mob := w.SpawnMob("husk", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
		mob.SetNoAI(true)
		mob.SetCanPickUpLoot(true)
		stack := inventory.NewSlot(item.IDByName("minecraft:diamond_sword"), 1)
		stack.SetDamage(7)
		dropPickupStack(w, mob.Position(), stack, 0)
		w.pickupMobEquipment()
		if w.items.Count() != 0 || !mob.PersistenceRequired() {
			t.Fatal("test pickup failed")
		}
		if cause == "loot_disabled" {
			if err := w.SetGameRule("doMobLoot", "false"); err != nil {
				t.Fatal(err)
			}
		}
		if cause == "removed" {
			mob.Remove()
		} else if mob.ApplyDamage(entity.NewDamageSource(entity.DamageTypeGeneric, 100), mob.ArmorDefense(), mob.ArmorToughness(), mob.ArmorEnchProtection()) <= 0 {
			t.Fatal("environmental kill failed")
		}
		for range 25 {
			w.tickMobs()
		}
		want := 0
		if cause == "death" {
			want = 1
		}
		if w.items.Count() != want || len(w.xpOrbs.All()) != 0 {
			t.Fatalf("%s items=%d XP=%d", cause, w.items.Count(), len(w.xpOrbs.All()))
		}
		for _, dropped := range w.items.All() {
			if !dropped.Stack().StackableWith(stack) || dropped.Stack().Damage != 7 {
				t.Fatal("guaranteed reward lost picked components")
			}
		}
	}
}

// TestMergedItemStackIsPublished checks the surviving entity's full metadata after merging.
func TestMergedItemStackIsPublished(t *testing.T) {
	w, player, _, sender := survivalTestWorld(t, nil)
	stack := inventory.NewSlot(item.IDByName("minecraft:stone"), 7)
	pos := entity.Vec3{X: 5.5, Y: 64, Z: 4.5}
	left, right := dropPickupStack(w, pos, stack, 0), dropPickupStack(w, pos, stack, 0)
	player.TrackedEntities()[left.ID()] = struct{}{}
	if !left.TryMerge(right) {
		t.Fatal("merge failed")
	}
	right.Remove()
	w.tickItems()
	updates := 0
	for _, packet := range sender.packets {
		if value, ok := packet.(*play.SetEntityMetadata); ok && value.EntityID == left.ID() {
			updates++
		}
	}
	if updates != 1 || left.Stack().ItemCount != 14 || w.items.Count() != 1 {
		t.Fatal("item merge was not published or removed its surviving entity")
	}
}
