package entity

import (
	"math"

	genattr "github.com/vitismc/vitis/internal/data/generated/attribute"
	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	genent "github.com/vitismc/vitis/internal/data/generated/entity"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
)

// MobCategory classifies mob behavior.
type MobCategory uint8

const (
	MobCategoryPassive MobCategory = iota
	MobCategoryHostile
	MobCategoryNeutral
	MobCategoryAmbient
	MobCategoryWaterCreature
)

// LootDrop represents a single possible loot drop from a mob.
type LootDrop struct {
	ItemName string
	MinCount int32
	MaxCount int32
	Chance   float64
}

// MobTypeDef defines the static properties of a mob type.
type MobTypeDef struct {
	ProtocolID   int32
	Name         string
	Category     MobCategory
	MaxHealth    float32
	AttackDmg    float32
	MoveSpeed    float64
	FollowRange  float64
	KnockbackRes float64
	Width        float64
	Height       float64
	XPReward     int32
	FireImmune   bool
	Drops        []LootDrop
}

// MobEntity extends LivingEntity with mob-specific state and AI support.
type MobEntity struct {
	*LivingEntity

	typeDef             *MobTypeDef
	noAI                bool
	target              int32
	ageTicks            int64
	persistenceRequired bool
	canPickUpLoot       bool
	noActionTime        int32
	swellDirection      int32
	slimeSize           int
	effectDiff          effect.Diff
	equipment           [7]inventory.ItemStack
	equipmentDropChance [7]float32
	equipmentDirty      uint8
	equipmentBroken     uint8
	equipmentModifiers  map[equipmentModifierKey]Modifier
}

// PersistenceRequired reports whether distance-based despawn is disabled.
func (m *MobEntity) PersistenceRequired() bool { return m.persistenceRequired }

// SetPersistenceRequired controls persistence for named, equipped and owned mobs.
func (m *MobEntity) SetPersistenceRequired(required bool) { m.persistenceRequired = required }

// CanPickUpLoot reports whether the mob may equip dropped items.
func (m *MobEntity) CanPickUpLoot() bool { return m.canPickUpLoot }

// SetCanPickUpLoot controls item pickup without changing despawn persistence.
func (m *MobEntity) SetCanPickUpLoot(enabled bool) { m.canPickUpLoot = enabled }

// NoActionTime returns the mob's idle lifetime used by natural despawn rules.
func (m *MobEntity) NoActionTime() int32 { return m.noActionTime }

// SetNoActionTime updates the idle lifetime on the owning world tick.
func (m *MobEntity) SetNoActionTime(ticks int32) { m.noActionTime = max(0, ticks) }

// NewMobEntity creates a mob entity from a type definition.
func NewMobEntity(id int32, uuid protocol.UUID, def *MobTypeDef, pos Vec3, rot Vec2) *MobEntity {
	base := NewEntity(id, uuid, EntityTypeMob, pos, rot)
	base.SetProtocolInfo(def.ProtocolID, 0)
	le := NewLivingEntity(base, def.MaxHealth)
	le.attributes = mobAttributes(def)
	le.fireImmune = def.FireImmune
	le.effects = effect.NewManagerWithFilter(mobEffectPolicy(def.Name))

	mob := &MobEntity{
		LivingEntity:   le,
		typeDef:        def,
		target:         -1,
		swellDirection: -1,
	}
	if def.Name == "slime" {
		mob.SetSlimeSize(1, true)
	}
	mob.initializeEquipment()
	return mob
}

// NewMobEntityByName creates a mob entity by looking up the entity name.
// Returns nil if the name is unknown or has no registered type definition.
func NewMobEntityByName(id int32, uuid protocol.UUID, name string, pos Vec3) *MobEntity {
	def := GetMobTypeDef(name)
	if def == nil {
		return nil
	}
	return NewMobEntity(id, uuid, def, pos, Vec2{})
}

// TypeDef returns the mob's instance properties used by physics and behavior.
func (m *MobEntity) TypeDef() *MobTypeDef { return m.typeDef }

// TypeName returns the mob's registered name.
func (m *MobEntity) TypeName() string { return m.typeDef.Name }

// MobType returns the protocol entity type ID.
func (m *MobEntity) MobType() int32 { return m.typeDef.ProtocolID }

