package registry

import (
	"testing"
)

func TestIDRegistryBasic(t *testing.T) {
	names := []string{"minecraft:a", "minecraft:b", "minecraft:c"}
	reg := NewIDRegistry("minecraft:test", names)

	if reg.Name() != "minecraft:test" {
		t.Fatalf("expected name minecraft:test, got %s", reg.Name())
	}
	if reg.Size() != 3 {
		t.Fatalf("expected size 3, got %d", reg.Size())
	}

	if id := reg.IDByName("minecraft:a"); id != 0 {
		t.Fatalf("expected ID 0 for minecraft:a, got %d", id)
	}
	if id := reg.IDByName("minecraft:c"); id != 2 {
		t.Fatalf("expected ID 2 for minecraft:c, got %d", id)
	}
	if id := reg.IDByName("minecraft:missing"); id != -1 {
		t.Fatalf("expected -1 for missing, got %d", id)
	}

	if name := reg.NameByID(1); name != "minecraft:b" {
		t.Fatalf("expected minecraft:b for ID 1, got %s", name)
	}
	if name := reg.NameByID(-1); name != "" {
		t.Fatalf("expected empty for ID -1, got %s", name)
	}
	if name := reg.NameByID(99); name != "" {
		t.Fatalf("expected empty for ID 99, got %s", name)
	}
}

func TestIDRegistryContains(t *testing.T) {
	reg := NewIDRegistry("test", []string{"minecraft:stone", "minecraft:dirt"})
	if !reg.Contains("minecraft:stone") {
		t.Fatal("expected Contains(minecraft:stone) = true")
	}
	if reg.Contains("minecraft:air") {
		t.Fatal("expected Contains(minecraft:air) = false")
	}
}

func TestIDRegistryImmutability(t *testing.T) {
	names := []string{"minecraft:a", "minecraft:b"}
	reg := NewIDRegistry("test", names)
	names[0] = "minecraft:MUTATED"
	if reg.NameByID(0) != "minecraft:a" {
		t.Fatal("registry was mutated by external slice change")
	}
	out := reg.Names()
	out[0] = "minecraft:MUTATED"
	if reg.NameByID(0) != "minecraft:a" {
		t.Fatal("registry was mutated via Names() return")
	}
}

func TestIDRegistryValidate(t *testing.T) {
	reg := NewIDRegistry("test", []string{"minecraft:a", "minecraft:b"})
	if err := reg.Validate(); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
	empty := NewIDRegistry("empty", nil)
	if err := empty.Validate(); err == nil {
		t.Fatal("expected validation error for empty registry")
	}
}

func TestManagerCreation(t *testing.T) {
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}
	if mgr.RegistryCount() == 0 {
		t.Fatal("expected non-zero registry count")
	}
	if mgr.ConfigRegistryCount() != 12 {
		t.Fatalf("expected 12 config registries, got %d", mgr.ConfigRegistryCount())
	}
}

func TestManagerBuiltinRegistries(t *testing.T) {
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	tests := []struct {
		registry string
		minSize  int
	}{
		{"minecraft:block", 1000},
		{"minecraft:item", 1300},
		{"minecraft:entity_type", 100},
		{"minecraft:fluid", 3},
		{"minecraft:sound_event", 1500},
		{"minecraft:mob_effect", 30},
		{"minecraft:particle_type", 100},
		{"minecraft:potion", 40},
	}

	for _, tt := range tests {
		reg := mgr.Registry(tt.registry)
		if reg == nil {
			t.Errorf("registry %q not found", tt.registry)
			continue
		}
		if reg.Size() < tt.minSize {
			t.Errorf("registry %q: expected at least %d entries, got %d", tt.registry, tt.minSize, reg.Size())
		}
	}
}

