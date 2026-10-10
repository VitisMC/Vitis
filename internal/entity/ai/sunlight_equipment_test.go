package ai

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
)

// sunlightTestWorld controls raw local brightness independently of sky exposure.
type sunlightTestWorld struct {
	actionTestWorld
	brightness uint8
}

// MobBrightness supplies a deterministic local light level for exposure checks.
func (w *sunlightTestWorld) MobBrightness(entity.Vec3) uint8 { return w.brightness }

// TestSunlightGoalsRespectHeadGearExposureAndBreakTicks validates independent live goal decisions.
func TestSunlightGoalsRespectHeadGearExposureAndBreakTicks(t *testing.T) {
	mob := entity.NewMobEntityByName(1, protocol.UUID{1}, "skeleton", entity.Vec3{Y: 64})
	environment := &actionTestWorld{sunlit: true}
	ctx := testContext(mob, nil)
	ctx.Blocks = environment
	burn, avoid := &BurnInSunlightGoal{random: func() float32 { return 0 }}, &AvoidSunlightGoal{}
	stack := inventory.NewSlot(item.IDByName("minecraft:diamond_helmet"), 1)
	stack.SetComponent(dc.ComponentMaxDamage, inventory.VarIntComponent(1))
	if err := mob.SetEquipment(entity.EquipmentHead, stack); err != nil {
		t.Fatal(err)
	}
	if !burn.CanStart(ctx) || avoid.CanStart(ctx) {
		t.Fatal("head gear disabled wear or unnecessarily triggered shade seeking")
	}
	for range 512 {
		burn.Tick(ctx)
		if mob.FireTicks() != 0 {
			t.Fatal("mob ignited while protected, including the break tick")
		}
		if mob.Equipment(entity.EquipmentHead).Empty() {
			break
		}
	}
	if !mob.Equipment(entity.EquipmentHead).Empty() || !avoid.CanStart(ctx) {
		t.Fatal("sunlight did not break the helmet or enable shade seeking")
	}
	burn.Tick(ctx)
	if mob.FireTicks() != 160 {
		t.Fatal("unprotected exposure waited for a packet or arbitrary counter")
	}
	mob.SetFireTicks(0)
	environment.sunlit = false
	burn.Tick(ctx)
	if mob.FireTicks() != 0 || burn.CanContinue(ctx) {
		t.Fatal("stale goal ignited a sheltered mob")
	}
	environment.sunlit = true
	if err := mob.SetEquipment(entity.EquipmentHead, inventory.NewSlot(item.IDByName("minecraft:carved_pumpkin"), 1)); err != nil {
		t.Fatal(err)
	}
	avoid.Start(ctx)
	if avoid.CanContinue(ctx) {
		t.Fatal("shade seeking continued after head protection was equipped")
	}
	mob.SetNoAI(true)
	if !burn.CanStart(ctx) {
		t.Fatal("disabled AI suppressed an environmental effect")
	}
}

// TestSunlightExposureUsesBrightnessProbabilityAndFluids checks environmental protection before wear.
func TestSunlightExposureUsesBrightnessProbabilityAndFluids(t *testing.T) {
	for _, test := range []struct {
		light  uint8
		sample float32
		state  int32
		burns  bool
	}{{15, .1, 0, false}, {15, .01, 0, true}, {11, 0, 0, false}, {12, 0, 0, true}, {13, 0, 0, true}, {15, 0, block.DefaultStateID("minecraft:water"), false}, {15, 0, block.DefaultStateID("minecraft:bubble_column"), false}, {15, 0, block.DefaultStateID("minecraft:powder_snow"), false}} {
		mob := entity.NewMobEntityByName(1, protocol.UUID{1}, "skeleton", entity.Vec3{})
		world := &sunlightTestWorld{actionTestWorld: actionTestWorld{sunlit: true, state: test.state}, brightness: test.light}
		ctx := testContext(mob, nil)
		ctx.Blocks = world
		goal := &BurnInSunlightGoal{random: func() float32 { return test.sample }}
		goal.Tick(ctx)
		if burning := mob.FireTicks() > 0; burning != test.burns {
			t.Fatalf("light=%d sample=%g state=%d burning=%v", test.light, test.sample, test.state, burning)
		}
	}
}
