package projectile

import (
	"reflect"
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	potiondata "github.com/vitismc/vitis/internal/data/generated/potion"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestArrowPotionComponentsRoundTrip exercises potion scaling, custom effects and detached snapshots.
func TestArrowPotionComponentsRoundTrip(t *testing.T) {
	arrow := NewArrow(1, protocol.UUID{1}, entity.Vec3{}, entity.Vec3{}, -1, protocol.UUID{})
	stack := inventory.NewSlot(item.IDByName("minecraft:tipped_arrow"), 12)
	stack.SetComponent(dc.ComponentPotionContents, inventory.StructuredComponent{Data: map[string]any{
		"potionId": potiondata.PotionPoison, "customColor": int32(0x123456), "customEffects": []any{}, "customName": "venom",
	}})
	stack.SetComponent(dc.ComponentCustomData, inventory.NBTComponent{Value: nbt.NewCompound().PutString("identity", "retained")})
	if err := arrow.SetPickupItem(stack); err != nil {
		t.Fatal(err)
	}
	custom := effect.Instance{ID: effectdata.EffectSlowness, Duration: 600, Amplifier: 2, Flags: effect.FlagAmbient | effect.FlagIcon}
	if err := arrow.AddEffect(custom); err != nil {
		t.Fatal(err)
	}
	if effects := arrow.HitEffects(); len(effects) != 2 || effects[0].Duration != 112 || effects[1] != custom || arrow.PotionColor() != 0x123456 {
		t.Fatalf("unexpected potion effects: %+v color=%x", effects, arrow.PotionColor())
	}
	effects := arrow.HitEffects()
	effects[0].Duration = 1
	pickup := arrow.PickupItem()
	pickup.RemoveComponent(dc.ComponentPotionContents)
	if arrow.HitEffects()[0].Duration != 112 || arrow.PickupItem().ItemCount != 1 {
		t.Fatal("mutable projectile snapshot escaped")
	}
	if original := effect.PotionEffects(potiondata.PotionPoison); original.Effects[0].Duration != 900 {
		t.Fatal("shared potion definition changed")
	}
	raw, err := arrow.PersistentNBT()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := DecodePersistent(9, raw)
	if err != nil {
		t.Fatal(err)
	}
	decoded := restored.(*Arrow)
	if !reflect.DeepEqual(decoded.HitEffects(), arrow.HitEffects()) || decoded.PotionColor() != arrow.PotionColor() || !decoded.PickupItem().StackableWith(arrow.PickupItem()) {
		t.Fatal("saved potion components or effects changed")
	}
	again, err := decoded.PersistentNBT()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(raw, again) {
		t.Fatal("persistent arrow did not round-trip exactly")
	}
	if !arrow.ConsumeMetadataDirty() || arrow.ConsumeMetadataDirty() {
		t.Fatal("color changes were not tracked once")
	}
}

// TestArrowEffectValidationPreservesAcceptedState checks bounded and transactional effect mutations.
func TestArrowEffectValidationPreservesAcceptedState(t *testing.T) {
	arrow := NewArrow(1, protocol.UUID{1}, entity.Vec3{}, entity.Vec3{}, -1, protocol.UUID{})
	valid := effect.Instance{ID: effectdata.EffectSlowness, Duration: 600, Flags: effect.DefaultFlags()}
	if err := arrow.AddEffect(valid); err != nil {
		t.Fatal(err)
	}
	if arrow.PotionColor() != int32(-16777216)|9154528 {
		t.Fatal("visible custom effect color is incorrect")
	}
	for _, invalid := range []effect.Instance{{ID: -1, Duration: 10}, {ID: 999, Duration: 10}, {ID: 1, Duration: 0}, {ID: 1, Duration: -1}, {ID: 1, Duration: 10, Amplifier: 256}, {ID: 1, Duration: 10, Flags: 8}} {
		if arrow.AddEffect(invalid) == nil || !reflect.DeepEqual(arrow.HitEffects(), []effect.Instance{valid}) {
			t.Fatal("invalid effect changed accepted state")
		}
	}
	if arrow.SetPickupItem(inventory.EmptySlot()) == nil {
		t.Fatal("empty pickup item accepted")
	}
	if arrow.SetPickupItem(inventory.NewSlot(item.IDByName("minecraft:diamond"), 1)) == nil {
		t.Fatal("non-arrow pickup item accepted")
	}
	bad := arrow.PickupItem()
	component, _ := bad.Component(dc.ComponentPotionContents)
	fields := component.(inventory.StructuredComponent).Data.(map[string]any)
	fields["potionId"] = int32(999)
	bad.SetComponent(dc.ComponentPotionContents, inventory.StructuredComponent{Data: fields})
	if arrow.SetPickupItem(bad) == nil || arrow.HitEffects()[0] != valid {
		t.Fatal("unknown potion changed accepted state")
	}
	for len(arrow.HitEffects()) < maxArrowCustomEffects {
		if err := arrow.AddEffect(valid); err != nil {
			t.Fatal(err)
		}
	}
	if arrow.AddEffect(valid) == nil || len(arrow.HitEffects()) != maxArrowCustomEffects {
		t.Fatal("custom effect bound was not enforced")
	}
}

// TestArrowSavedEffectsRejectMalformedData rejects corrupt owned components before publication.
func TestArrowSavedEffectsRejectMalformedData(t *testing.T) {
	arrow := NewArrow(1, protocol.UUID{1}, entity.Vec3{}, entity.Vec3{}, -1, protocol.UUID{})
	if err := arrow.AddEffect(effect.Instance{ID: 1, Duration: 600, Flags: effect.DefaultFlags()}); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		change func(*nbt.Compound)
	}{
		{"unknown", func(c *nbt.Compound) { c.PutString("id", "minecraft:missing") }},
		{"negative", func(c *nbt.Compound) { c.PutInt("duration", -2) }},
		{"hidden", func(c *nbt.Compound) {
			c.PutCompound("hidden_effect", nbt.NewCompound().PutByte("amplifier", 0).PutInt("duration", 10))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw, err := arrow.PersistentNBT()
			if err != nil {
				t.Fatal(err)
			}
			item, _ := raw.GetCompound("item")
			components, _ := item.GetCompound("components")
			potion, _ := components.GetCompound("minecraft:potion_contents")
			custom, _ := potion.GetList("custom_effects")
			test.change(custom.Elements()[0].(*nbt.Compound))
			if _, err := DecodePersistent(2, raw); err == nil {
				t.Fatal("malformed saved effect accepted")
			}
		})
	}
}