func TestManagerConfigRegistries(t *testing.T) {
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	expectedConfigs := []struct {
		name     string
		minCount int
	}{
		{"minecraft:banner_pattern", 40},
		{"minecraft:chat_type", 5},
		{"minecraft:damage_type", 40},
		{"minecraft:dimension_type", 3},
		{"minecraft:enchantment", 40},
		{"minecraft:instrument", 7},
		{"minecraft:jukebox_song", 15},
		{"minecraft:painting_variant", 40},
		{"minecraft:trim_material", 10},
		{"minecraft:trim_pattern", 15},
		{"minecraft:wolf_variant", 8},
		{"minecraft:worldgen/biome", 60},
	}

	for _, tt := range expectedConfigs {
		entries := mgr.ConfigEntries(tt.name)
		if len(entries) < tt.minCount {
			t.Errorf("config registry %q: expected at least %d entries, got %d", tt.name, tt.minCount, len(entries))
		}
		for i, e := range entries {
			if e.Name == "" {
				t.Errorf("config registry %q entry %d: empty name", tt.name, i)
			}
			if len(e.Data) == 0 {
				t.Errorf("config registry %q entry %d (%s): empty NBT data", tt.name, i, e.Name)
			}
		}
	}
}

func TestManagerSpecificLookups(t *testing.T) {
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	if id := mgr.IDByName("minecraft:block", "minecraft:stone"); id < 0 {
		t.Error("minecraft:stone not found in block registry")
	}
	if id := mgr.IDByName("minecraft:block", "minecraft:air"); id != 0 {
		t.Errorf("expected minecraft:air ID=0, got %d", id)
	}
	if id := mgr.IDByName("minecraft:item", "minecraft:diamond"); id < 0 {
		t.Error("minecraft:diamond not found in item registry")
	}
	if id := mgr.IDByName("minecraft:entity_type", "minecraft:player"); id < 0 {
		t.Error("minecraft:player not found in entity_type registry")
	}
	if id := mgr.IDByName("minecraft:worldgen/biome", "minecraft:plains"); id < 0 {
		t.Error("minecraft:plains not found in worldgen/biome registry")
	}
	if id := mgr.IDByName("minecraft:dimension_type", "minecraft:overworld"); id < 0 {
		t.Error("minecraft:overworld not found in dimension_type registry")
	}
	if id := mgr.IDByName("nonexistent:reg", "foo"); id != -1 {
		t.Errorf("expected -1 for nonexistent registry, got %d", id)
	}
}

func TestManagerBidirectionalLookup(t *testing.T) {
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	reg := mgr.Registry("minecraft:block")
	if reg == nil {
		t.Fatal("block registry not found")
	}

	for i := int32(0); i < int32(reg.Size()); i++ {
		name := reg.NameByID(i)
		if name == "" {
			t.Fatalf("block ID %d has no name", i)
		}
		reverseID := reg.IDByName(name)
		if reverseID != i {
			t.Fatalf("bidirectional mismatch: ID %d → %q → %d", i, name, reverseID)
		}
	}
}

func TestManagerNoDuplicates(t *testing.T) {
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	reg := mgr.Registry("minecraft:item")
	if reg == nil {
		t.Fatal("item registry not found")
	}

	seen := make(map[string]bool, reg.Size())
	for i := int32(0); i < int32(reg.Size()); i++ {
		name := reg.NameByID(i)
		if seen[name] {
			t.Fatalf("duplicate entry %q at ID %d", name, i)
		}
		seen[name] = true
	}
}

func TestBuildRegistryDataPackets(t *testing.T) {
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	packets := mgr.BuildRegistryDataPackets(false)
	if len(packets) != 12 {
		t.Fatalf("expected 12 RegistryData packets, got %d", len(packets))
	}

	for _, pkt := range packets {
		if pkt.RegistryID == "" {
			t.Error("packet has empty RegistryID")
		}
		if len(pkt.Entries) == 0 {
			t.Errorf("packet %q has no entries", pkt.RegistryID)
		}
		for i, entry := range pkt.Entries {
			if !entry.HasData {
				t.Errorf("packet %q entry %d: HasData must be true", pkt.RegistryID, i)
			}
			if len(entry.Data) == 0 {
				t.Errorf("packet %q entry %d (%s): empty Data", pkt.RegistryID, i, entry.EntryID)
			}
			if entry.Data[0] != 0x0a {
				t.Errorf("packet %q entry %d (%s): NBT must start with compound tag (0x0a), got 0x%02x",
					pkt.RegistryID, i, entry.EntryID, entry.Data[0])
			}
		}
	}
}

