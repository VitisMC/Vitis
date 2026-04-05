package entity

import (
	"github.com/vitismc/vitis/internal/effect"
)

// LivingEntity extends Entity with health, damage, and combat state.
// All fields are accessed exclusively from the world tick goroutine.
type LivingEntity struct {
	*Entity

	health       float32
	maxHealth    float32
	absorption   float32
	hurtTime     int32
	hurtCooldown int32
	deathTime    int32
	dead         bool

	fallDistance float64

	lastDamageSource string
	lastDamageAmount float32

	foodLevel      int32
	foodSaturation float32
	foodExhaustion float32
	foodTickTimer  int32

	xpLevel int32
	xpTotal int32
	xpBar   float32

	gameMode int32

	attackCooldown int32

	attributes *AttributeContainer
	effects    *effect.Manager
}

// NewLivingEntity creates a living entity with default health values.
func NewLivingEntity(e *Entity, maxHealth float32) *LivingEntity {
	return &LivingEntity{
		Entity:         e,
		health:         maxHealth,
		maxHealth:      maxHealth,
		foodLevel:      20,
		foodSaturation: 5.0,
		attributes:     DefaultPlayerAttributes(),
		effects:        effect.NewManager(),
	}
}

// Attributes returns the entity's attribute container.
func (l *LivingEntity) Attributes() *AttributeContainer { return l.attributes }

// AttackCooldown returns remaining attack cooldown ticks.
func (l *LivingEntity) AttackCooldown() int32 { return l.attackCooldown }

// ResetAttackCooldown sets attack cooldown to the given number of ticks.
func (l *LivingEntity) ResetAttackCooldown(ticks int32) { l.attackCooldown = ticks }

// OnGround returns whether the underlying entity is on the ground.
func (l *LivingEntity) OnGround() bool { return l.Entity.OnGround() }

// Health returns current health.
func (l *LivingEntity) Health() float32 { return l.health }

// SetHealth sets current health, clamped to [0, maxHealth].
func (l *LivingEntity) SetHealth(h float32) {
	if h < 0 {
		h = 0
	}
	if h > l.maxHealth {
		h = l.maxHealth
	}
	l.health = h
}

// MaxHealth returns maximum health.
func (l *LivingEntity) MaxHealth() float32 { return l.maxHealth }

// SetMaxHealth sets maximum health.
func (l *LivingEntity) SetMaxHealth(h float32) {
	if h < 1 {
		h = 1
	}
	l.maxHealth = h
	if l.health > l.maxHealth {
		l.health = l.maxHealth
	}
}

// Absorption returns current absorption (yellow hearts).
func (l *LivingEntity) Absorption() float32 { return l.absorption }

// SetAbsorption sets absorption amount.
func (l *LivingEntity) SetAbsorption(a float32) {
	if a < 0 {
		a = 0
	}
	l.absorption = a
}

// IsDead returns whether the entity has died.
func (l *LivingEntity) IsDead() bool { return l.dead }

// DeathTime returns the death animation tick counter.
func (l *LivingEntity) DeathTime() int32 { return l.deathTime }

// HurtTime returns the remaining hurt animation ticks.
func (l *LivingEntity) HurtTime() int32 { return l.hurtTime }

// HurtCooldown returns the remaining invulnerability ticks.
func (l *LivingEntity) HurtCooldown() int32 { return l.hurtCooldown }

// FallDistance returns current accumulated fall distance.
func (l *LivingEntity) FallDistance() float64 { return l.fallDistance }

// SetFallDistance sets the fall distance accumulator.
func (l *LivingEntity) SetFallDistance(d float64) { l.fallDistance = d }

// Damage applies damage to the entity after absorption and cooldown checks.
// Returns the actual damage dealt. Sets dead=true if health reaches zero.
func (l *LivingEntity) Damage(amount float32, source string) float32 {
	if l.dead || amount <= 0 {
		return 0
	}
	if l.hurtCooldown > 0 {
		return 0
	}

	actual := amount
	if l.absorption > 0 {
		absorbed := l.absorption
		if absorbed > actual {
			absorbed = actual
		}
		l.absorption -= absorbed
		actual -= absorbed
	}

	l.health -= actual
	if l.health <= 0 {
		l.health = 0
		l.dead = true
		l.deathTime = 1
	}

	l.hurtTime = 10
	l.hurtCooldown = 10
	l.lastDamageSource = source
	l.lastDamageAmount = amount

	return actual
}

// TickLiving advances living entity state by one tick.
func (l *LivingEntity) TickLiving() {
	if l.hurtTime > 0 {
		l.hurtTime--
	}
	if l.hurtCooldown > 0 {
		l.hurtCooldown--
	}
	if l.attackCooldown > 0 {
		l.attackCooldown--
	}
	if l.dead && l.deathTime > 0 && l.deathTime < 20 {
		l.deathTime++
	}
}

