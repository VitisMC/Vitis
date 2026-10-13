package worldgen

import (
	"testing"

	"github.com/vitismc/vitis/internal/datapack"
)

// TestReleaseGenerationCachesPreservesContent validates rebuilds in every dimension.
func TestReleaseGenerationCachesPreservesContent(t *testing.T) {
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	for _, dimension := range []string{"minecraft:overworld", "minecraft:nether", "minecraft:end"} {
		t.Run(dimension, func(t *testing.T) {
			generator, err := runtime.NewTerrainGenerator(dimension, 345678765456, 0)
			if err != nil {
				t.Fatal(err)
			}
			generator.EnableDiagnostics()
			before, err := generator.Generate(-7, -31)
			if err != nil {
				t.Fatal(err)
			}
			before.ComputeLight()
			counts := generator.Diagnostics()
			if !generator.TryReleaseGenerationCaches() {
				t.Fatal("idle generation could not be released")
			}
			if generator.compiled != nil || generator.resource != nil || generator.rawCache != nil || generator.baseCache != nil || generator.baseActive != nil || generator.rawTerrains != 0 || generator.rawBorders != 0 || generator.terrainLRU.Len() != 0 || generator.borderLRU.Len() != 0 || generator.borderSeamLRU.Len() != 0 || generator.baseLRU.Len() != 0 || generator.baseSeamLRU.Len() != 0 {
				t.Fatal("generation state retained after release")
			}
			if got := generator.Diagnostics(); got != counts {
				t.Fatal("releasing history changed diagnostic totals")
			}
			after, err := generator.Generate(-7, -31)
			if err != nil {
				t.Fatal(err)
			}
			after.ComputeLight()
			assertGeneratedContent(t, after, before)
			if err := resources.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
				t.Fatal(err)
			}
			if !generator.TryReleaseGenerationCaches() || !generator.TryReleaseGenerationCaches() {
				t.Fatal("release is not repeatable after reload")
			}
		})
	}
}

// TestReleaseGenerationCachesDefersBusyState prevents invalidating pending publications.
func TestReleaseGenerationCachesDefersBusyState(t *testing.T) {
	g := &TerrainGenerator{compiled: &terrainSnapshot{}}
	g.generationMu.RLock()
	if g.TryReleaseGenerationCaches() {
		t.Fatal("released an active generation")
	}
	g.generationMu.RUnlock()
	g.mu.Lock()
	if g.TryReleaseGenerationCaches() {
		t.Fatal("released an active compilation")
	}
	g.mu.Unlock()
	g.rawMu.Lock()
	if g.TryReleaseGenerationCaches() {
		t.Fatal("released a busy raw cache")
	}
	g.rawMu.Unlock()
	g.baseMu.Lock()
	if g.TryReleaseGenerationCaches() {
		t.Fatal("released a busy terrain cache")
	}
	g.baseMu.Unlock()
	if g.compiled == nil || !g.TryReleaseGenerationCaches() || g.compiled != nil {
		t.Fatal("busy state was changed or idle state was retained")
	}
}
