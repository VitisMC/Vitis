package entity

import (
	"math"

	genattr "github.com/vitismc/vitis/internal/data/generated/attribute"
	genent "github.com/vitismc/vitis/internal/data/generated/entity"
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
	Drops        []LootDrop
}

// MobEntity extends LivingEntity with mob-specific state and AI support.
type MobEntity struct {
	*LivingEntity

	typeDef  *MobTypeDef
	noAI     bool
	target   int32
	ageTicks int64
}

// NewMobEntity creates a mob entity from a type definition.
func NewMobEntity(id int32, uuid protocol.UUID, def *MobTypeDef, pos Vec3, rot Vec2) *MobEntity {
	base := NewEntity(id, uuid, EntityTypeMob, pos, rot)
	base.SetProtocolInfo(def.ProtocolID, 0)
	le := NewLivingEntity(base, def.MaxHealth)
	le.attributes = mobAttributes(def)

	return &MobEntity{
		LivingEntity: le,
		typeDef:      def,
		target:       -1,
	}
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

// TypeDef returns the mob's static type definition.
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
func (m *MobEntity) SetNoAI(v bool) { m.noAI = v }

// Target returns the entity ID of the current attack target, or -1.
func (m *MobEntity) Target() int32 { return m.target }

// SetTarget sets the current attack target entity ID.
func (m *MobEntity) SetTarget(id int32) { m.target = id }

// AgeTicks returns how many ticks this mob has existed.
func (m *MobEntity) AgeTicks() int64 { return m.ageTicks }

// XPReward returns the XP dropped on death.
func (m *MobEntity) XPReward() int32 { return m.typeDef.XPReward }

// Living returns the underlying LivingEntity.
func (m *MobEntity) Living() *LivingEntity { return m.LivingEntity }

// TickMob advances mob state for one tick.
func (m *MobEntity) TickMob() {
	m.LivingEntity.TickLiving()
	m.ageTicks++

	if m.IsDead() && m.DeathTime() >= 20 {
		m.Entity.Remove()
	}
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
	ac.attrs[genattr.AttrGravity] = &Attribute{Base: 0.08}
	ac.attrs[genattr.AttrStepHeight] = &Attribute{Base: 0.6}
	return ac
}

var mobTypeDefs = map[string]*MobTypeDef{}

// RegisterMobTypeDef registers a mob type definition by name.
func RegisterMobTypeDef(def *MobTypeDef) {
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
		{Name: "slime", Category: MobCategoryHostile, MaxHealth: 16, AttackDmg: 4, MoveSpeed: 0.2, FollowRange: 16, XPReward: 4,
			Drops: []LootDrop{{ItemName: "minecraft:slime_ball", MinCount: 0, MaxCount: 2, Chance: 1.0}}},
		{Name: "blaze", Category: MobCategoryHostile, MaxHealth: 20, AttackDmg: 6, MoveSpeed: 0.23, FollowRange: 48, XPReward: 10,
			Drops: []LootDrop{{ItemName: "minecraft:blaze_rod", MinCount: 0, MaxCount: 1, Chance: 1.0}}},
		{Name: "wither_skeleton", Category: MobCategoryHostile, MaxHealth: 20, AttackDmg: 8, MoveSpeed: 0.25, FollowRange: 16, XPReward: 5,
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
