package ai

import "github.com/vitismc/vitis/internal/entity"

// NewPassiveGoals creates a goal selector for passive mobs (cow, pig, sheep, chicken).
func NewPassiveGoals(def *entity.MobTypeDef) *GoalSelector {
	gs := NewGoalSelector()
	gs.AddGoal(0, &SwimGoal{})
	gs.AddGoal(1, &PanicGoal{Speed: def.MoveSpeed * 1.5})
	gs.AddGoal(3, &TemptGoal{Speed: def.MoveSpeed * 1.2, MaxDist: 10})
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

// NewZombieGoals creates zombie-specific goals: burn in sunlight, melee attack.
func NewZombieGoals(def *entity.MobTypeDef) *GoalSelector {
	gs := NewGoalSelector()
	gs.AddGoal(0, &SwimGoal{})
	gs.AddGoal(1, &BurnInSunlightGoal{})
	gs.AddGoal(2, &MeleeAttackGoal{Speed: def.MoveSpeed, AttackDist: 2.0, Cooldown: 20})
	gs.AddGoal(5, &WanderGoal{Speed: def.MoveSpeed, Interval: 80})
	gs.AddGoal(6, &LookAtPlayerGoal{MaxDist: 16})
	return gs
}

// NewSkeletonGoals creates skeleton-specific goals: burn in sunlight, ranged attack with strafing.
func NewSkeletonGoals(def *entity.MobTypeDef) *GoalSelector {
	gs := NewGoalSelector()
	gs.AddGoal(0, &SwimGoal{})
	if !def.FireImmune {
		gs.AddGoal(1, &AvoidSunlightGoal{Speed: def.MoveSpeed * 1.2})
		gs.AddGoal(2, &BurnInSunlightGoal{})
	}
	gs.AddGoal(3, &RangedAttackGoal{Speed: def.MoveSpeed, AttackDist: 15, Cooldown: 30, StrafeSpeed: def.MoveSpeed * 0.5, RequiresBow: true})
	gs.AddGoal(3, &MeleeAttackGoal{Speed: def.MoveSpeed, AttackDist: 2, Cooldown: 20, RequiresNoBow: true})
	gs.AddGoal(5, &WanderGoal{Speed: def.MoveSpeed, Interval: 80})
	gs.AddGoal(6, &LookAtPlayerGoal{MaxDist: 16})
	return gs
}

// NewCreeperGoals creates creeper-specific goals: swell and explode near player.
func NewCreeperGoals(def *entity.MobTypeDef) *GoalSelector {
	gs := NewGoalSelector()
	gs.AddGoal(0, &SwimGoal{})
	gs.AddGoal(1, &CreeperSwellGoal{SwellDist: 3.0, FuseTime: 30, ExplosionPw: 3.0})
	gs.AddGoal(3, &MeleeAttackGoal{Speed: def.MoveSpeed, AttackDist: 3.0, Cooldown: 20})
	gs.AddGoal(5, &WanderGoal{Speed: def.MoveSpeed, Interval: 80})
	gs.AddGoal(6, &LookAtPlayerGoal{MaxDist: 16})
	return gs
}

// NewSpiderGoals creates spider-specific goals: neutral during day, hostile at night.
func NewSpiderGoals(def *entity.MobTypeDef) *GoalSelector {
	gs := NewGoalSelector()
	gs.AddGoal(0, &SwimGoal{})
	gs.AddGoal(2, &MeleeAttackGoal{Speed: def.MoveSpeed, AttackDist: 2.0, Cooldown: 20})
	gs.AddGoal(5, &WanderGoal{Speed: def.MoveSpeed, Interval: 100})
	gs.AddGoal(6, &LookAtPlayerGoal{MaxDist: 12})
	return gs
}

// NewSpiderTargetGoals creates spider target selector: hostile only at night.
func NewSpiderTargetGoals(def *entity.MobTypeDef) *GoalSelector {
	gs := NewGoalSelector()
	gs.AddGoal(1, &SpiderNeutralGoal{MaxDist: def.FollowRange})
	return gs
}

// NewEndermanGoals creates enderman-specific goals: teleport on hurt, melee attack.
func NewEndermanGoals(def *entity.MobTypeDef) *GoalSelector {
	gs := NewGoalSelector()
	gs.AddGoal(0, &SwimGoal{})
	gs.AddGoal(1, &RandomTeleportGoal{Range: 32, Cooldown: 40})
	gs.AddGoal(2, &MeleeAttackGoal{Speed: def.MoveSpeed, AttackDist: 2.0, Cooldown: 20})
	gs.AddGoal(5, &WanderGoal{Speed: def.MoveSpeed, Interval: 100})
	gs.AddGoal(6, &LookAtPlayerGoal{MaxDist: 64})
	return gs
}

// GoalsForMob returns goal and target selectors appropriate for a mob's type.
// Dispatches to mob-specific presets where available, falling back to category defaults.
func GoalsForMob(def *entity.MobTypeDef) (goals *GoalSelector, targets *GoalSelector) {
	// Mob-specific presets
	switch def.Name {
	case "zombie", "drowned":
		return NewZombieGoals(def), NewHostileTargetGoals(def)
	case "husk":
		return NewHostileGoals(def), NewHostileTargetGoals(def)
	case "skeleton", "stray", "wither_skeleton":
		return NewSkeletonGoals(def), NewHostileTargetGoals(def)
	case "creeper":
		return NewCreeperGoals(def), NewHostileTargetGoals(def)
	case "spider", "cave_spider":
		return NewSpiderGoals(def), NewSpiderTargetGoals(def)
	case "enderman":
		return NewEndermanGoals(def), NewHostileTargetGoals(def)
	case "slime":
		goals := NewGoalSelector()
		goals.AddGoal(1, &SlimeHopGoal{})
		targets := NewGoalSelector()
		targets.AddGoal(1, &NearestAttackableTargetGoal{MaxDist: def.FollowRange, MaxVertical: 4, Interval: 10})
		return goals, targets
	}

	// Category fallbacks
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
