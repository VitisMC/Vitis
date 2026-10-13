package world

import (
	"context"
	"testing"
)

// TestSpawnSettingsReadsCurrentSeaLevel verifies runtime metadata is not cached in the world.
func TestSpawnSettingsReadsCurrentSeaLevel(t *testing.T) {
	level, known := 63, true
	instance, err := New(Config{Name: "spawn_settings", ReadSeaLevel: func() (int, bool) { return level, known }})
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close(context.Background())
	if settings := instance.SpawnSettings(); settings.SeaLevel != 63 || !settings.SeaLevelKnown {
		t.Fatalf("initial settings=%+v", settings)
	}
	level, known = 80, true
	if settings := instance.SpawnSettings(); settings.SeaLevel != 80 || !settings.SeaLevelKnown {
		t.Fatalf("updated settings=%+v", settings)
	}
	known = false
	if instance.SpawnSettings().SeaLevelKnown {
		t.Fatal("missing metadata retained known sea level")
	}
}
