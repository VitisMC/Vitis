package world

import (
	"math"
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/equipment"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/level"
)

// equipTestPlayer caches a full-stack armor set using the same rules as inventory changes.
func equipTestPlayer(player *entity.Player, windows *inventory.WindowManager, stack inventory.ItemStack) {
	for slot := inventory.ArmorStart; slot <= inventory.ArmorEnd; slot++ {
		windows.Inventory().Set(slot, stack)
	}
	stats := equipment.PlayerArmorStats(windows.Inventory())
	player.Living().SetArmorStats(stats.Defense, stats.Toughness, stats.Protection)
	player.Living().SetArmorDamageProtection(stats.Fire, stats.Blast, stats.Projectile, stats.Fall)
}

// TestPlayerArmorWearUsesAcceptedDamage checks sources, incremental hurt cooldown and bypasses.
func TestPlayerArmorWearUsesAcceptedDamage(t *testing.T) {
	for _, test := range []struct {
		source entity.DamageType
		amount float32
		wear   int32
	}{{entity.DamageTypeMobAttack, 12, 3}, {entity.DamageTypeProjectile, 8, 2}, {entity.DamageTypeExplosion, 16, 4}, {entity.DamageTypePlayerAttack, 4, 1}, {entity.DamageTypeSweep, 1, 1}, {entity.DamageTypeCactus, 1, 1}, {entity.DamageTypeFire, 1, 1}, {entity.DamageTypeFall, 5, 0}, {entity.DamageTypeGeneric, 5, 0}, {entity.DamageTypeFlyIntoWall, 5, 0}, {entity.DamageTypeFireTick, 1, 0}} {
		w, player, windows, _ := survivalTestWorld(t, nil)
		stack := inventory.NewSlot(item.IDByName("minecraft:diamond_chestplate"), 1)
		equipTestPlayer(player, windows, stack)
		living := player.Living()
		source := entity.NewDamageSource(test.source, test.amount)
		living.ApplyDamage(source, living.ArmorDefense(), living.ArmorToughness(), living.ArmorEnchProtection())
		if damage := windows.Inventory().Get(inventory.ArmorStart).Damage; damage != test.wear {
			t.Fatalf("source=%d armor wear=%d expected=%d", test.source, damage, test.wear)
		}
		if actual := living.ApplyDamage(source, living.ArmorDefense(), living.ArmorToughness(), living.ArmorEnchProtection()); actual != 0 || windows.Inventory().Get(inventory.ArmorStart).Damage != test.wear {
			t.Fatal("rejected repeat hit wore equipment")
		}
		if test.wear > 0 {
			source.Amount += 8
			living.ApplyDamage(source, living.ArmorDefense(), living.ArmorToughness(), living.ArmorEnchProtection())
			if windows.Inventory().Get(inventory.ArmorStart).Damage != test.wear+2 {
				t.Fatal("cooldown stronger-hit wear used full damage instead of its increment")
			}
		}
		w.DetachPlayer(player)
	}
}

