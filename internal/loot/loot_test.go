package loot

import (
	"math/rand"
	"testing"
)

func TestManagerLoad(t *testing.T) {
	m := NewManager("../../.mcdata/1.21.4")
	if err := m.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	count := m.BlockTableCount()
	if count == 0 {
		t.Fatal("No block tables loaded")
	}
	t.Logf("Loaded %d block loot tables", count)
}

func TestDiamondOreDrops(t *testing.T) {
	m := NewManager("../../.mcdata/1.21.4")
	if err := m.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	table := m.GetBlockTable("minecraft:diamond_ore")
	if table == nil {
		t.Fatal("diamond_ore table not found")
	}

	rng := rand.New(rand.NewSource(42))

	ctx := NewContext(rng)
	drops := table.GetDrops(ctx)
	if len(drops) == 0 {
		t.Fatal("Expected drops from diamond ore")
	}
	if drops[0].ItemID != "minecraft:diamond" {
		t.Errorf("Expected diamond drop, got %s", drops[0].ItemID)
	}

	ctx = NewContext(rng).WithTool("minecraft:diamond_pickaxe", map[string]int{
		"minecraft:silk_touch": 1,
	})
	drops = table.GetDrops(ctx)
	if len(drops) == 0 {
		t.Fatal("Expected drops with silk touch")
	}
	if drops[0].ItemID != "minecraft:diamond_ore" {
		t.Errorf("Expected diamond_ore with silk touch, got %s", drops[0].ItemID)
	}
}

func TestGrassBlockDrops(t *testing.T) {
	m := NewManager("../../.mcdata/1.21.4")
	if err := m.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	table := m.GetBlockTable("minecraft:grass_block")
	if table == nil {
		t.Fatal("grass_block table not found")
	}

	rng := rand.New(rand.NewSource(42))

	ctx := NewContext(rng)
	drops := table.GetDrops(ctx)
	if len(drops) == 0 {
		t.Fatal("Expected drops from grass block")
	}
	if drops[0].ItemID != "minecraft:dirt" {
		t.Errorf("Expected dirt drop, got %s", drops[0].ItemID)
	}

	ctx = NewContext(rng).WithTool("minecraft:diamond_shovel", map[string]int{
		"minecraft:silk_touch": 1,
	})
	drops = table.GetDrops(ctx)
	if len(drops) == 0 {
		t.Fatal("Expected drops with silk touch")
	}
	if drops[0].ItemID != "minecraft:grass_block" {
		t.Errorf("Expected grass_block with silk touch, got %s", drops[0].ItemID)
	}
}

func TestFortuneOreDrops(t *testing.T) {
	m := NewManager("../../.mcdata/1.21.4")
	if err := m.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	table := m.GetBlockTable("minecraft:diamond_ore")
	if table == nil {
		t.Fatal("diamond_ore table not found")
	}

	totalNoFortune := 0
	totalFortune3 := 0
	iterations := 1000

	for i := 0; i < iterations; i++ {
		rng := rand.New(rand.NewSource(int64(i)))
		ctx := NewContext(rng)
		drops := table.GetDrops(ctx)
		for _, d := range drops {
			totalNoFortune += d.Count
		}
	}

	for i := 0; i < iterations; i++ {
		rng := rand.New(rand.NewSource(int64(i)))
		ctx := NewContext(rng).WithTool("minecraft:diamond_pickaxe", map[string]int{
			"minecraft:fortune": 3,
		})
		drops := table.GetDrops(ctx)
		for _, d := range drops {
			totalFortune3 += d.Count
		}
	}

	avgNoFortune := float64(totalNoFortune) / float64(iterations)
	avgFortune3 := float64(totalFortune3) / float64(iterations)

	t.Logf("Average drops without fortune: %.2f", avgNoFortune)
	t.Logf("Average drops with fortune 3: %.2f", avgFortune3)

	if avgFortune3 <= avgNoFortune {
		t.Error("Fortune 3 should increase average drops")
	}
}
