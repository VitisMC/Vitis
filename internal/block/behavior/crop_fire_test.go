package behavior

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
)

// TestCropRandomGrowthAndMaturity checks light, randomness, and scheduled tick removal.
func TestCropRandomGrowthAndMaturity(t *testing.T) {
	initial := block.StateID("minecraft:wheat", map[string]string{"age": "0"})
	b := GetByState(initial)
	if b.ScheduleOnPlace() || b.OnScheduledTick(&TickContext{StateID: initial}) != nil {
		t.Fatal("crop still grows from scheduled ticks")
	}
	random := b.(RandomTickBehavior)
	ctx := &RandomTickContext{
		StateID: initial, Light: 9,
		GetBlock:    func(int, int, int) (int32, bool) { return 0, true },
		RandomBound: func(int32) int32 { return 0 },
	}
	result := random.OnRandomTick(ctx)
	if result == nil || !result.SetBlock || result.Reschedule || block.PropertiesFromState(result.NewBlockState)["age"] != "1" {
		t.Fatalf("crop growth = %+v", result)
	}
	ctx.Light = 8
	if random.OnRandomTick(ctx) != nil {
		t.Fatal("crop grew below light threshold")
	}
	ctx.Light = 15
	ctx.RandomBound = func(int32) int32 { return 1 }
	if random.OnRandomTick(ctx) != nil {
		t.Fatal("crop ignored growth chance")
	}
	ctx.StateID = block.StateID("minecraft:wheat", map[string]string{"age": "7"})
	if random.CanRandomTick(ctx.StateID) || random.OnRandomTick(ctx) != nil {
		t.Fatal("mature crop kept growing")
	}
}

func TestFireSchedulesAfterPlacement(t *testing.T) {
	state := block.DefaultStateID("minecraft:fire")
	if state < 0 {
		t.Fatal("fire state unavailable")
	}
	b := GetByState(state)
	if !b.ScheduleOnPlace() || !b.ScheduleOnNeighborUpdate() || b.TickDelay() <= 0 {
		t.Fatal("fire behavior is not scheduled")
	}
}

func TestGravelKeepsFallingBehaviorWithCustomDrops(t *testing.T) {
	state := block.DefaultStateID("minecraft:gravel")
	if state < 0 {
		t.Fatal("gravel state unavailable")
	}
	b := GetByState(state)
	result := b.OnScheduledTick(&TickContext{StateID: state})
	if !b.ScheduleOnPlace() || !b.ScheduleOnNeighborUpdate() || result == nil || !result.SpawnFallingBlock || result.BlockStateID != state {
		t.Fatalf("gravel falling behavior = %+v", result)
	}
	if drops := b.OnBreak(&Context{StateID: state, ToolEnchantments: map[int32]int32{33: 1}}); len(drops) != 1 {
		t.Fatalf("gravel custom drops = %+v", drops)
	}
}
