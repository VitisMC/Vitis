package enchantment

import (
	"testing"

	enchdata "github.com/vitismc/vitis/internal/data/generated/enchantment"
)

func TestNewList(t *testing.T) {
	l := NewList()
	if l.Count() != 0 {
		t.Errorf("expected 0 enchantments, got %d", l.Count())
	}
}

func TestAddAndHas(t *testing.T) {
	l := NewList()
	id := enchdata.EnchantmentIDByName("minecraft:sharpness")
	ok := l.Add(id, 3)
	if !ok {
		t.Error("expected Add to return true")
	}
	if !l.Has(id) {
		t.Error("expected Has(sharpness) to be true")
	}
	if l.Level(id) != 3 {
		t.Errorf("expected level 3, got %d", l.Level(id))
	}
	if l.Count() != 1 {
		t.Errorf("expected 1 enchantment, got %d", l.Count())
	}
}

func TestAddExceedsMaxLevel(t *testing.T) {
	l := NewList()
	id := enchdata.EnchantmentIDByName("minecraft:sharpness")
	ok := l.Add(id, 10)
	if ok {
		t.Error("expected Add to return false for level > max")
	}
}

func TestAddExclusive(t *testing.T) {
	l := NewList()
	sharp := enchdata.EnchantmentIDByName("minecraft:sharpness")
	smite := enchdata.EnchantmentIDByName("minecraft:smite")

	l.Add(sharp, 3)
	ok := l.Add(smite, 3)
	if ok {
		t.Error("expected Add to return false for exclusive enchantment")
	}
	if l.Has(smite) {
		t.Error("expected smite to not be added")
	}
}

func TestAddUpgrade(t *testing.T) {
	l := NewList()
	id := enchdata.EnchantmentIDByName("minecraft:sharpness")
	l.Add(id, 3)
	ok := l.Add(id, 5)
	if !ok {
		t.Error("expected upgrade to succeed")
	}
	if l.Level(id) != 5 {
		t.Errorf("expected level 5, got %d", l.Level(id))
	}
	if l.Count() != 1 {
		t.Errorf("expected still 1 enchantment, got %d", l.Count())
	}
}

func TestSet(t *testing.T) {
	l := NewList()
	id := enchdata.EnchantmentIDByName("minecraft:sharpness")
	l.Set(id, 5)
	if l.Level(id) != 5 {
		t.Errorf("expected level 5, got %d", l.Level(id))
	}
	l.Set(id, 0)
	if l.Has(id) {
		t.Error("expected enchantment removed when set to 0")
	}
}

func TestRemove(t *testing.T) {
	l := NewList()
	id := enchdata.EnchantmentIDByName("minecraft:sharpness")
	l.Add(id, 3)
	ok := l.Remove(id)
	if !ok {
		t.Error("expected Remove to return true")
	}
	if l.Has(id) {
		t.Error("expected enchantment to be removed")
	}
	ok = l.Remove(id)
	if ok {
		t.Error("expected Remove to return false for non-existent")
	}
}

func TestClear(t *testing.T) {
	l := NewList()
	l.Add(enchdata.EnchantmentIDByName("minecraft:sharpness"), 3)
	l.Add(enchdata.EnchantmentIDByName("minecraft:unbreaking"), 2)
	l.Clear()
	if l.Count() != 0 {
		t.Errorf("expected 0 after clear, got %d", l.Count())
	}
}

func TestFromEntries(t *testing.T) {
	entries := []Entry{
		{ID: enchdata.EnchantmentIDByName("minecraft:sharpness"), Level: 5},
		{ID: enchdata.EnchantmentIDByName("minecraft:unbreaking"), Level: 3},
	}
	l := FromEntries(entries)
	if l.Count() != 2 {
		t.Errorf("expected 2 enchantments, got %d", l.Count())
	}
	if l.Level(enchdata.EnchantmentIDByName("minecraft:sharpness")) != 5 {
		t.Error("expected sharpness level 5")
	}
}

