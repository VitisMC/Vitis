package ai

import "github.com/vitismc/vitis/internal/entity"

// NewPassiveGoals creates a goal selector for passive mobs (cow, pig, sheep, chicken).
func NewPassiveGoals(def *entity.MobTypeDef) *GoalSelector {
	gs := NewGoalSelector()
	gs.AddGoal(0, &SwimGoal{})
	gs.AddGoal(1, &PanicGoal{Speed: def.MoveSpeed * 1.5})
	gs.AddGoal(5, &WanderGoal{Speed: def.MoveSpeed, Interval: 120})
	gs.AddGoal(6, &LookAtPlayerGoal{MaxDist: 8})
	return gs
}

// NewHostileGoals creates a goal selector for hostile mobs (zombie, skeleton, etc.).
func NewHostileGoals(def *entity.MobTypeDef) *GoalSelector {
	gs := NewGoalSelector()
	gs.AddGoal(0, &SwimGoal{})
	gs.AddGoal(2, &MeleeAttackGoal{Speed: def.MoveSpeed, AttackDist: 2.0, Cooldown: 20})
	gs.AddGoal(5, &WanderGoal{Speed: def.MoveSpeed, Interval: 80})
	gs.AddGoal(6, &LookAtPlayerGoal{MaxDist: 16})
	return gs
}

// NewHostileTargetGoals creates a target selector for hostile mobs.
func NewHostileTargetGoals(def *entity.MobTypeDef) *GoalSelector {
	gs := NewGoalSelector()
	gs.AddGoal(1, &NearestAttackableTargetGoal{MaxDist: def.FollowRange, Interval: 10})
	return gs
}

// NewNeutralGoals creates a goal selector for neutral mobs (wolf, spider, enderman).
func NewNeutralGoals(def *entity.MobTypeDef) *GoalSelector {
	gs := NewGoalSelector()
	gs.AddGoal(0, &SwimGoal{})
	gs.AddGoal(2, &MeleeAttackGoal{Speed: def.MoveSpeed, AttackDist: 2.0, Cooldown: 20})
	gs.AddGoal(5, &WanderGoal{Speed: def.MoveSpeed, Interval: 100})
	gs.AddGoal(6, &LookAtPlayerGoal{MaxDist: 12})
	return gs
}

// NewAmbientGoals creates a goal selector for ambient mobs (bat).
func NewAmbientGoals(def *entity.MobTypeDef) *GoalSelector {
	gs := NewGoalSelector()
	gs.AddGoal(5, &WanderGoal{Speed: def.MoveSpeed, Interval: 200})
	return gs
}

// GoalsForMob returns goal and target selectors appropriate for a mob's category.
func GoalsForMob(def *entity.MobTypeDef) (goals *GoalSelector, targets *GoalSelector) {
	switch def.Category {
	case entity.MobCategoryHostile:
		return NewHostileGoals(def), NewHostileTargetGoals(def)
	case entity.MobCategoryNeutral:
		return NewNeutralGoals(def), nil
	case entity.MobCategoryPassive:
		return NewPassiveGoals(def), nil
	case entity.MobCategoryAmbient:
		return NewAmbientGoals(def), nil
	default:
		return NewPassiveGoals(def), nil
	}
}