// Category returns the mob's behavior category.
func (m *MobEntity) Category() MobCategory { return m.typeDef.Category }

// NoAI returns whether AI is disabled.
func (m *MobEntity) NoAI() bool { return m.noAI }

// SetNoAI enables or disables AI.
func (m *MobEntity) SetNoAI(v bool) { m.noAI = v; m.Entity.dirty |= DirtyLivingState }

// SwellDirection returns the creeper's client-visible fuse direction.
func (m *MobEntity) SwellDirection() int32 { return m.swellDirection }

// SetSwellDirection marks a creeper's fuse direction for observer synchronization.
func (m *MobEntity) SetSwellDirection(direction int32) {
	if direction != m.swellDirection {
		m.swellDirection = direction
		m.Entity.dirty |= DirtyLivingState
	}
}

// LootTraits captures supported saved species properties without exposing mutable NBT.
func (m *MobEntity) LootTraits() (baby bool, color string, sheared bool, size int) {
	color, size = "white", 4
	if m.TypeName() == "slime" {
		size = m.SlimeSize()
	}
	if m.persistent == nil {
		return
	}
	if age, ok := m.persistent.GetInt("Age"); ok {
		baby = age < 0
	}
	if savedBaby, ok := m.persistent.GetBool("IsBaby"); ok {
		baby = savedBaby
	}
	if wool, ok := m.persistent.GetByte("Color"); ok && wool >= 0 && wool < 16 {
		color = [...]string{"white", "orange", "magenta", "light_blue", "yellow", "lime", "pink", "gray", "light_gray", "cyan", "purple", "blue", "brown", "green", "red", "black"}[wool]
	}
	sheared, _ = m.persistent.GetBool("Sheared")
	if savedSize, ok := m.persistent.GetInt("Size"); ok && m.TypeName() != "slime" && savedSize >= 0 && savedSize <= 126 {
		size = int(savedSize) + 1
	}
	return
}

// Target returns the entity ID of the current attack target, or -1.
func (m *MobEntity) Target() int32 { return m.target }

// SetTarget sets the current attack target entity ID.
func (m *MobEntity) SetTarget(id int32) { m.target = id }

// AgeTicks returns how many ticks this mob has existed.
func (m *MobEntity) AgeTicks() int64 { return m.ageTicks }

// XPReward returns the base death experience, including supported baby-species rules.
func (m *MobEntity) XPReward() int32 {
	baby, _, _, _ := m.LootTraits()
	if baby {
		switch m.TypeName() {
		case "zombie", "husk", "drowned":
			return m.typeDef.XPReward * 5 / 2
		default:
			return 0
		}
	}
	return m.typeDef.XPReward
}

// mobEffectPolicy captures immutable effect immunities for supported species.
func mobEffectPolicy(name string) func(effect.Instance) bool {
	return func(instance effect.Instance) bool {
		switch name {
		case "zombie", "husk", "drowned", "skeleton", "stray", "wither_skeleton", "phantom":
			if instance.ID == effectdata.EffectPoison || instance.ID == effectdata.EffectRegeneration {
				return false
			}
		}
		if name == "wither_skeleton" && instance.ID == effectdata.EffectWither {
			return false
		}
		return (name != "spider" && name != "cave_spider") || instance.ID != effectdata.EffectPoison
	}
}

// Living returns the underlying LivingEntity.
func (m *MobEntity) Living() *LivingEntity { return m.LivingEntity }

