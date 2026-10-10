package ai

import (
	"testing"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestSpeciesPresetsUseCorrectSunlightAndWeapons prevents inherited goals from changing variant behavior.
func TestSpeciesPresetsUseCorrectSunlightAndWeapons(t *testing.T) {
	for _, name := range []string{"husk", "wither_skeleton", "zombie", "skeleton"} {
		mob := entity.NewMobEntityByName(1, protocol.UUID{1}, name, entity.Vec3{Y: 64})
		world := &actionTestWorld{visible: true, sunlit: true}
		players := []PlayerInfo{{EntityID: 2, Pos: entity.Vec3{X: 1, Y: 64}}}
		ctx := testContext(mob, players)
		ctx.Blocks = world
		mob.SetTarget(2)
		brain := NewBrain(mob.TypeDef())
		for _, entry := range brain.Goals.entries {
			if sunlight, ok := entry.goal.(*BurnInSunlightGoal); ok {
				sunlight.random = func() float32 { return 0 }
			}
		}
		for range 40 {
			brain.Goals.Tick(ctx)
		}
		if name == "husk" || name == "wither_skeleton" {
			if mob.FireTicks() != 0 || world.melee == 0 || world.arrows != 0 {
				t.Fatalf("%s burned or used the wrong weapon: hits=%d arrows=%d", name, world.melee, world.arrows)
			}
		} else if name == "zombie" && mob.FireTicks() == 0 {
			t.Fatal("zombie lost sunlight burning")
		}
	}
}
