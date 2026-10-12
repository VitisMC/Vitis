package world

import (
	"math/rand"
	"testing"

	trackeddata "github.com/vitismc/vitis/internal/data/generated/tracked_data"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/metadata"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/loot"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// TestSlimeContactUsesSizeRangeAndDamage validates final world actions independently of AI requests.
func TestSlimeContactUsesSizeRangeAndDamage(t *testing.T) {
	w, player, _, _ := survivalTestWorld(t, nil)
	mob := w.SpawnMob("slime", entity.Vec3{X: 5, Y: 64, Z: 4.5})
	if w.AttackMobTarget(mob, player.ID(), false) || player.Health() != 20 {
		t.Fatal("tiny slime damaged the player")
	}
	mob.SetSlimeSize(4, true)
	if !w.AttackMobTarget(mob, player.ID(), false) || player.Health() != 16 || player.LastAttackerID() != mob.ID() {
		t.Fatal("large slime did not deal size-dependent contact damage")
	}
	if w.AttackMobTarget(mob, player.ID(), false) || player.Health() != 16 {
		t.Fatal("contact bypassed repeated-hit protection")
	}
	for range 11 {
		player.TickLiving()
	}
	mob.SetPosition(entity.Vec3{X: 6.3, Y: 64, Z: 4.5})
	if w.AttackMobTarget(mob, player.ID(), false) || w.AttackMobTarget(mob, player.ID(), true) {
		t.Fatal("slime attacked beyond its body contact or fired an arrow")
	}
}

// TestSlimeBrainExecutesContactDamageOnWorldTick checks the complete AI-to-combat connection.
func TestSlimeBrainExecutesContactDamageOnWorldTick(t *testing.T) {
	w, player, _, _ := survivalTestWorld(t, nil)
	mob := w.SpawnMob("slime", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
	mob.SetSlimeSize(4, true)
	mob.SetOnGround(true)
	w.tickMobs()
	if player.Health() != 16 || mob.Velocity().Y <= 0 || player.LastAttackerID() != mob.ID() {
		t.Fatal("world tick did not execute the slime's hop and contact attack")
	}
	for range 5 {
		w.tickMobs()
	}
	if player.Health() != 16 {
		t.Fatal("world-tick contact bypassed damage immunity")
	}
}

// slimeMetadataSize decodes the supported living metadata sent to observers.
func slimeMetadataSize(t *testing.T, encoded []byte) (int32, bool) {
	t.Helper()
	buffer := protocol.WrapBuffer(encoded)
	for {
		index, err := buffer.ReadByte()
		if err != nil {
			t.Fatal(err)
		}
		if index == metadata.EndMarker {
			return 0, false
		}
		kind, err := buffer.ReadVarInt()
		if err != nil {
			t.Fatal(err)
		}
		switch kind {
		case metadata.TypeByte:
			_, err = buffer.ReadByte()
		case metadata.TypeFloat:
			_, err = buffer.ReadFloat32()
		case metadata.TypeVarInt:
			var value int32
			value, err = buffer.ReadVarInt()
			if err == nil && index == trackeddata.SlimeIdSize {
				return value, true
			}
		default:
			t.Fatalf("unsupported test metadata type %d", kind)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

// TestSlimeStateSynchronizesAndSplitSurvivesDeathReload verifies metadata, brain registration and one-time rewards.
func TestSlimeStateSynchronizesAndSplitSurvivesDeathReload(t *testing.T) {
	resources := &datapack.Manager{}
	runtime, err := loot.NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{"../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	w, player, _, sender := survivalTestWorld(t, runtime)
	parent := w.SpawnMob("slime", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
	parent.SetSlimeSize(4, true)
	parent.SetNoAI(true)
	w.sendEntitySpawnState(player, parent.Entity)
	seenSize, seenMaxHealth := false, false
	for _, packet := range sender.packets {
		switch value := packet.(type) {
		case *play.SetEntityMetadata:
			if size, ok := slimeMetadataSize(t, value.Metadata); ok && size == 4 {
				seenSize = true
			}
		case *play.EntityUpdateAttributes:
			for _, attribute := range value.Properties {
				seenMaxHealth = seenMaxHealth || attribute.Value == 16
			}
		}
	}
	if !seenSize || !seenMaxHealth {
		t.Fatal("spawn did not publish size and attributes")
	}
	source := entity.NewAttackDamageSource(entity.DamageTypePlayerAttack, 100, player.ID())
	source.AttackerType = "minecraft:player"
	if !w.AttackMob(player, parent.ID(), source, 3) {
		t.Fatal("large slime could not be killed")
	}
	for range 5 {
		w.tickMobs()
	}
	beforeXP := w.xpOrbs.Count()
	if beforeXP == 0 || w.items.Count() != 0 {
		t.Fatal("large slime did not award XP or incorrectly dropped slime balls")
	}
	root, err := parent.PersistentNBT()
	if err != nil {
		t.Fatal(err)
	}
	w.mobs.Evict(parent.ID())
	w.entities.Remove(parent.ID())
	restored, err := entity.DecodePersistentEntity(w.entities.AllocateID(), root)
	if err != nil {
		t.Fatal(err)
	}
	w.mobs.Add(restored.(*entity.MobEntity))
	for range 25 {
		w.tickMobs()
	}
	if w.mobs.Count() < 2 || w.mobs.Count() > 4 || w.xpOrbs.Count() != beforeXP || w.items.Count() != 0 {
		t.Fatal("death reload lost the split or repeated rewards")
	}
	for _, child := range w.mobs.Mobs() {
		if child.SlimeSize() != 2 || child.IsDead() || child.HasPlayerKillCredit() || w.mobBrains.Get(child.ID()) == nil {
			t.Fatal("split child lost size, life state or brain registration")
		}
		if _, err := child.PersistentNBT(); err != nil {
			t.Fatal(err)
		}
	}
	ctx := loot.NewContext(rand.New(rand.NewSource(1)))
	ctx.Entity = &loot.EntityInfo{Type: "minecraft:slime", Size: 1}
	balls := int32(0)
	for range 30 {
		for _, stack := range runtime.RollStacks("minecraft:entities/slime", ctx) {
			if item.NameByID(stack.ItemID) != "minecraft:slime_ball" {
				t.Fatal("tiny slime produced the wrong item")
			}
			balls += stack.ItemCount
		}
	}
	if balls == 0 {
		t.Fatal("tiny slime loot predicate never matched")
	}
}

// TestChunkInhabitationAdvancesOnceAndStopsWithoutEligiblePlayers verifies the connected tick policy.
func TestChunkInhabitationAdvancesOnceAndStopsWithoutEligiblePlayers(t *testing.T) {
	w, player, _, _ := survivalTestWorld(t, nil)
	center, ok := w.chunks.PeekLoaded(0, 0)
	if !ok {
		t.Fatal("missing resident chunk")
	}
	w.tickChunkInhabitation()
	if center.InhabitedTime() != 1 || w.SpawnInhabitedTime(4, 4) != 1 {
		t.Fatal("resident spawn-candidate chunk did not accumulate local difficulty")
	}
	player.SetGameMode(3)
	w.tickChunkInhabitation()
	if center.InhabitedTime() != 1 {
		t.Fatal("spectator advanced local difficulty")
	}
	player.SetGameMode(0)
	player.SetClientLoaded(false)
	w.tickChunkInhabitation()
	if center.InhabitedTime() != 1 || w.SpawnInhabitedTime(10000, 10000) != 0 {
		t.Fatal("unavailable player or chunk advanced difficulty")
	}
}