// TickMob advances mob state for one tick, applying gravity and collision.
func (m *MobEntity) TickMob(world physics.BlockAccess) {
	m.LivingEntity.TickLiving()
	m.effectDiff = m.LivingEntity.TickEffects()
	m.effectDiff.Actions = nil
	m.ageTicks++

	if m.IsDead() && m.DeathTime() >= 20 {
		m.Entity.Remove()
		return
	}

	if world == nil {
		return
	}
	if !m.IsDead() {
		for _, source := range m.TickEnvironment(world.GetBlockStateAt) {
			m.ApplyDamage(source, m.ArmorDefense(), m.ArmorToughness(), m.ArmorEnchProtection())
		}
	}

	gravity := 0.08
	if attr, ok := m.attributes.attrs[genattr.AttrGravity]; ok {
		gravity = attr.Base
	}
	stepHeight := 0.6
	if attr, ok := m.attributes.attrs[genattr.AttrStepHeight]; ok {
		stepHeight = attr.Base
	}

	vel := m.Entity.Velocity()
	vel.Y -= gravity
	if vel.Y < physics.TerminalVelocity {
		vel.Y = physics.TerminalVelocity
	}

	dims := physics.EntityDimensions{Width: m.typeDef.Width, Height: m.typeDef.Height}
	box := dims.MakeBoundingBox(m.Entity.pos.X, m.Entity.pos.Y, m.Entity.pos.Z)

	result := physics.MoveWithStepUp(world, box, vel.X, vel.Y, vel.Z, stepHeight, m.Entity.onGround)

	m.Entity.pos.X += result.Dx
	m.Entity.pos.Y += result.Dy
	m.Entity.pos.Z += result.Dz
	m.Entity.dirty |= DirtyPosition
	m.Entity.onGround = result.OnGround

	if result.CollidedX {
		vel.X = 0
	}
	if result.CollidedY {
		vel.Y = 0
	}
	if result.CollidedZ {
		vel.Z = 0
	}

	if m.Entity.onGround {
		vel.X *= 0.91 * 0.6
		vel.Z *= 0.91 * 0.6
	} else {
		vel.X *= 0.91
		vel.Z *= 0.91
	}

	m.Entity.vel = vel

	if m.Entity.pos.Y < -128 {
		m.Entity.Remove()
	}
}

// ConsumeEffectDiff returns and clears effect updates produced by the last mob tick.
func (m *MobEntity) ConsumeEffectDiff() effect.Diff {
	diff := m.effectDiff
	m.effectDiff = effect.Diff{}
	return diff
}

