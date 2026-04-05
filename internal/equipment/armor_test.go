package equipment

import (
	"math"
	"testing"
)

func TestGetArmorKnown(t *testing.T) {
	p := GetArmor("minecraft:diamond_chestplate")
	if p == nil {
		t.Fatal("expected armor properties for diamond_chestplate")
	}
	if p.Defense != 8 {
		t.Errorf("defense = %f, want 8", p.Defense)
	}
	if p.Toughness != 2 {
		t.Errorf("toughness = %f, want 2", p.Toughness)
	}
	if p.Slot != SlotChestplate {
		t.Errorf("slot = %d, want %d", p.Slot, SlotChestplate)
	}
}

func TestGetArmorUnknown(t *testing.T) {
	if GetArmor("minecraft:stick") != nil {
		t.Error("expected nil for non-armor item")
	}
}

func TestIsArmor(t *testing.T) {
	if !IsArmor("minecraft:iron_helmet") {
		t.Error("iron_helmet should be armor")
	}
	if IsArmor("minecraft:stone") {
		t.Error("stone should not be armor")
	}
}

func TestDamageReductionNoArmor(t *testing.T) {
	result := CalculateDamageReduction(10.0, 0, 0)
	if result != 10.0 {
		t.Errorf("damage = %f, want 10.0", result)
	}
}

func TestDamageReductionFullDiamond(t *testing.T) {
	result := CalculateDamageReduction(10.0, 20.0, 8.0)
	if result >= 10.0 || result <= 0 {
		t.Errorf("damage = %f, expected reduced but positive", result)
	}
}

func TestDamageReductionIronArmor(t *testing.T) {
	result := CalculateDamageReduction(5.0, 15.0, 0)
	effective := 15.0 - 4.0*5.0/(0+8.0)
	expected := 5.0 * (1.0 - effective/25.0)
	if math.Abs(result-expected) > 0.01 {
		t.Errorf("damage = %f, want %f", result, expected)
	}
}