// TestPlayerArmorWearBreaksSynchronizesAndPersists checks same-hit defense, observers and restart.
func TestPlayerArmorWearBreaksSynchronizesAndPersists(t *testing.T) {
	w, player, windows, sender := survivalTestWorld(t, nil)
	observerSender := &lightTestSender{}
	observer := entity.NewPlayer(101, protocol.UUID{101}, "observer", player.Position(), entity.Vec2{}, observerSender, 2)
	w.AddPlayer(observer)
	observer.TrackedEntities()[player.ID()] = struct{}{}
	stack := inventory.NewSlot(item.IDByName("minecraft:diamond_chestplate"), 1)
	stack.SetComponent(dc.ComponentMaxDamage, inventory.VarIntComponent(2))
	stack.SetComponent(dc.ComponentCustomData, inventory.NBTComponent{Value: nbt.NewCompound().PutString("identity", "saved")})
	equipTestPlayer(player, windows, stack)
	living := player.Living()
	source := entity.NewDamageSource(entity.DamageTypeMobAttack, 4)
	living.ApplyDamage(source, living.ArmorDefense(), living.ArmorToughness(), living.ArmorEnchProtection())
	root := t.TempDir()
	data := level.DefaultPlayerData(protocol.UUIDToString(player.UUID()), 4.5, 64, 4.5, 0)
	for slot := inventory.ArmorStart; slot <= inventory.ArmorEnd; slot++ {
		nbtSlot, _ := level.WindowSlotToNBT(slot)
		data.Inventory = append(data.Inventory, level.InventorySlot{Slot: nbtSlot, Stack: windows.Inventory().Get(slot)})
	}
	if err := data.Save(root); err != nil {
		t.Fatal(err)
	}
	restored, err := level.LoadPlayerData(root, data.UUID)
	if err != nil || len(restored.Inventory) != 4 {
		t.Fatalf("worn armor restart failed: %v", err)
	}
	for _, entry := range restored.Inventory {
		if entry.Stack.Damage != 1 || entry.Stack.MaxStackSize() != 1 {
			t.Fatal("restart lost damage or component defaults")
		}
		copy := entry.Stack.Clone()
		copy.SetDamage(0)
		if !copy.StackableWith(stack) {
			t.Fatal("restart lost armor components")
		}
	}
	for range 11 {
		player.TickLiving()
	}
	sender.packets, observerSender.packets = nil, nil
	before := living.Health()
	if actual := living.ApplyDamage(source, living.ArmorDefense(), living.ArmorToughness(), living.ArmorEnchProtection()); actual != 4 || living.Health() != before-4 || living.ArmorDefense() != 0 {
		t.Fatal("broken armor protected its breaking hit")
	}
	for _, receiver := range []*lightTestSender{sender, observerSender} {
		breaks, updates := 0, 0
		for _, packet := range receiver.packets {
			switch value := packet.(type) {
			case *play.EntityStatus:
				if value.EntityID == player.ID() && value.Status >= 49 && value.Status <= 52 {
					breaks++
				}
			case *play.EntityEquipment:
				if len(value.Slots) != 4 {
					t.Fatal("break omitted cleared slots")
				}
				for _, slot := range value.Slots {
					if !slot.Stack.Empty() || !slot.Empty {
						t.Fatal("break retained client equipment")
					}
				}
				updates++
			}
		}
		if breaks != 4 || updates != 1 {
			t.Fatalf("breaks=%d equipment updates=%d", breaks, updates)
		}
	}
	resynced := false
	for _, packet := range sender.packets {
		if content, ok := packet.(*play.SetContainerContent); ok && content.WindowID == 0 {
			resynced = true
			for slot := inventory.ArmorStart; slot <= inventory.ArmorEnd; slot++ {
				if !content.Slots[slot].Empty() {
					t.Fatal("owner inventory retained broken equipment")
				}
			}
		}
	}
	if !resynced {
		t.Fatal("owner inventory was not synchronized")
	}
	restored.Inventory = nil
	if err := restored.Save(root); err != nil {
		t.Fatal(err)
	}
	cleared, err := level.LoadPlayerData(root, data.UUID)
	if err != nil || len(cleared.Inventory) != 0 {
		t.Fatal("restart resurrected armor from the previous inventory snapshot")
	}
}

