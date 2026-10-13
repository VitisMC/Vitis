package ai

import "sort"

// Goal represents one tick-owned AI behavior.
type Goal interface {
	CanStart(ctx *Context) bool
	Start(ctx *Context)
	Tick(ctx *Context)
	CanContinue(ctx *Context) bool
	Stop(ctx *Context)
}

// goalControls describes mutually exclusive AI resources.
type goalControls uint8

const (
	controlMove goalControls = 1 << iota
	controlLook
	controlJump
)

// goalEntry stores a behavior's priority and execution state.
type goalEntry struct {
	goal     Goal
	priority int
	running  bool
}

// GoalSelector executes compatible behaviors in stable priority order.
type GoalSelector struct{ entries []goalEntry }

// NewGoalSelector creates an empty selector.
func NewGoalSelector() *GoalSelector { return &GoalSelector{} }

// AddGoal registers a behavior with lower numeric priorities taking precedence.
func (gs *GoalSelector) AddGoal(priority int, goal Goal) {
	if goal == nil {
		return
	}
	gs.entries = append(gs.entries, goalEntry{goal: goal, priority: priority})
	sort.SliceStable(gs.entries, func(i, j int) bool { return gs.entries[i].priority < gs.entries[j].priority })
}

// Active returns the highest-priority running behavior for compatibility.
func (gs *GoalSelector) Active() Goal {
	for _, entry := range gs.entries {
		if entry.running {
			return entry.goal
		}
	}
	return nil
}

// controls identifies the resources consumed by existing behavior types.
func controls(goal Goal) goalControls {
	switch goal.(type) {
	case *BurnInSunlightGoal:
		return 0
	case *SwimGoal:
		return controlJump
	case *SlimeHopGoal:
		return controlMove | controlLook | controlJump
	case *LookAtPlayerGoal:
		return controlLook
	default:
		return controlMove | controlLook
	}
}

// Tick stops expired goals, admits compatible goals, and executes them once.
func (gs *GoalSelector) Tick(ctx *Context) {
	for i := range gs.entries {
		entry := &gs.entries[i]
		if entry.running && !entry.goal.CanContinue(ctx) {
			entry.goal.Stop(ctx)
			entry.running = false
		}
	}
	for i := range gs.entries {
		candidate := &gs.entries[i]
		if candidate.running {
			continue
		}
		blocked := false
		for j := range gs.entries {
			other := &gs.entries[j]
			if other.running && other.priority <= candidate.priority && controls(other.goal)&controls(candidate.goal) != 0 {
				blocked = true
				break
			}
		}
		if blocked || !candidate.goal.CanStart(ctx) {
			continue
		}
		for j := range gs.entries {
			other := &gs.entries[j]
			if other.running && controls(other.goal)&controls(candidate.goal) != 0 {
				other.goal.Stop(ctx)
				other.running = false
			}
		}
		candidate.goal.Start(ctx)
		candidate.running = true
	}
	for i := range gs.entries {
		if gs.entries[i].running && !ctx.Mob.IsDead() && !ctx.Mob.Removed() {
			gs.entries[i].goal.Tick(ctx)
		}
	}
}