// Respawn resets health, food, and death state.
func (l *LivingEntity) Respawn() {
	l.health = l.maxHealth
	l.dead = false
	l.deathTime = 0
	l.hurtTime = 0
	l.hurtCooldown = 0
	l.fallDistance = 0
	l.absorption = 0
	l.foodLevel = 20
	l.foodSaturation = 5.0
	l.foodExhaustion = 0
	l.foodTickTimer = 0
}

// FoodLevel returns current food level (0-20).
func (l *LivingEntity) FoodLevel() int32 { return l.foodLevel }

// SetFoodLevel sets food level clamped to [0, 20].
func (l *LivingEntity) SetFoodLevel(f int32) {
	if f < 0 {
		f = 0
	}
	if f > 20 {
		f = 20
	}
	l.foodLevel = f
}

// FoodSaturation returns current saturation.
func (l *LivingEntity) FoodSaturation() float32 { return l.foodSaturation }

// SetFoodSaturation sets saturation clamped to [0, foodLevel].
func (l *LivingEntity) SetFoodSaturation(s float32) {
	if s < 0 {
		s = 0
	}
	max := float32(l.foodLevel)
	if s > max {
		s = max
	}
	l.foodSaturation = s
}

// FoodExhaustion returns accumulated exhaustion.
func (l *LivingEntity) FoodExhaustion() float32 { return l.foodExhaustion }

// AddExhaustion adds to the exhaustion counter. When >= 4, decreases saturation or food.
func (l *LivingEntity) AddExhaustion(amount float32) {
	l.foodExhaustion += amount
	for l.foodExhaustion >= 4.0 {
		l.foodExhaustion -= 4.0
		if l.foodSaturation > 0 {
			l.foodSaturation -= 1.0
			if l.foodSaturation < 0 {
				l.foodSaturation = 0
			}
		} else if l.foodLevel > 0 {
			l.foodLevel--
		}
	}
}

// TickHunger performs one hunger tick: natural regen at >=18 food, starvation at 0 food.
func (l *LivingEntity) TickHunger() {
	if l.dead || l.gameMode == 1 || l.gameMode == 3 {
		return
	}

	l.foodTickTimer++

	if l.foodLevel >= 18 && l.health < l.maxHealth {
		if l.foodTickTimer >= 80 {
			l.foodTickTimer = 0
			l.health += 1.0
			if l.health > l.maxHealth {
				l.health = l.maxHealth
			}
			l.AddExhaustion(6.0)
		}
	} else if l.foodLevel <= 0 {
		if l.foodTickTimer >= 80 {
			l.foodTickTimer = 0
			if l.health > 1.0 {
				l.health -= 1.0
			}
		}
	} else {
		l.foodTickTimer = 0
	}
}

// XPLevel returns the current experience level.
func (l *LivingEntity) XPLevel() int32 { return l.xpLevel }

// XPTotal returns total accumulated experience points.
func (l *LivingEntity) XPTotal() int32 { return l.xpTotal }

// XPBar returns the experience bar progress (0.0-1.0).
func (l *LivingEntity) XPBar() float32 { return l.xpBar }

// SetXP sets experience values.
func (l *LivingEntity) SetXP(level, total int32, bar float32) {
	l.xpLevel = level
	l.xpTotal = total
	l.xpBar = bar
}

// GameMode returns the player's game mode.
func (l *LivingEntity) GameMode() int32 { return l.gameMode }

// SetGameMode sets the game mode (0=survival, 1=creative, 2=adventure, 3=spectator).
func (l *LivingEntity) SetGameMode(mode int32) { l.gameMode = mode }

// LastDamageSource returns the source identifier of the last damage taken.
func (l *LivingEntity) LastDamageSource() string { return l.lastDamageSource }

// LastDamageAmount returns the amount of the last damage taken.
func (l *LivingEntity) LastDamageAmount() float32 { return l.lastDamageAmount }

// Effects returns the entity's effect manager.
func (l *LivingEntity) Effects() *effect.Manager { return l.effects }

// TickEffects advances all active effects and applies gameplay actions.
// Returns the diff for the caller to broadcast packets.
func (l *LivingEntity) TickEffects() effect.Diff {
	if l.effects == nil {
		return effect.Diff{}
	}
	diff := l.effects.Tick()
	for _, action := range diff.Actions {
		if action.Heal > 0 && !l.dead {
			l.health += action.Heal
			if l.health > l.maxHealth {
				l.health = l.maxHealth
			}
		}
		if action.Damage > 0 {
			l.Damage(action.Damage, action.DamageSource)
		}
		if action.Exhaustion > 0 {
			l.AddExhaustion(action.Exhaustion)
		}
		if action.FoodRestore > 0 {
			l.foodLevel += action.FoodRestore
			if l.foodLevel > 20 {
				l.foodLevel = 20
			}
		}
		if action.SatRestore > 0 {
			l.foodSaturation += action.SatRestore
			max := float32(l.foodLevel)
			if l.foodSaturation > max {
				l.foodSaturation = max
			}
		}
		if action.Absorption > 0 {
			l.absorption += action.Absorption
		}
	}
	return diff
}
