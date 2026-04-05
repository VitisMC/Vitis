package entity

import (
	"math"
	"testing"

	genattr "github.com/vitismc/vitis/internal/data/generated/attribute"
)

func TestDefaultPlayerAttributes(t *testing.T) {
	ac := DefaultPlayerAttributes()
	if ac.GetBase(genattr.AttrMaxHealth) != 20.0 {
		t.Errorf("max_health base = %f, want 20.0", ac.GetBase(genattr.AttrMaxHealth))
	}
	if ac.GetBase(genattr.AttrMovementSpeed) != 0.1 {
		t.Errorf("movement_speed base = %f, want 0.1", ac.GetBase(genattr.AttrMovementSpeed))
	}
	if ac.GetBase(genattr.AttrAttackDamage) != 1.0 {
		t.Errorf("attack_damage base = %f, want 1.0", ac.GetBase(genattr.AttrAttackDamage))
	}
}

func TestAttributeValueNoModifiers(t *testing.T) {
	ac := NewAttributeContainer()
	ac.SetBase(genattr.AttrMaxHealth, 20.0)
	if ac.GetValue(genattr.AttrMaxHealth) != 20.0 {
		t.Errorf("value = %f, want 20.0", ac.GetValue(genattr.AttrMaxHealth))
	}
}

func TestAttributeModifierAdd(t *testing.T) {
	ac := NewAttributeContainer()
	ac.SetBase(genattr.AttrMaxHealth, 20.0)
	ac.AddModifier(genattr.AttrMaxHealth, Modifier{ID: "test:add", Amount: 4.0, Operation: 0})
	v := ac.GetValue(genattr.AttrMaxHealth)
	if math.Abs(v-24.0) > 0.001 {
		t.Errorf("value = %f, want 24.0", v)
	}
}

func TestAttributeModifierMultiplyBase(t *testing.T) {
	ac := NewAttributeContainer()
	ac.SetBase(genattr.AttrMaxHealth, 20.0)
	ac.AddModifier(genattr.AttrMaxHealth, Modifier{ID: "test:mul_base", Amount: 0.5, Operation: 1})
	v := ac.GetValue(genattr.AttrMaxHealth)
	if math.Abs(v-30.0) > 0.001 {
		t.Errorf("value = %f, want 30.0", v)
	}
}

func TestAttributeModifierMultiplyTotal(t *testing.T) {
	ac := NewAttributeContainer()
	ac.SetBase(genattr.AttrMaxHealth, 20.0)
	ac.AddModifier(genattr.AttrMaxHealth, Modifier{ID: "test:mul_total", Amount: 0.5, Operation: 2})
	v := ac.GetValue(genattr.AttrMaxHealth)
	if math.Abs(v-30.0) > 0.001 {
		t.Errorf("value = %f, want 30.0", v)
	}
}

func TestAttributeRemoveModifier(t *testing.T) {
	ac := NewAttributeContainer()
	ac.SetBase(genattr.AttrArmor, 0.0)
	ac.AddModifier(genattr.AttrArmor, Modifier{ID: "armor:chest", Amount: 8.0, Operation: 0})
	ac.RemoveModifier(genattr.AttrArmor, "armor:chest")
	v := ac.GetValue(genattr.AttrArmor)
	if v != 0.0 {
		t.Errorf("value = %f, want 0.0 after removal", v)
	}
}

func TestAttributeDirtyProperties(t *testing.T) {
	ac := NewAttributeContainer()
	ac.SetBase(genattr.AttrMaxHealth, 20.0)
	ac.SetBase(genattr.AttrArmor, 5.0)

	dirty := ac.DirtyProperties()
	if len(dirty) != 2 {
		t.Fatalf("dirty count = %d, want 2", len(dirty))
	}

	dirty2 := ac.DirtyProperties()
	if len(dirty2) != 0 {
		t.Errorf("dirty count after clear = %d, want 0", len(dirty2))
	}
}

func TestAttributeToProperties(t *testing.T) {
	ac := DefaultPlayerAttributes()
	props := ac.ToProperties()
	if len(props) == 0 {
		t.Error("expected non-empty properties from default player attributes")
	}
}
