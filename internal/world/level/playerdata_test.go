package level

import (
	"path/filepath"
	"testing"

	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

func TestPlayerDataSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	worldDir := filepath.Join(dir, "testworld")

	pd := DefaultPlayerData("550e8400-e29b-41d4-a716-446655440000", 100.5, 65.0, -200.5, 0)
	pd.Yaw = 90.0
	pd.Pitch = -15.0
	pd.Health = 18.5
	pd.FoodLevel = 15
	pd.FoodExhaustion = 2.5
	pd.XPLevel = 8
	pd.XPTotal = 100
	pd.XPBar = 0.25
	pd.SpawnX, pd.SpawnY, pd.SpawnZ = 10, 65, -20
	pd.HasSpawnPoint = true
	pd.Effects = []effect.Instance{{ID: effectdata.EffectRegeneration, Amplifier: 1, Duration: 200, Flags: effect.FlagParticles | effect.FlagIcon}}
	pd.GameMode = 0

	if err := pd.Save(worldDir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadPlayerData(worldDir, "550e8400-e29b-41d4-a716-446655440000")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.GameMode != 0 {
		t.Fatalf("GameMode = %d, want 0", loaded.GameMode)
	}
	if loaded.Health != 18.5 {
		t.Fatalf("Health = %f, want 18.5", loaded.Health)
	}
	if loaded.FoodLevel != 15 {
		t.Fatalf("FoodLevel = %d, want 15", loaded.FoodLevel)
	}
	if loaded.FoodExhaustion != 2.5 || loaded.XPLevel != 8 || loaded.XPTotal != 100 || loaded.XPBar != 0.25 {
		t.Fatalf("survival state was not restored: %+v", loaded)
	}
	if !loaded.HasSpawnPoint || loaded.SpawnX != 10 || loaded.SpawnY != 65 || loaded.SpawnZ != -20 {
		t.Fatalf("bed spawn was not restored: %+v", loaded)
	}
	if len(loaded.Effects) != 1 || loaded.Effects[0].ID != effectdata.EffectRegeneration || loaded.Effects[0].Duration != 200 {
		t.Fatalf("status effects were not restored: %+v", loaded.Effects)
	}
	if loaded.Dimension != "minecraft:overworld" {
		t.Fatalf("Dimension = %q, want minecraft:overworld", loaded.Dimension)
	}
	if loaded.DataVersion != protocol.CurrentDataVersion {
		t.Fatalf("DataVersion = %d, want %d", loaded.DataVersion, protocol.CurrentDataVersion)
	}
}

func TestPlayerDataLoadNonExistent(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadPlayerData(dir, "nonexistent-uuid")
	if err == nil {
		t.Fatal("expected error for non-existent player data")
	}
}

func TestPlayerDataOverwrite(t *testing.T) {
	dir := t.TempDir()
	worldDir := filepath.Join(dir, "testworld")
	uuid := "test-uuid-1234"

	pd1 := DefaultPlayerData(uuid, 0, 65, 0, 0)
	pd1.Health = 10.0
	if err := pd1.Save(worldDir); err != nil {
		t.Fatalf("Save 1: %v", err)
	}

	pd2 := DefaultPlayerData(uuid, 50, 70, 50, 0)
	pd2.Health = 5.0
	if err := pd2.Save(worldDir); err != nil {
		t.Fatalf("Save 2: %v", err)
	}

	loaded, err := LoadPlayerData(worldDir, uuid)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Health != 5.0 {
		t.Fatalf("Health = %f, want 5.0", loaded.Health)
	}
}

func TestPlayerDataSaveRejectsUnsupportedDataVersion(t *testing.T) {
	pd := DefaultPlayerData("uuid", 0, 64, 0, 0)
	pd.DataVersion = protocol.CurrentDataVersion - 1
	if err := pd.Save(t.TempDir()); err == nil {
		t.Fatal("Save succeeded for unsupported DataVersion")
	}
}

func TestPlayerDataSavePreservesUnknownNBTAndItemComponents(t *testing.T) {
	worldDir := filepath.Join(t.TempDir(), "world")
	pd := DefaultPlayerData("lossless-player", 0, 64, 0, 0)
	pd.raw = nbt.NewCompound().PutString("UnknownPlayerTag", "preserve")
	pd.Inventory = []InventorySlot{{
		Slot:     0,
		ItemName: "minecraft:stone",
		Count:    1,
		raw: nbt.NewCompound().PutCompound("components", nbt.NewCompound().
			PutString("minecraft:custom_name", `"preserve"`)),
	}}

	if err := pd.Save(worldDir); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := LoadPlayerData(worldDir, pd.UUID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if value, _ := loaded.raw.GetString("UnknownPlayerTag"); value != "preserve" {
		t.Fatalf("UnknownPlayerTag = %q; want preserve", value)
	}
	if len(loaded.Inventory) != 1 {
		t.Fatalf("inventory size = %d; want 1", len(loaded.Inventory))
	}
	components, ok := loaded.Inventory[0].raw.GetCompound("components")
	if !ok {
		t.Fatal("components compound was lost")
	}
	if value, _ := components.GetString("minecraft:custom_name"); value != `"preserve"` {
		t.Fatalf("custom component = %q; want preserve", value)
	}
}

// TestPlayerEffectsPreserveUnknownEntries retains unsupported effect NBT.
func TestPlayerEffectsPreserveUnknownEntries(t *testing.T) {
	worldDir := filepath.Join(t.TempDir(), "world")
	pd := DefaultPlayerData("effects-player", 0, 64, 0, 0)
	active := nbt.NewList(nbt.TagCompound)
	active.Add(nbt.NewCompound().PutString("id", "mod:unknown_effect").PutInt("duration", 40).PutString("custom", "keep"))
	pd.raw = nbt.NewCompound().PutList("active_effects", active)
	pd.Effects = []effect.Instance{{ID: effectdata.EffectRegeneration, Duration: 60, Flags: effect.DefaultFlags()}}
	if err := pd.Save(worldDir); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadPlayerData(worldDir, pd.UUID)
	if err != nil {
		t.Fatal(err)
	}
	list, ok := loaded.raw.GetList("active_effects")
	if !ok || list.Len() != 2 {
		t.Fatalf("active effects = %v", list)
	}
	unknown, ok := list.Elements()[0].(*nbt.Compound)
	if !ok {
		t.Fatal("unknown effect entry was lost")
	}
	if value, _ := unknown.GetString("custom"); value != "keep" {
		t.Fatal("unknown effect data was lost")
	}
}

// TestPlayerInventoryRejectsMalformedSlotData prevents replacing invalid saved inventories.
func TestPlayerInventoryRejectsMalformedSlotData(t *testing.T) {
	for _, entries := range []*nbt.List{
		nbt.NewList(nbt.TagCompound).Add(nbt.NewCompound().PutString("id", "minecraft:stone")),
		nbt.NewList(nbt.TagCompound).Add(nbt.NewCompound().PutByte("Slot", 0).PutString("id", "minecraft:stone")).Add(nbt.NewCompound().PutByte("Slot", 36).PutString("id", "minecraft:dirt")),
		nbt.NewList(nbt.TagCompound).Add(nbt.NewCompound().PutByte("Slot", 99).PutString("id", "minecraft:stone")),
	} {
		data := &PlayerData{}
		data.parseCompound(nbt.NewCompound().PutList("Inventory", entries))
		if data.parseError == nil {
			t.Fatal("invalid inventory accepted")
		}
	}
}
