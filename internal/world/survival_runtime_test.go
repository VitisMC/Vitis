package world

import (
	"math"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	ench "github.com/vitismc/vitis/internal/data/generated/enchantment"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/projectile"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/loot"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/worldgen/random"
)

// survivalTestWorld creates a fresh resident world with an active loaded player.
func survivalTestWorld(t *testing.T, resources *loot.Runtime) (*World, *entity.Player, *inventory.WindowManager, *lightTestSender) {
	t.Helper()
	storage := chunk.NewStorage(25)
	loadTestNeighborhood(storage, 0, 0, 2)
	center, _ := storage.Get(0, 0)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			center.SetBlock(x, 63, z, block.DefaultStateID("minecraft:stone"))
		}
	}
	w, err := New(Config{Name: "survival-test", Dimension: "minecraft:overworld", Difficulty: 2, SimulationDistance: 1, LootRuntime: resources, ChunkManager: chunk.NewManager(chunk.ManagerConfig{Storage: storage})})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := w.Close(nil); err != nil {
			t.Error(err)
		}
	})
	sender := &lightTestSender{}
	p := entity.NewPlayer(100, protocol.UUID{100}, "survivor", entity.Vec3{X: 4.5, Y: 64, Z: 4.5}, entity.Vec2{}, sender, 2)
	p.SetClientLoaded(true)
	p.SetOnGround(true)
	w.AddPlayer(p)
	w.updateActivity()
	windows := inventory.NewWindowManager()
	w.SetPlayerInventoryHandler(func(player *entity.Player) *inventory.WindowManager {
		if player == p {
			return windows
		}
		return nil
	})
	return w, p, windows, sender
}

