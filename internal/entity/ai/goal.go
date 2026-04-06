package ai

// Goal represents a single AI behavior that can be started, ticked, and stopped.
type Goal interface {
	CanStart(ctx *Context) bool
	Start(ctx *Context)
	Tick(ctx *Context)
	CanContinue(ctx *Context) bool
	Stop(ctx *Context)
}

type goalEntry struct {
	goal     Goal
	priority int
}

// GoalSelector manages a priority-ordered list of goals for a mob.
// Lower priority values are more important (checked first).
type GoalSelector struct {
	entries        []goalEntry
	active         Goal
	activePriority int
}

// NewGoalSelector creates an empty goal selector.
func NewGoalSelector() *GoalSelector {
	return &GoalSelector{
		activePriority: -1,
	}
}

// AddGoal registers a goal with the given priority.
func (gs *GoalSelector) AddGoal(priority int, goal Goal) {
	gs.entries = append(gs.entries, goalEntry{goal: goal, priority: priority})
}

// Active returns the currently active goal, or nil.
func (gs *GoalSelector) Active() Goal {
	return gs.active
}

// Tick evaluates goals and ticks the active one.
func (gs *GoalSelector) Tick(ctx *Context) {
	best := gs.findBestGoal(ctx)

	if best != gs.active {
		if gs.active != nil {
			gs.active.Stop(ctx)
		}
		gs.active = best
		if gs.active != nil {
			gs.active.Start(ctx)
		}
	}

	if gs.active != nil {
		if gs.active.CanContinue(ctx) {
			gs.active.Tick(ctx)
		} else {
			gs.active.Stop(ctx)
			gs.active = nil
			gs.activePriority = -1
		}
	}
}

func (gs *GoalSelector) findBestGoal(ctx *Context) Goal {
	var bestGoal Goal
	bestPriority := int(^uint(0) >> 1)

	if gs.active != nil && gs.active.CanContinue(ctx) {
		bestGoal = gs.active
		bestPriority = gs.activePriority
	}

	for _, entry := range gs.entries {
		if entry.priority >= bestPriority {
			continue
		}
		if entry.goal == gs.active {
			continue
		}
		if entry.goal.CanStart(ctx) {
			bestGoal = entry.goal
			bestPriority = entry.priority
		}
	}

	gs.activePriority = bestPriority
	return bestGoal
}
