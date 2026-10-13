package entity

import (
	"math"

	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/inventory"
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
	deathHandled bool

	fallDistance float64

	lastDamageSource       string
	lastDamageAmount       float32
	lastDamageType         DamageType
	lastDamageRegistryName string
	lastDamageIsDirect     bool
	lastAttackerID         int32
	playerCreditTicks      int32
	deathWeapon            inventory.ItemStack
	lastAttackerType       string
	lastDirectType         string

	airSupply  int32
	fireTicks  int32
	fireImmune bool

	foodLevel      int32
	foodSaturation float32
	foodExhaustion float32
	foodTickTimer  int32

	xpLevel int32
	xpTotal int32
	xpBar   float32

	gameMode int32

	attackCooldown int32

	armorDefense        float64
	armorToughness      float64
	armorEnchProt       float64
	armorFireProt       float64
	armorBlastProt      float64
	armorProjectileProt float64
	armorFallProt       float64
	armorDamageHandler  func(*DamageSource, float32)

	blocking       bool
	shieldCooldown int32

	attributes                *AttributeContainer
	effects                   *effect.Manager
	effectMovementLevels      [2]int32
	effectMovementInitialized bool
}

// NewLivingEntity creates a living entity with default health values.
func NewLivingEntity(e *Entity, maxHealth float32) *LivingEntity {
	return &LivingEntity{
		Entity:         e,
		health:         maxHealth,
		maxHealth:      maxHealth,
		foodLevel:      20,
		foodSaturation: 5.0,
		airSupply:      300,
		lastAttackerID: -1,
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

// ArmorDefense returns cached total armor defense points.
func (l *LivingEntity) ArmorDefense() float64 { return l.armorDefense }

// ArmorToughness returns cached total armor toughness.
func (l *LivingEntity) ArmorToughness() float64 { return l.armorToughness }

// ArmorEnchProtection returns cached total enchantment protection factor.
func (l *LivingEntity) ArmorEnchProtection() float64 { return l.armorEnchProt }

// SetArmorStats updates cached armor defense, toughness, and enchantment protection.
func (l *LivingEntity) SetArmorStats(defense, toughness, enchProt float64) {
	l.armorDefense = defense
	l.armorToughness = toughness
	l.armorEnchProt = enchProt
}

// SetArmorDamageHandler binds tick-owned equipment wear to accepted damage events.
func (l *LivingEntity) SetArmorDamageHandler(handler func(*DamageSource, float32)) {
	l.armorDamageHandler = handler
}

// SetArmorDamageProtection caches enchantments that apply only to specific damage types.
func (l *LivingEntity) SetArmorDamageProtection(fire, blast, projectile, fall float64) {
	l.armorFireProt, l.armorBlastProt, l.armorProjectileProt, l.armorFallProt = fire, blast, projectile, fall
}

// damageEnchantmentBonus selects protection for the current structured damage source.
func (l *LivingEntity) damageEnchantmentBonus(source DamageType) float64 {
	switch {
	case source.IsFire():
		return l.armorFireProt
	case source.IsExplosion():
		return l.armorBlastProt
	case source.IsProjectile():
		return l.armorProjectileProt
	case source == DamageTypeFall:
		return l.armorFallProt
	default:
		return 0
	}
}

// IsBlocking returns true if the entity is actively blocking with a shield.
func (l *LivingEntity) IsBlocking() bool { return l.blocking }

// SetBlocking sets the shield blocking state.
func (l *LivingEntity) SetBlocking(v bool) { l.blocking = v }

// ShieldCooldown returns remaining shield cooldown ticks.
func (l *LivingEntity) ShieldCooldown() int32 { return l.shieldCooldown }

// SetShieldCooldown sets the shield cooldown (e.g. 100 ticks = 5 seconds after axe hit).
func (l *LivingEntity) SetShieldCooldown(ticks int32) { l.shieldCooldown = ticks }

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
	l.Entity.dirty |= DirtyLivingState
	if h == 0 {
		l.dead = true
		if l.deathTime == 0 {
			l.deathTime = 1
		}
	}
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
	kind := damageTypeFromSource(source)
	actual := l.ApplyDamage(NewDamageSource(kind, amount), 0, 0, 0)
	if actual > 0 {
		l.lastDamageSource = source
	}
	return actual
}

// DeathHandled reports whether one-time death drops have been processed.
func (l *LivingEntity) DeathHandled() bool { return l.deathHandled }

// MarkDeathHandled prevents duplicate drops while the death screen is open.
func (l *LivingEntity) MarkDeathHandled() { l.deathHandled = true }

// TickLiving advances living entity state by one tick.
func (l *LivingEntity) TickLiving() {
	if l.playerCreditTicks > 0 && !l.dead {
		l.playerCreditTicks--
	}
	if l.hurtTime > 0 {
		l.hurtTime--
	}
	if l.hurtCooldown > 0 {
		l.hurtCooldown--
	}
	if l.attackCooldown > 0 {
		l.attackCooldown--
	}
	if l.shieldCooldown > 0 {
		l.shieldCooldown--
		if l.shieldCooldown <= 0 {
			l.blocking = false
		}
	}
	if l.dead && l.deathTime > 0 && l.deathTime < 20 {
		l.deathTime++
	}
}

// Respawn resets health, food, experience, effects, and death state.
func (l *LivingEntity) Respawn() {
	l.RespawnWithKeepExperience(false)
}

// RespawnWithKeepExperience preserves XP only when keepInventory is enabled.
func (l *LivingEntity) RespawnWithKeepExperience(keepExperience bool) {
	l.playerCreditTicks = 0
	l.deathWeapon = inventory.ItemStack{}
	l.lastAttackerType = ""
	l.lastDirectType = ""
	l.health = l.maxHealth
	l.dead = false
	l.deathHandled = false
	l.deathTime = 0
	l.hurtTime = 0
	l.hurtCooldown = 0
	l.fallDistance = 0
	l.absorption = 0
	l.foodLevel = 20
	l.foodSaturation = 5.0
	l.foodExhaustion = 0
	l.foodTickTimer = 0
	if !keepExperience {
		l.xpLevel = 0
		l.xpTotal = 0
		l.xpBar = 0
	}
	l.effects.Clear()
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

// SetFoodExhaustion restores a persisted exhaustion value.
func (l *LivingEntity) SetFoodExhaustion(value float32) {
	if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) || value < 0 {
		value = 0
	}
	if value > 4 {
		value = 4
	}
	l.foodExhaustion = value
}

// AddExhaustion adds to the exhaustion counter. When >= 4, decreases saturation or food.
func (l *LivingEntity) AddExhaustion(amount float32) {
	if math.IsNaN(float64(amount)) || math.IsInf(float64(amount), 0) || amount <= 0 {
		return
	}
	if amount > 40 {
		amount = 40
	}
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

// TickHunger performs one normal-difficulty hunger tick.
func (l *LivingEntity) TickHunger() {
	l.TickHungerWithDifficulty(2)
}

// TickHungerWithDifficulty applies natural regeneration and difficulty-capped starvation.
func (l *LivingEntity) TickHungerWithDifficulty(difficulty int32) {
	l.TickHungerWithRules(difficulty, true)
}

// TickHungerWithRules applies regeneration and starvation under current rules.
func (l *LivingEntity) TickHungerWithRules(difficulty int32, naturalRegeneration bool) {
	if l.dead || l.gameMode == 1 || l.gameMode == 3 {
		return
	}
	if difficulty == 0 {
		if l.foodLevel < 20 && l.foodTickTimer%10 == 0 {
			l.foodLevel++
		}
		if naturalRegeneration && l.health < l.maxHealth && l.foodTickTimer%20 == 0 {
			l.health = min(l.maxHealth, l.health+1)
		}
		l.foodTickTimer++
		return
	}

	l.foodTickTimer++

	if naturalRegeneration && l.foodLevel == 20 && l.foodSaturation > 0 && l.health < l.maxHealth {
		if l.foodTickTimer >= 10 {
			l.foodTickTimer = 0
			amount := min(l.foodSaturation, float32(6))
			l.health = min(l.maxHealth, l.health+amount/6)
			l.AddExhaustion(amount)
		}
	} else if naturalRegeneration && l.foodLevel >= 18 && l.health < l.maxHealth {
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
			floor := float32(1)
			if difficulty == 1 {
				floor = 10
			} else if difficulty >= 3 {
				floor = 0
			}
			if l.health > floor {
				l.Damage(1, "starve")
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
	l.SyncEffectMovement()
	diff := l.effects.Tick()
	l.SyncEffectMovement()
	for _, action := range diff.Actions {
		if action.Heal > 0 && !l.dead {
			l.SetHealth(min(l.health+action.Heal, l.maxHealth))
		}
		if action.Damage > 0 {
			if action.DamageSource != "poison" || l.health > 1 {
				l.Damage(action.Damage, action.DamageSource)
			}
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
