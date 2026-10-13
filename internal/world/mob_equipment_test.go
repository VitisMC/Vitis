package world

import (
	"math"
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// TestMobEquipmentSynchronizesSpawnChangesAndEmptySlots checks actual observer packets and ownership.
func TestMobEquipmentSynchronizesSpawnChangesAndEmptySlots(t *testing.T) {
	w, player, _, sender := survivalTestWorld(t, nil)
	hiddenSender := &lightTestSender{}
	hidden := entity.NewPlayer(101, protocol.UUID{101}, "hidden", player.Position(), entity.Vec2{}, hiddenSender, 2)
	w.AddPlayer(hidden)
	mob := w.SpawnMob("skeleton", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
	w.tracker.TickWithChunkFilter(w.entities, func(id, x, z int32) bool { return id == player.ID() && x == 0 && z == 0 })
	found := false
	for _, packet := range sender.packets {
		if gear, ok := packet.(*play.EntityEquipment); ok && gear.EntityID == mob.ID() {
			if len(gear.Slots) != 7 || gear.Slots[0].Stack.ItemID != item.IDByName("minecraft:bow") {
				t.Fatal("spawn gear missing")
			}
			found = true
		}
	}
	if !found || len(hiddenSender.packets) != 0 {
		t.Fatal("spawn gear leaked or omitted")
	}
	sender.packets = nil
	stack := inventory.NewSlot(item.IDByName("minecraft:diamond_chestplate"), 1)
	stack.SetDamage(7)
	stack.SetComponent(dc.ComponentCustomData, inventory.NBTComponent{Value: nbt.NewCompound().PutString("identity", "retained")})
	if err := mob.SetEquipment(entity.EquipmentChest, stack); err != nil {
		t.Fatal(err)
	}
	w.syncMobStates()
	var update *play.EntityEquipment
	for _, packet := range sender.packets {
		if gear, ok := packet.(*play.EntityEquipment); ok && gear.EntityID == mob.ID() {
			update = gear
		}
	}
	if update == nil || len(update.Slots) != 1 || update.Slots[0].SlotID != 4 || !update.Slots[0].Stack.StackableWith(stack) {
		t.Fatal("changed components not synchronized")
	}
	if err := mob.SetEquipment(entity.EquipmentChest, inventory.EmptySlot()); err != nil {
		t.Fatal(err)
	}
	w.syncMobStates()
	if !update.Slots[0].Stack.StackableWith(stack) {
		t.Fatal("pending packet shares mutable equipment")
	}
	var last *play.EntityEquipment
	for _, packet := range sender.packets {
		if gear, ok := packet.(*play.EntityEquipment); ok {
			last = gear
		}
	}
	if last == nil || len(last.Slots) != 1 || !last.Slots[0].Empty || len(hiddenSender.packets) != 0 {
		t.Fatal("cleared gear retained client state")
	}
	buffer := protocol.NewBuffer(128)
	if err := last.Encode(buffer); err != nil {
		t.Fatal(err)
	}
	decoded := &play.EntityEquipment{}
	if err := decoded.Decode(protocol.WrapBuffer(buffer.Bytes())); err != nil || !decoded.Slots[0].Stack.Empty() {
		t.Fatal("empty slot wire encoding failed")
	}
}

// TestMobEquipmentDeathDropsAndExperienceOnce covers component rewards, game rules and removal.
func TestMobEquipmentDeathDropsAndExperienceOnce(t *testing.T) {
	w, player, _, _ := survivalTestWorld(t, nil)
	mob := w.SpawnMob("skeleton", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
	mob.SetNoAI(true)
	stack := inventory.NewSlot(item.IDByName("minecraft:diamond_sword"), 1)
	stack.SetDamage(7)
	stack.SetComponent(dc.ComponentCustomData, inventory.NBTComponent{Value: nbt.NewCompound().PutString("identity", "death")})
	if err := mob.SetEquipment(entity.EquipmentMainHand, stack); err != nil {
		t.Fatal(err)
	}
	if err := mob.SetEquipmentDropChance(entity.EquipmentMainHand, 2); err != nil {
		t.Fatal(err)
	}
	source := entity.NewAttackDamageSource(entity.DamageTypePlayerAttack, 100, player.ID())
	source.AttackerType = "minecraft:player"
	if !w.AttackMob(player, mob.ID(), source, 3) {
		t.Fatal("kill failed")
	}
	w.tickMobs()
	if w.items.Count() != 1 || !mob.Equipment(entity.EquipmentMainHand).Empty() {
		t.Fatal("gear missing from runtime death")
	}
	for id := range w.entities.Entities() {
		if dropped := w.items.Get(id); dropped != nil && !dropped.Stack().StackableWith(stack) {
			t.Fatal("death lost components or forced durability")
		}
	}
	xp := int32(0)
	for _, orb := range w.xpOrbs.All() {
		xp += orb.XPValue() * orb.Count()
	}
	if xp != 5 {
		t.Fatalf("forced gear added XP: %d", xp)
	}
	for range 25 {
		w.tickMobs()
	}
	remainingXP := int32(0)
	for _, orb := range w.xpOrbs.All() {
		remainingXP += orb.XPValue() * orb.Count()
	}
	if w.items.Count() != 1 || remainingXP != xp {
		t.Fatal("death animation repeated equipment reward")
	}
	if err := w.SetGameRule("doMobLoot", "false"); err != nil {
		t.Fatal(err)
	}
	second := w.SpawnMob("wither_skeleton", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
	if err := second.SetEquipmentDropChance(entity.EquipmentMainHand, 2); err != nil {
		t.Fatal(err)
	}
	second.SetNoAI(true)
	if !w.AttackMob(player, second.ID(), source, 3) {
		t.Fatal("second kill failed")
	}
	w.tickMobs()
	third := w.SpawnMob("wither_skeleton", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
	third.Remove()
	w.tickMobs()
	remainingXP = 0
	for _, orb := range w.xpOrbs.All() {
		remainingXP += orb.XPValue() * orb.Count()
	}
	if w.items.Count() != 1 || remainingXP != xp {
		t.Fatal("game rule or removal issued equipment drops")
	}
}

// TestEquipmentArmorAndWeaponsAffectRuntimeCombat checks accepted hits and saved weapon identity.
func TestEquipmentArmorAndWeaponsAffectRuntimeCombat(t *testing.T) {
	w, player, _, _ := survivalTestWorld(t, nil)
	mob := w.SpawnMob("husk", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
	weapon := inventory.NewSlot(item.IDByName("minecraft:diamond_sword"), 1)
	if err := mob.SetEquipment(entity.EquipmentMainHand, weapon); err != nil {
		t.Fatal(err)
	}
	if !w.AttackMobTarget(mob, player.ID(), false) || player.Health() != 11 || !player.DeathWeapon().StackableWith(weapon) || player.Effects().Has(effectdata.EffectHunger) {
		t.Fatal("weapon damage, attribution or Hunger gate failed")
	}
	if err := mob.SetEquipment(entity.EquipmentChest, inventory.NewSlot(item.IDByName("minecraft:diamond_chestplate"), 1)); err != nil {
		t.Fatal(err)
	}
	if err := mob.SetEquipment(entity.EquipmentFeet, inventory.NewSlot(item.IDByName("minecraft:netherite_boots"), 1)); err != nil {
		t.Fatal(err)
	}
	mob.SetOnGround(true)
	source := entity.NewAttackDamageSource(entity.DamageTypePlayerAttack, 8, player.ID())
	source.AttackerType = "minecraft:player"
	if !w.AttackMob(player, mob.ID(), source, 3) || mob.Health() <= 12 || mob.Health() >= 20 || math.Abs(mob.Velocity().X-.36) > 1e-7 {
		t.Fatal("armor or equipment knockback resistance ignored")
	}
	if w.AttackMob(player, mob.ID(), source, 3) {
		t.Fatal("equipment bypassed repeated-hit protection")
	}
	if mob.Equipment(entity.EquipmentChest).Damage != 0 || mob.Equipment(entity.EquipmentFeet).Damage != 0 {
		t.Fatal("ordinary mob hits incorrectly wore humanoid armor")
	}
}

// TestHuskHungerUsesAcceptedUnarmedDamageAndLocalDifficulty covers all server difficulties.
func TestHuskHungerUsesAcceptedUnarmedDamageAndLocalDifficulty(t *testing.T) {
	for _, test := range []struct{ difficulty, duration int32 }{{0, 0}, {1, 0}, {2, 140}, {3, 280}} {
		w, player, _, _ := survivalTestWorld(t, nil)
		w.SetDifficulty(test.difficulty)
		mob := w.SpawnMob("husk", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
		player.SetRotation(entity.Vec2{X: -90})
		player.SetBlocking(true)
		if w.AttackMobTarget(mob, player.ID(), false) || player.Effects().Has(effectdata.EffectHunger) {
			t.Fatal("shielded hit applied Hunger")
		}
		player.SetBlocking(false)
		if accepted := w.AttackMobTarget(mob, player.ID(), false); accepted != (test.difficulty != 0) {
			t.Fatal("difficulty hit acceptance failed")
		}
		instance := player.Effects().Get(effectdata.EffectHunger)
		if test.duration == 0 && instance != nil || test.duration > 0 && (instance == nil || instance.Duration != test.duration) {
			t.Fatalf("difficulty=%d Hunger=%+v", test.difficulty, instance)
		}
		player.Effects().Remove(effectdata.EffectHunger)
		if w.AttackMobTarget(mob, player.ID(), false) || player.Effects().Has(effectdata.EffectHunger) {
			t.Fatal("repeated hit renewed Hunger")
		}
	}
}