func TestBuildRegistryDataPacketsKnownVanilla(t *testing.T) {
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	packets := mgr.BuildRegistryDataPackets(true)
	if len(packets) != 12 {
		t.Fatalf("expected 12 RegistryData packets, got %d", len(packets))
	}

	for _, pkt := range packets {
		if pkt.RegistryID == "" {
			t.Error("packet has empty RegistryID")
		}
		if len(pkt.Entries) == 0 {
			t.Errorf("packet %q has no entries", pkt.RegistryID)
		}
		for i, entry := range pkt.Entries {
			if entry.HasData {
				t.Errorf("packet %q entry %d: HasData must be false in known-packs mode", pkt.RegistryID, i)
			}
			if len(entry.Data) != 0 {
				t.Errorf("packet %q entry %d (%s): Data must be empty in known-packs mode", pkt.RegistryID, i, entry.EntryID)
			}
		}
	}
}

func TestBuildUpdateTags(t *testing.T) {
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	tags := mgr.BuildUpdateTags()
	if tags == nil {
		t.Fatal("BuildUpdateTags returned nil")
	}
	if len(tags.Registries) == 0 {
		t.Fatal("no registry tags returned")
	}

	var biomeTags, enchTags *int
	for _, reg := range tags.Registries {
		switch reg.Registry {
		case "minecraft:worldgen/biome":
			n := len(reg.Tags)
			biomeTags = &n
		case "minecraft:enchantment":
			n := len(reg.Tags)
			enchTags = &n
		}
	}

	if biomeTags == nil || *biomeTags < 10 {
		t.Error("expected at least 10 biome tags")
	}
	if enchTags == nil || *enchTags < 5 {
		t.Error("expected at least 5 enchantment tags")
	}
}

func TestBuildUpdateTagsIDResolution(t *testing.T) {
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	tags := mgr.BuildUpdateTags()
	biomeReg := mgr.Registry("minecraft:worldgen/biome")
	if biomeReg == nil {
		t.Fatal("biome registry not found")
	}

	for _, reg := range tags.Registries {
		if reg.Registry != "minecraft:worldgen/biome" {
			continue
		}
		for _, tag := range reg.Tags {
			for _, id := range tag.Entries {
				name := biomeReg.NameByID(id)
				if name == "" {
					t.Errorf("tag %q contains invalid biome ID %d", tag.Name, id)
				}
			}
		}
	}
}

func TestManagerValidation(t *testing.T) {
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}
	if err := mgr.Validate(); err != nil {
		t.Fatalf("Validate() failed: %v", err)
	}
}

func TestNBTDataIntegrity(t *testing.T) {
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	dimEntries := mgr.ConfigEntries("minecraft:dimension_type")
	overworldFound := false
	for _, e := range dimEntries {
		if e.Name == "minecraft:overworld" {
			overworldFound = true
			if len(e.Data) < 20 {
				t.Errorf("overworld dimension_type NBT too short: %d bytes", len(e.Data))
			}
			if e.Data[0] != 0x0a {
				t.Errorf("overworld NBT must start with compound tag, got 0x%02x", e.Data[0])
			}
			lastByte := e.Data[len(e.Data)-1]
			if lastByte != 0x00 {
				t.Errorf("overworld NBT must end with TAG_End (0x00), got 0x%02x", lastByte)
			}
		}
	}
	if !overworldFound {
		t.Error("minecraft:overworld not found in dimension_type entries")
	}

	biomeEntries := mgr.ConfigEntries("minecraft:worldgen/biome")
	plainsFound := false
	for _, e := range biomeEntries {
		if e.Name == "minecraft:plains" {
			plainsFound = true
			if len(e.Data) < 30 {
				t.Errorf("plains biome NBT too short: %d bytes", len(e.Data))
			}
		}
	}
	if !plainsFound {
		t.Error("minecraft:plains not found in biome entries")
	}
}

func BenchmarkManagerLookup(b *testing.B) {
	mgr, err := NewManager()
	if err != nil {
		b.Fatalf("NewManager() failed: %v", err)
	}

	b.Run("IDByName_block", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mgr.IDByName("minecraft:block", "minecraft:stone")
		}
	})

	b.Run("IDByName_biome", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mgr.IDByName("minecraft:worldgen/biome", "minecraft:plains")
		}
	})

	b.Run("NameByID_block", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mgr.NameByID("minecraft:block", 1)
		}
	})
}

func BenchmarkBuildRegistryDataPackets(b *testing.B) {
	mgr, err := NewManager()
	if err != nil {
		b.Fatalf("NewManager() failed: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.BuildRegistryDataPackets(false)
	}
}
