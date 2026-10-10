package entity

import (
	"math"
	"testing"

	genattr "github.com/vitismc/vitis/internal/data/generated/attribute"
	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestMovementEffectModifiersReplaceExpireAndRestore checks modifier identity across the lifecycle.
func TestMovementEffectModifiersReplaceExpireAndRestore(t *testing.T) {
	mob := NewMobEntityByName(1, protocol.UUID{1}, "cow", Vec3{})
	base := mob.Attributes().GetBase(genattr.AttrMovementSpeed)
	mob.Attributes().AddModifier(genattr.AttrMovementSpeed, Modifier{ID: "example:equipment", Amount: .1, Operation: 0})
	mob.Effects().Add(effect.Instance{ID: effectdata.EffectSlowness, Amplifier: 1, Duration: 3, Flags: effect.DefaultFlags()})
	for range 5 {
		mob.SyncEffectMovement()
	}
	want := (base + .1) * (1 + float64(float32(-.15))*2)
	if actual := mob.Attributes().GetValue(genattr.AttrMovementSpeed); math.Abs(actual-want) > 1e-9 {
		t.Fatalf("slowness accumulated: got=%v want=%v", actual, want)
	}
	mob.Effects().Add(effect.Instance{ID: effectdata.EffectSpeed, Duration: 20, Flags: effect.DefaultFlags()})
	mob.SyncEffectMovement()
	raw, err := mob.PersistentNBT()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := DecodePersistentEntity(2, raw)
	if err != nil {
		t.Fatal(err)
	}
	loaded := restored.(*MobEntity)
	loaded.SyncEffectMovement()
	want *= 1 + float64(float32(.2))
	if actual := loaded.Attributes().GetValue(genattr.AttrMovementSpeed); math.Abs(actual-want) > 1e-9 {
		t.Fatal("restore duplicated movement modifiers")
	}
	for range 3 {
		loaded.TickEffects()
	}
	want = (base + .1) * (1 + float64(float32(.2)))
	if actual := loaded.Attributes().GetValue(genattr.AttrMovementSpeed); math.Abs(actual-want) > 1e-9 {
		t.Fatal("expired slowness kept its movement modifier")
	}
	loaded.Effects().Clear()
	loaded.SyncEffectMovement()
	if actual := loaded.Attributes().GetValue(genattr.AttrMovementSpeed); math.Abs(actual-(base+.1)) > 1e-9 {
		t.Fatal("clearing effects changed equipment modifier")
	}
}

// TestRegenerationMarksHealthForSynchronization checks the living-state mutation used by mob updates.
func TestRegenerationMarksHealthForSynchronization(t *testing.T) {
	mob := NewMobEntityByName(1, protocol.UUID{1}, "cow", Vec3{})
	mob.SetHealth(5)
	mob.ClearDirty()
	mob.Effects().Add(effect.Instance{ID: effectdata.EffectRegeneration, Duration: 50})
	mob.TickEffects()
	if mob.Health() != 6 || mob.Dirty()&DirtyLivingState == 0 {
		t.Fatal("effect healing was not marked for client synchronization")
	}
}
