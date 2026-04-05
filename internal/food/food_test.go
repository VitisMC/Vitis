package food

import "testing"

func TestGetKnownFood(t *testing.T) {
	p := Get("minecraft:cooked_beef")
	if p == nil {
		t.Fatal("cooked_beef should be food")
	}
	if p.Nutrition != 8 {
		t.Errorf("cooked_beef nutrition = %d, want 8", p.Nutrition)
	}
	if p.EatDuration != 32 {
		t.Errorf("cooked_beef eat duration = %d, want 32", p.EatDuration)
	}
}

func TestGetUnknownItem(t *testing.T) {
	if Get("minecraft:stone") != nil {
		t.Error("stone should not be food")
	}
}

func TestIsFood(t *testing.T) {
	if !IsFood("minecraft:apple") {
		t.Error("apple should be food")
	}
	if IsFood("minecraft:diamond") {
		t.Error("diamond should not be food")
	}
}

func TestCanAlwaysEat(t *testing.T) {
	p := Get("minecraft:golden_apple")
	if p == nil {
		t.Fatal("golden_apple should be food")
	}
	if !p.CanAlwaysEat {
		t.Error("golden_apple should have CanAlwaysEat")
	}

	p = Get("minecraft:bread")
	if p == nil {
		t.Fatal("bread should be food")
	}
	if p.CanAlwaysEat {
		t.Error("bread should not have CanAlwaysEat")
	}
}

func TestDriedKelpFastEat(t *testing.T) {
	p := Get("minecraft:dried_kelp")
	if p == nil {
		t.Fatal("dried_kelp should be food")
	}
	if p.EatDuration != 16 {
		t.Errorf("dried_kelp eat duration = %d, want 16", p.EatDuration)
	}
}

func TestEatRestoresFood(t *testing.T) {
	newFood, newSat := Eat(10, 2.0, 8, 0.8)
	if newFood != 18 {
		t.Errorf("food = %d, want 18", newFood)
	}
	expectedSat := float32(2.0 + 8*0.8*2.0)
	if newSat < expectedSat-0.01 || newSat > expectedSat+0.01 {
		t.Errorf("saturation = %f, want ~%f", newSat, expectedSat)
	}
}

func TestEatClampsFood(t *testing.T) {
	newFood, _ := Eat(18, 5.0, 8, 0.8)
	if newFood != 20 {
		t.Errorf("food = %d, want 20 (clamped)", newFood)
	}
}

func TestEatClampsSaturation(t *testing.T) {
	newFood, newSat := Eat(19, 19.0, 1, 1.2)
	if newFood != 20 {
		t.Errorf("food = %d, want 20", newFood)
	}
	if newSat > float32(newFood) {
		t.Errorf("saturation %f should not exceed food level %d", newSat, newFood)
	}
}