// TestMobCombatAppliesMeleeArrowsAndVisibility connects world actions to actual entities.
func TestMobCombatAppliesMeleeArrowsAndVisibility(t *testing.T) {
	w, p, _, _ := survivalTestWorld(t, nil)
	mob := w.SpawnMob("zombie", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
	if !w.AttackMobTarget(mob, p.ID(), false) || p.Health() != 17 || p.LastAttackerID() != mob.ID() || p.LastDamageType() != entity.DamageTypeMobAttack {
		t.Fatal("mob melee did not apply attributed damage")
	}
	if w.AttackMobTarget(mob, p.ID(), false) {
		t.Fatal("identical repeated hit bypassed invulnerability")
	}
	for range 11 {
		p.TickLiving()
	}
	p.SetRotation(entity.Vec2{X: -90})
	p.SetBlocking(true)
	if w.AttackMobTarget(mob, p.ID(), false) || p.Health() != 17 {
		t.Fatal("frontal shield did not block melee")
	}
	p.SetBlocking(false)
	w.setBlockAt(5, 65, 4, block.DefaultStateID("minecraft:stone"))
	if w.AttackMobTarget(mob, p.ID(), false) {
		t.Fatal("mob attacked through a wall")
	}
	w.setBlockAt(5, 65, 4, 0)
	mob.Remove()
	skeleton := w.SpawnMob("skeleton", entity.Vec3{X: 10.5, Y: 64, Z: 4.5})
	if !w.AttackMobTarget(skeleton, p.ID(), true) || w.projectiles.ArrowCount() != 1 {
		t.Fatal("ranged action did not create an arrow")
	}
	for range 12 {
		w.tickProjectiles()
	}
	if p.Health() >= 17 || p.LastAttackerID() != skeleton.ID() || p.LastDamageType() != entity.DamageTypeProjectile {
		t.Fatalf("arrow did not reach target: health=%v attacker=%d", p.Health(), p.LastAttackerID())
	}
	skeleton.Remove()
	if w.AttackMobTarget(skeleton, p.ID(), true) {
		t.Fatal("removed shooter created another arrow")
	}
}

// TestProjectileShieldUsesImpactDirectionAndPiercing validates defenses at damage execution.
func TestProjectileShieldUsesImpactDirectionAndPiercing(t *testing.T) {
	w, player, _, _ := survivalTestWorld(t, nil)
	player.SetBlocking(true)
	arrow := projectile.NewArrow(w.entities.AllocateID(), protocol.GenerateUUID(), entity.Vec3{X: 4.5, Y: 65, Z: 6.5}, entity.Vec3{Z: -1}, -1, protocol.UUID{})
	w.SpawnArrow(arrow)
	source := entity.NewAttackDamageSource(entity.DamageTypeProjectile, 4, -1)
	source.DirectID, source.DirectType = arrow.ID(), "minecraft:arrow"
	w.applyProjectileDamage(player.ID(), source, -1)
	if player.Health() != 20 {
		t.Fatal("frontal shield did not block the arrow")
	}
	arrow.SetPosition(entity.Vec3{X: 4.5, Y: 65, Z: 2.5})
	w.applyProjectileDamage(player.ID(), source, -1)
	if player.Health() != 16 {
		t.Fatal("shield blocked an arrow from behind")
	}
	for range 11 {
		player.TickLiving()
	}
	arrow.SetPosition(entity.Vec3{X: 4.5, Y: 65, Z: 6.5})
	arrow.SetPiercing(1)
	w.applyProjectileDamage(player.ID(), source, -1)
	if player.Health() != 12 {
		t.Fatal("shield blocked a piercing arrow")
	}
}

// TestCreeperExplosionHonorsMobGriefingAndDoesNotDropDeathRewards verifies world explosion wiring.
func TestCreeperExplosionHonorsMobGriefingAndDoesNotDropDeathRewards(t *testing.T) {
	w, p, _, _ := survivalTestWorld(t, nil)
	if err := w.SetGameRule("mobGriefing", "false"); err != nil {
		t.Fatal(err)
	}
	creeper := w.SpawnMob("creeper", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
	if !w.ExplodeMob(creeper, 3) || w.ExplodeMob(creeper, 3) {
		t.Fatal("creeper explosion was missing or duplicated")
	}
	if !creeper.Removed() || creeper.IsDead() || w.GetBlock(5, 63, 4) != block.DefaultStateID("minecraft:stone") {
		t.Fatal("non-griefing explosion changed blocks or marked a loot-producing death")
	}
	if p.Health() >= 20 || p.LastDamageType() != entity.DamageTypeExplosion || p.LastAttackerID() != creeper.ID() {
		t.Fatal("explosion did not preserve damage attribution")
	}
	w.tickMobs()
	if w.xpOrbs.Count() != 0 || w.items.Count() != 0 {
		t.Fatal("detonation generated death loot")
	}
}

// TestPlayerMobDeathEmitsLootAndXPExactlyOnce verifies the complete fresh-world reward path.
func TestPlayerMobDeathEmitsLootAndXPExactlyOnce(t *testing.T) {
	resources := &datapack.Manager{}
	runtime, err := loot.NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{"../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	w, p, _, sender := survivalTestWorld(t, runtime)
	mob := w.SpawnMob("cow", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
	p.UpdateTracking(w.entities)
	weapon := inventory.NewSlot(item.IDByName("minecraft:diamond_sword"), 1)
	weapon.SetEnchant(ench.EnchantmentIDByName("minecraft:looting"), 3)
	source := entity.NewAttackDamageSource(entity.DamageTypePlayerAttack, 100, p.ID())
	source.AttackerType, source.Weapon = "minecraft:player", weapon
	if !w.AttackMob(p, mob.ID(), source, 3) {
		t.Fatal("player could not kill a resident mob")
	}
	w.tickMobs()
	items, orbs := w.items.Count(), w.xpOrbs.Count()
	if items == 0 || orbs == 0 {
		t.Fatalf("death rewards missing: items=%d orbs=%d", items, orbs)
	}
	for range 25 {
		w.tickMobs()
	}
	if w.items.Count() != items || w.xpOrbs.Count() != orbs {
		t.Fatal("death animation issued duplicate rewards")
	}
	deaths := 0
	for _, packet := range sender.packets {
		if status, ok := packet.(*play.EntityStatus); ok && status.EntityID == mob.ID() && status.Status == 3 {
			deaths++
		}
	}
	if deaths != 1 {
		t.Fatalf("death animations=%d", deaths)
	}
	uncredited := w.SpawnMob("cow", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
	uncredited.Damage(100, "generic")
	w.tickMobs()
	if w.xpOrbs.Count() != orbs {
		t.Fatal("uncredited death granted player XP")
	}
	if err := w.SetGameRule("doMobLoot", "false"); err != nil {
		t.Fatal(err)
	}
	beforeItems := w.items.Count()
	muted := w.SpawnMob("cow", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
	muted.Damage(100, "generic")
	w.tickMobs()
	if w.items.Count() != beforeItems || w.xpOrbs.Count() != orbs {
		t.Fatal("doMobLoot=false still generated rewards")
	}
}

// TestBabyMobDeathExperienceUsesSpeciesRulesOnce exercises credited deaths through the world reward path.
func TestBabyMobDeathExperienceUsesSpeciesRulesOnce(t *testing.T) {
	for _, name := range []string{"zombie", "husk", "drowned", "cow"} {
		t.Run(name, func(t *testing.T) {
			w, player, _, _ := survivalTestWorld(t, nil)
			mob := entity.NewMobEntityByName(w.entities.AllocateID(), protocol.GenerateUUID(), name, entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
			raw, err := mob.PersistentNBT()
			if err != nil {
				t.Fatal(err)
			}
			if name == "cow" {
				raw.PutInt("Age", -24000)
			} else {
				raw.PutBool("IsBaby", true)
			}
			restored, err := entity.DecodePersistentEntity(mob.ID(), raw)
			if err != nil {
				t.Fatal(err)
			}
			baby := restored.(*entity.MobEntity)
			w.mobs.Add(baby)
			source := entity.NewAttackDamageSource(entity.DamageTypePlayerAttack, 100, player.ID())
			source.AttackerType = "minecraft:player"
			if !w.AttackMob(player, baby.ID(), source, 3) {
				t.Fatal("credited baby death failed")
			}
			w.tickMobs()
			want := int32(12)
			if name == "cow" {
				want = 0
			}
			total := int32(0)
			for _, orb := range w.xpOrbs.All() {
				total += orb.XPValue() * orb.Count()
			}
			if total != want {
				t.Fatalf("experience=%d want=%d", total, want)
			}
			for range 25 {
				w.tickMobs()
			}
			total = 0
			for _, orb := range w.xpOrbs.All() {
				total += orb.XPValue() * orb.Count()
			}
			if total != want {
				t.Fatal("death animation duplicated experience")
			}
		})
	}
}

// TestExperiencePickupRepairsEquippedMendingThenAwardsRemainder covers count, delay and persistence.
func TestExperiencePickupRepairsEquippedMendingThenAwardsRemainder(t *testing.T) {
	w, p, windows, _ := survivalTestWorld(t, nil)
	stack := inventory.NewSlot(item.IDByName("minecraft:diamond_pickaxe"), 1)
	stack.SetDamage(3)
	stack.SetEnchant(ench.EnchantmentIDByName("minecraft:mending"), 1)
	windows.Inventory().Set(inventory.HotbarStart, stack)
	orb := entity.NewXPOrb(w.entities.AllocateID(), protocol.GenerateUUID(), p.Position(), 7)
	if !orb.AddAwards(1) {
		t.Fatal("failed to add a counted award")
	}
	w.SpawnXPOrb(orb)
	w.pickupXPOrbs()
	if windows.HeldItem().Damage != 0 || p.XPTotal() != 6 || orb.Count() != 1 || orb.Removed() {
		t.Fatalf("repair/pickup state: damage=%d xp=%d count=%d", windows.HeldItem().Damage, p.XPTotal(), orb.Count())
	}
	w.pickupXPOrbs()
	if p.XPTotal() != 6 {
		t.Fatal("pickup delay was ignored")
	}
	p.TickClientLoadTimeout()
	p.TickClientLoadTimeout()
	w.pickupXPOrbs()
	if p.XPTotal() != 13 || w.xpOrbs.Count() != 0 {
		t.Fatal("remaining award was lost or duplicated")
	}
	raw, err := inventory.ItemStackToNBT(windows.HeldItem())
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := inventory.ItemStackFromNBT(raw)
	if err != nil || loaded.Damage != 0 || loaded.EnchantLevel(ench.EnchantmentIDByName("minecraft:mending")) != 1 {
		t.Fatal("repaired equipment did not survive persistent encoding")
	}
}

// TestOrdinaryExperienceAwardUsesIndividualMergeAttempts preserves normal death-orb counts.
func TestOrdinaryExperienceAwardUsesIndividualMergeAttempts(t *testing.T) {
	w, player, _, _ := survivalTestWorld(t, nil)
	w.randomTicks = random.NewLegacy(0)
	w.AwardExperience(player.Position(), 6)
	var total int32
	for _, orb := range w.xpOrbs.All() {
		if orb.XPValue() != 3 || orb.Count() != 1 {
			t.Fatal("ordinary awards were unconditionally batched")
		}
		total += orb.XPValue()
	}
	if total != 6 || w.xpOrbs.Count() != 2 {
		t.Fatal("ordinary death award lost value or orb count")
	}
}

// TestExperienceAwardBoundsEntityCountAndPreservesValue covers large commands without tick stalls.
func TestExperienceAwardBoundsEntityCountAndPreservesValue(t *testing.T) {
	w, p, _, _ := survivalTestWorld(t, nil)
	w.AwardExperience(p.Position(), math.MaxInt32)
	var sum int64
	for _, orb := range w.xpOrbs.All() {
		sum += int64(orb.XPValue()) * int64(orb.Count())
		if _, err := orb.PersistentNBT(); err != nil {
			t.Fatal(err)
		}
	}
	if sum != math.MaxInt32 || w.xpOrbs.Count() > 11 {
		t.Fatalf("award sum=%d entities=%d", sum, w.xpOrbs.Count())
	}
}