// TestRuntimeArmorWearIncludesCombatSourcesAndDefenses covers actual actions rather than isolated calculations.
func TestRuntimeArmorWearIncludesCombatSourcesAndDefenses(t *testing.T) {
	w, player, windows, _ := survivalTestWorld(t, nil)
	equipTestPlayer(player, windows, inventory.NewSlot(item.IDByName("minecraft:diamond_chestplate"), 1))
	mob := w.SpawnMob("zombie", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
	player.SetRotation(entity.Vec2{X: -90})
	player.SetBlocking(true)
	if w.AttackMobTarget(mob, player.ID(), false) || windows.Inventory().Get(5).Damage != 0 {
		t.Fatal("shield block wore armor")
	}
	player.SetBlocking(false)
	if !w.AttackMobTarget(mob, player.ID(), false) || windows.Inventory().Get(5).Damage != 1 {
		t.Fatal("runtime mob attack omitted armor wear")
	}
	for range 11 {
		player.TickLiving()
	}
	w.applyProjectileDamage(player.ID(), entity.NewAttackDamageSource(entity.DamageTypeProjectile, 8, mob.ID()), mob.ID())
	if windows.Inventory().Get(5).Damage != 3 {
		t.Fatal("runtime projectile omitted armor wear")
	}
	for range 11 {
		player.TickLiving()
	}
	w.applyExplosionDamage(entity.ExplosionEntityEffect{EntityID: player.ID(), Damage: 8})
	if windows.Inventory().Get(5).Damage != 5 {
		t.Fatal("runtime explosion omitted armor wear")
	}
	for range 11 {
		player.TickLiving()
	}
	player.Living().SetAbsorption(20)
	w.applyProjectileDamage(player.ID(), entity.NewAttackDamageSource(entity.DamageTypeProjectile, 8, mob.ID()), mob.ID())
	if windows.Inventory().Get(5).Damage != 7 {
		t.Fatal("absorption prevented armor wear")
	}
	player.Living().SetGameMode(1)
	w.applyProjectileDamage(player.ID(), entity.NewAttackDamageSource(entity.DamageTypeProjectile, 20, mob.ID()), mob.ID())
	if windows.Inventory().Get(5).Damage != 7 {
		t.Fatal("creative immunity wore armor")
	}
	player.Living().SetGameMode(0)
	player.Effects().Add(effect.Instance{ID: effectdata.EffectFireResistance, Duration: 20})
	living := player.Living()
	living.ApplyDamage(entity.NewDamageSource(entity.DamageTypeLava, 4), living.ArmorDefense(), living.ArmorToughness(), living.ArmorEnchProtection())
	if windows.Inventory().Get(5).Damage != 7 {
		t.Fatal("fire immunity wore armor")
	}
}

// TestArmorOwnershipFollowsPlayerTransfers removes old callbacks and binds the destination inventory.
func TestArmorOwnershipFollowsPlayerTransfers(t *testing.T) {
	first, player, firstWindows, _ := survivalTestWorld(t, nil)
	second, _, _, _ := survivalTestWorld(t, nil)
	firstStack := inventory.NewSlot(item.IDByName("minecraft:diamond_chestplate"), 1)
	equipTestPlayer(player, firstWindows, firstStack)
	if !first.DetachPlayer(player) {
		t.Fatal("detach failed")
	}
	secondWindows := inventory.NewWindowManager()
	second.SetPlayerInventoryHandler(func(p *entity.Player) *inventory.WindowManager {
		if p == player {
			return secondWindows
		}
		return nil
	})
	if existing := second.entities.Get(player.ID()); existing != nil {
		second.RemovePlayer(second.tracker.Players()[0])
	}
	second.AddPlayer(player)
	equipTestPlayer(player, secondWindows, firstStack)
	if first.DetachPlayer(player) {
		t.Fatal("source world detached the destination's player")
	}
	living := player.Living()
	living.ApplyDamage(entity.NewDamageSource(entity.DamageTypeMobAttack, 4), living.ArmorDefense(), living.ArmorToughness(), living.ArmorEnchProtection())
	if firstWindows.Inventory().Get(5).Damage != 0 || secondWindows.Inventory().Get(5).Damage != 1 {
		t.Fatal("transfer mutated source inventory or lost destination wear")
	}
	second.DetachPlayer(player)
	for range 11 {
		player.TickLiving()
	}
	living.ApplyDamage(entity.NewDamageSource(entity.DamageTypeMobAttack, math.MaxFloat32), living.ArmorDefense(), living.ArmorToughness(), living.ArmorEnchProtection())
	if secondWindows.Inventory().Get(5).Damage != 1 {
		t.Fatal("detached player retained a world mutation callback")
	}
}