// TestGroundedArrowPotionDecaySurvivesRestore checks the exact decay threshold and one-shot event.
func TestGroundedArrowPotionDecaySurvivesRestore(t *testing.T) {
	arrow := NewArrow(1, protocol.UUID{1}, entity.Vec3{}, entity.Vec3{}, -1, protocol.UUID{})
	if err := arrow.AddEffect(effect.Instance{ID: 1, Duration: 600, Flags: effect.DefaultFlags()}); err != nil {
		t.Fatal(err)
	}
	arrow.inGround = true
	for range 599 {
		arrow.Tick(nil, nil)
	}
	if len(arrow.HitEffects()) != 1 || arrow.ConsumePotionExpiration() {
		t.Fatal("potion decayed before six hundred grounded ticks")
	}
	raw, err := arrow.PersistentNBT()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodePersistent(2, raw)
	if err != nil {
		t.Fatal(err)
	}
	loaded := decoded.(*Arrow)
	loaded.Tick(nil, nil)
	if len(loaded.HitEffects()) != 0 || loaded.PotionColor() != -1 || loaded.PickupItem().ItemID != item.IDByName("minecraft:arrow") || !loaded.ConsumePotionExpiration() || loaded.ConsumePotionExpiration() {
		t.Fatal("restored arrow did not decay exactly once")
	}
	loaded.Tick(nil, nil)
	if loaded.ConsumePotionExpiration() {
		t.Fatal("decay event repeated")
	}
	raw.PutInt("vitis:in_ground_ticks", -1)
	if _, err := DecodePersistent(2, raw); err == nil {
		t.Fatal("negative grounded time accepted")
	}
}
