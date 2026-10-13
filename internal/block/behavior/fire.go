package behavior

import "github.com/vitismc/vitis/internal/block"

// FireBehavior schedules the world-owned fire simulation after placement.
type FireBehavior struct{ DefaultBehavior }

// ScheduleOnPlace starts fire updates for a newly placed block.
func (*FireBehavior) ScheduleOnPlace() bool { return true }

// ScheduleOnNeighborUpdate rechecks fire when its support changes.
func (*FireBehavior) ScheduleOnNeighborUpdate() bool { return true }

// TickDelay is the interval between basic fire updates.
func (*FireBehavior) TickDelay() int { return 30 }

func init() {
	for _, name := range []string{"minecraft:fire", "minecraft:soul_fire"} {
		if info := block.InfoByName(name); info != nil {
			Register(info.ID, &FireBehavior{})
		}
	}
}