// DistanceTo returns the distance to another position.
func (m *MobEntity) DistanceTo(target Vec3) float64 {
	pos := m.Position()
	dx := pos.X - target.X
	dy := pos.Y - target.Y
	dz := pos.Z - target.Z
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// DistanceToSq returns the squared distance to another position.
func (m *MobEntity) DistanceToSq(target Vec3) float64 {
	pos := m.Position()
	dx := pos.X - target.X
	dy := pos.Y - target.Y
	dz := pos.Z - target.Z
	return dx*dx + dy*dy + dz*dz
}

func mobAttributes(def *MobTypeDef) *AttributeContainer {
	ac := NewAttributeContainer()
	ac.attrs[genattr.AttrMaxHealth] = &Attribute{Base: float64(def.MaxHealth)}
	ac.attrs[genattr.AttrMovementSpeed] = &Attribute{Base: def.MoveSpeed}
	ac.attrs[genattr.AttrAttackDamage] = &Attribute{Base: float64(def.AttackDmg)}
	ac.attrs[genattr.AttrFollowRange] = &Attribute{Base: def.FollowRange}
	ac.attrs[genattr.AttrKnockbackResistance] = &Attribute{Base: def.KnockbackRes}
	ac.attrs[genattr.AttrArmor] = &Attribute{}
	ac.attrs[genattr.AttrArmorToughness] = &Attribute{}
	ac.attrs[genattr.AttrAttackKnockback] = &Attribute{}
	if def.Name == "zombie" || def.Name == "husk" || def.Name == "drowned" {
		ac.attrs[genattr.AttrArmor].Base = 2
	}
	ac.attrs[genattr.AttrGravity] = &Attribute{Base: 0.08}
	ac.attrs[genattr.AttrStepHeight] = &Attribute{Base: 0.6}
	return ac
}

var mobTypeDefs = map[string]*MobTypeDef{}

// RegisterMobTypeDef registers a mob type definition by name.
// It auto-fills ProtocolID, Width, and Height from generated entity data if unset.
func RegisterMobTypeDef(def *MobTypeDef) {
	if info := genent.EntityByName(def.Name); info != nil {
		if def.ProtocolID == 0 {
			def.ProtocolID = info.ID
		}
		if def.Width == 0 {
			def.Width = info.Width
		}
		if def.Height == 0 {
			def.Height = info.Height
		}
	}
	mobTypeDefs[def.Name] = def
}

// GetMobTypeDef returns the mob type definition for the given name, or nil.
func GetMobTypeDef(name string) *MobTypeDef {
	return mobTypeDefs[name]
}

// AllMobTypeDefs returns all registered mob type definitions.
func AllMobTypeDefs() map[string]*MobTypeDef {
	return mobTypeDefs
}

func init() {
	registerCoreMobs()
}

func registerCoreMobs() {
	defs := []MobTypeDef{
		{Name: "zombie", Category: MobCategoryHostile, MaxHealth: 20, AttackDmg: 3, MoveSpeed: 0.23, FollowRange: 35, XPReward: 5,
			Drops: []LootDrop{{ItemName: "minecraft:rotten_flesh", MinCount: 0, MaxCount: 2, Chance: 1.0}}},
		{Name: "skeleton", Category: MobCategoryHostile, MaxHealth: 20, AttackDmg: 2, MoveSpeed: 0.25, FollowRange: 16, XPReward: 5,
			Drops: []LootDrop{
				{ItemName: "minecraft:bone", MinCount: 0, MaxCount: 2, Chance: 1.0},
				{ItemName: "minecraft:arrow", MinCount: 0, MaxCount: 2, Chance: 1.0},
			}},
		{Name: "creeper", Category: MobCategoryHostile, MaxHealth: 20, AttackDmg: 0, MoveSpeed: 0.25, FollowRange: 16, XPReward: 5,
			Drops: []LootDrop{{ItemName: "minecraft:gunpowder", MinCount: 0, MaxCount: 2, Chance: 1.0}}},
		{Name: "spider", Category: MobCategoryNeutral, MaxHealth: 16, AttackDmg: 2, MoveSpeed: 0.3, FollowRange: 16, XPReward: 5,
			Drops: []LootDrop{
				{ItemName: "minecraft:string", MinCount: 0, MaxCount: 2, Chance: 1.0},
				{ItemName: "minecraft:spider_eye", MinCount: 0, MaxCount: 1, Chance: 0.33},
			}},
		{Name: "enderman", Category: MobCategoryNeutral, MaxHealth: 40, AttackDmg: 7, MoveSpeed: 0.3, FollowRange: 64, XPReward: 5,
			Drops: []LootDrop{{ItemName: "minecraft:ender_pearl", MinCount: 0, MaxCount: 1, Chance: 1.0}}},
		{Name: "cow", Category: MobCategoryPassive, MaxHealth: 10, AttackDmg: 0, MoveSpeed: 0.2, FollowRange: 16, XPReward: 3,
			Drops: []LootDrop{
				{ItemName: "minecraft:beef", MinCount: 1, MaxCount: 3, Chance: 1.0},
				{ItemName: "minecraft:leather", MinCount: 0, MaxCount: 2, Chance: 1.0},
			}},
		{Name: "pig", Category: MobCategoryPassive, MaxHealth: 10, AttackDmg: 0, MoveSpeed: 0.25, FollowRange: 16, XPReward: 3,
			Drops: []LootDrop{{ItemName: "minecraft:porkchop", MinCount: 1, MaxCount: 3, Chance: 1.0}}},
		{Name: "sheep", Category: MobCategoryPassive, MaxHealth: 8, AttackDmg: 0, MoveSpeed: 0.23, FollowRange: 16, XPReward: 3,
			Drops: []LootDrop{
				{ItemName: "minecraft:mutton", MinCount: 1, MaxCount: 2, Chance: 1.0},
				{ItemName: "minecraft:white_wool", MinCount: 1, MaxCount: 1, Chance: 1.0},
			}},
		{Name: "chicken", Category: MobCategoryPassive, MaxHealth: 4, AttackDmg: 0, MoveSpeed: 0.25, FollowRange: 16, XPReward: 3,
			Drops: []LootDrop{
				{ItemName: "minecraft:chicken", MinCount: 1, MaxCount: 1, Chance: 1.0},
				{ItemName: "minecraft:feather", MinCount: 0, MaxCount: 2, Chance: 1.0},
			}},
		{Name: "wolf", Category: MobCategoryNeutral, MaxHealth: 8, AttackDmg: 4, MoveSpeed: 0.3, FollowRange: 16, XPReward: 3},
		{Name: "bat", Category: MobCategoryAmbient, MaxHealth: 6, AttackDmg: 0, MoveSpeed: 0.1, FollowRange: 16, XPReward: 0},
		{Name: "iron_golem", Category: MobCategoryPassive, MaxHealth: 100, AttackDmg: 15, MoveSpeed: 0.25, FollowRange: 16, KnockbackRes: 1.0, XPReward: 0,
			Drops: []LootDrop{
				{ItemName: "minecraft:iron_ingot", MinCount: 3, MaxCount: 5, Chance: 1.0},
				{ItemName: "minecraft:poppy", MinCount: 0, MaxCount: 2, Chance: 1.0},
			}},
		{Name: "snow_golem", Category: MobCategoryPassive, MaxHealth: 4, AttackDmg: 0, MoveSpeed: 0.2, FollowRange: 16, XPReward: 0,
			Drops: []LootDrop{{ItemName: "minecraft:snowball", MinCount: 0, MaxCount: 15, Chance: 1.0}}},
		{Name: "husk", Category: MobCategoryHostile, MaxHealth: 20, AttackDmg: 3, MoveSpeed: 0.23, FollowRange: 35, XPReward: 5,
			Drops: []LootDrop{{ItemName: "minecraft:rotten_flesh", MinCount: 0, MaxCount: 2, Chance: 1.0}}},
		{Name: "drowned", Category: MobCategoryHostile, MaxHealth: 20, AttackDmg: 3, MoveSpeed: 0.23, FollowRange: 35, XPReward: 5,
			Drops: []LootDrop{{ItemName: "minecraft:rotten_flesh", MinCount: 0, MaxCount: 2, Chance: 1.0}}},
		{Name: "stray", Category: MobCategoryHostile, MaxHealth: 20, AttackDmg: 2, MoveSpeed: 0.25, FollowRange: 16, XPReward: 5,
			Drops: []LootDrop{
				{ItemName: "minecraft:bone", MinCount: 0, MaxCount: 2, Chance: 1.0},
				{ItemName: "minecraft:arrow", MinCount: 0, MaxCount: 2, Chance: 1.0},
			}},
		{Name: "cave_spider", Category: MobCategoryHostile, MaxHealth: 12, AttackDmg: 2, MoveSpeed: 0.3, FollowRange: 16, XPReward: 5,
			Drops: []LootDrop{
				{ItemName: "minecraft:string", MinCount: 0, MaxCount: 2, Chance: 1.0},
				{ItemName: "minecraft:spider_eye", MinCount: 0, MaxCount: 1, Chance: 0.33},
			}},
		{Name: "witch", Category: MobCategoryHostile, MaxHealth: 26, AttackDmg: 0, MoveSpeed: 0.25, FollowRange: 16, XPReward: 5},
		{Name: "slime", Category: MobCategoryHostile, MaxHealth: 1, AttackDmg: 1, MoveSpeed: 0.3, FollowRange: 16, XPReward: 1,
			Drops: []LootDrop{{ItemName: "minecraft:slime_ball", MinCount: 0, MaxCount: 2, Chance: 1.0}}},
		{Name: "blaze", Category: MobCategoryHostile, MaxHealth: 20, AttackDmg: 6, MoveSpeed: 0.23, FollowRange: 48, XPReward: 10, FireImmune: true,
			Drops: []LootDrop{{ItemName: "minecraft:blaze_rod", MinCount: 0, MaxCount: 1, Chance: 1.0}}},
		{Name: "wither_skeleton", Category: MobCategoryHostile, MaxHealth: 20, AttackDmg: 4, MoveSpeed: 0.25, FollowRange: 16, XPReward: 5, FireImmune: true,
			Drops: []LootDrop{
				{ItemName: "minecraft:bone", MinCount: 0, MaxCount: 2, Chance: 1.0},
				{ItemName: "minecraft:coal", MinCount: 0, MaxCount: 1, Chance: 1.0},
			}},
		{Name: "phantom", Category: MobCategoryHostile, MaxHealth: 20, AttackDmg: 6, MoveSpeed: 0.1, FollowRange: 64, XPReward: 5,
			Drops: []LootDrop{{ItemName: "minecraft:phantom_membrane", MinCount: 0, MaxCount: 1, Chance: 1.0}}},
	}

	for i := range defs {
		info := genent.EntityByName(defs[i].Name)
		if info != nil {
			defs[i].ProtocolID = info.ID
			defs[i].Width = info.Width
			defs[i].Height = info.Height
		}
		RegisterMobTypeDef(&defs[i])
	}
}