func TestAreExclusive(t *testing.T) {
	sharp := enchdata.EnchantmentIDByName("minecraft:sharpness")
	smite := enchdata.EnchantmentIDByName("minecraft:smite")
	unbr := enchdata.EnchantmentIDByName("minecraft:unbreaking")

	if !AreExclusive(sharp, smite) {
		t.Error("expected sharpness and smite to be exclusive")
	}
	if AreExclusive(sharp, unbr) {
		t.Error("expected sharpness and unbreaking to not be exclusive")
	}
}

func TestProtectionExclusive(t *testing.T) {
	prot := enchdata.EnchantmentIDByName("minecraft:protection")
	fire := enchdata.EnchantmentIDByName("minecraft:fire_protection")
	if !AreExclusive(prot, fire) {
		t.Error("expected protection and fire_protection to be exclusive")
	}
}

func TestFortuneAndSilkTouchExclusive(t *testing.T) {
	fortune := enchdata.EnchantmentIDByName("minecraft:fortune")
	silk := enchdata.EnchantmentIDByName("minecraft:silk_touch")
	if !AreExclusive(fortune, silk) {
		t.Error("expected fortune and silk_touch to be exclusive")
	}
}

func TestSharpnessDamage(t *testing.T) {
	l := NewList()
	id := enchdata.EnchantmentIDByName("minecraft:sharpness")
	l.Add(id, 5)
	dmg := SharpnessDamage(l)
	expected := 0.5*5.0 + 0.5
	if dmg != expected {
		t.Errorf("expected sharpness damage %f, got %f", expected, dmg)
	}
}

func TestSmiteDamage(t *testing.T) {
	l := NewList()
	id := enchdata.EnchantmentIDByName("minecraft:smite")
	l.Add(id, 3)
	dmg := SmiteDamage(l)
	if dmg != 7.5 {
		t.Errorf("expected 7.5 smite damage, got %f", dmg)
	}
}

func TestKnockbackLevel(t *testing.T) {
	l := NewList()
	id := enchdata.EnchantmentIDByName("minecraft:knockback")
	l.Add(id, 2)
	if KnockbackLevel(l) != 2 {
		t.Errorf("expected knockback level 2, got %d", KnockbackLevel(l))
	}
}

func TestHasSilkTouch(t *testing.T) {
	l := NewList()
	if HasSilkTouch(l) {
		t.Error("expected no silk touch on empty list")
	}
	l.Add(enchdata.EnchantmentIDByName("minecraft:silk_touch"), 1)
	if !HasSilkTouch(l) {
		t.Error("expected silk touch to be present")
	}
}

func TestHasMending(t *testing.T) {
	l := NewList()
	l.Add(enchdata.EnchantmentIDByName("minecraft:mending"), 1)
	if !HasMending(l) {
		t.Error("expected mending to be present")
	}
}

func TestNilList(t *testing.T) {
	if SharpnessDamage(nil) != 0 {
		t.Error("expected 0 damage for nil list")
	}
	if ProtectionFactor(nil) != 0 {
		t.Error("expected 0 protection for nil list")
	}
	if KnockbackLevel(nil) != 0 {
		t.Error("expected 0 knockback for nil list")
	}
	if HasSilkTouch(nil) {
		t.Error("expected false for nil list")
	}
}

func TestByName(t *testing.T) {
	id := ByName("minecraft:sharpness")
	if id < 0 {
		t.Error("expected valid ID for sharpness")
	}
	id = ByName("minecraft:nonexistent")
	if id != -1 {
		t.Error("expected -1 for nonexistent enchantment")
	}
}

func TestInfo(t *testing.T) {
	info := Info(0)
	if info == nil {
		t.Fatal("expected non-nil info for ID 0")
	}
	if info.Name != "minecraft:aqua_affinity" {
		t.Errorf("expected aqua_affinity, got %s", info.Name)
	}
}
