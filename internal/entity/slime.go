package entity

import (
	"fmt"
	"math/rand"

	genattr "github.com/vitismc/vitis/internal/data/generated/attribute"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

// SlimeSize returns the authoritative size used by physics, combat and metadata.
func (m *MobEntity) SlimeSize() int { return max(1, m.slimeSize) }

// SetSlimeSize updates a slime's instance properties on the world tick.
// Values are clamped to the vanilla range; other mob types are left unchanged.
func (m *MobEntity) SetSlimeSize(size int, resetHealth bool) {
	if m.TypeName() != "slime" {
		return
	}
	size = max(1, min(127, size))
	definition := *GetMobTypeDef("slime")
	definition.Width *= float64(size)
	definition.Height *= float64(size)
	definition.MaxHealth = float32(size * size)
	definition.MoveSpeed = float64(float32(.2) + float32(.1)*float32(size))
	definition.AttackDmg = float32(size)
	definition.XPReward = int32(size)
	m.typeDef = &definition
	m.slimeSize = size
	m.SetMaxHealth(definition.MaxHealth)
	m.attributes.SetBase(genattr.AttrMaxHealth, float64(definition.MaxHealth))
	m.attributes.SetBase(genattr.AttrMovementSpeed, definition.MoveSpeed)
	m.attributes.SetBase(genattr.AttrAttackDamage, float64(size))
	if resetHealth {
		m.SetHealth(definition.MaxHealth)
	}
	m.Entity.dirty |= DirtyLivingState
}

// restoreSlimeSize validates saved size before restoring health and attributes.
func (m *MobEntity) restoreSlimeSize(root *nbt.Compound) error {
	if m.TypeName() != "slime" {
		return nil
	}
	if err := ValidatePersistentTags(root, map[string]byte{"Size": nbt.TagInt}); err != nil {
		return err
	}
	saved, _ := root.GetInt("Size")
	if saved < 0 || saved > 126 {
		return fmt.Errorf("invalid saved slime size %d", saved)
	}
	m.SetSlimeSize(int(saved)+1, false)
	return nil
}

// splitSlime creates living children only after a completed death animation.
func (mm *MobManager) splitSlime(parent *MobEntity) {
	size := parent.SlimeSize() / 2
	count := 2 + rand.Intn(3)
	for index := 0; index < count; index++ {
		position := parent.Position()
		position.X += (float64(index%2) - .5) * parent.TypeDef().Width / 2
		position.Y += .5
		position.Z += (float64(index/2) - .5) * parent.TypeDef().Width / 2
		child := NewMobEntityByName(mm.nextEntityID(), protocol.GenerateUUID(), "slime", position)
		child.SetSlimeSize(size, true)
		child.SetRotation(Vec2{X: rand.Float32() * 360})
		child.SetNoAI(parent.NoAI())
		child.SetPersistenceRequired(parent.PersistenceRequired())
		child.SetAbsorption(parent.Absorption())
		for _, instance := range parent.Effects().Active() {
			child.Effects().Add(instance)
		}
		if parent.persistent != nil {
			inherited := nbt.NewCompound()
			for _, name := range []string{"CustomName", "CustomNameVisible", "Invulnerable", "NoGravity", "Silent", "Tags", "LeftHanded", "PortalCooldown"} {
				if value, tag, ok := parent.persistent.Get(name); ok {
					inherited.PutTag(name, tag, value)
				}
			}
			child.persistent = inherited
		}
		mm.Add(child)
	}
}
