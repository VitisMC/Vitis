package projectile

import (
	"fmt"
	"math"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

// persistentBase snapshots projectile state without persisting process-local owner IDs.
func (b *Base) persistentBase(name string) (*nbt.Compound, error) {
	root, err := b.Entity.PersistentBaseNBT(name)
	if err != nil {
		return nil, err
	}
	if b.life < 0 || b.life > math.MaxInt16 {
		return nil, fmt.Errorf("projectile lifetime exceeds persistent range")
	}
	root.PutShort("life", int16(b.life)).PutBool("inGround", b.inGround)
	if b.ownerUUID != (protocol.UUID{}) {
		root.PutIntArray("Owner", entity.UUIDToIntArray(b.ownerUUID))
	} else {
		root.Delete("Owner")
	}
	return root, nil
}

// PersistentNBT captures an arrow's damage, pickup policy, and flight state.
func (a *Arrow) PersistentNBT() (*nbt.Compound, error) {
	root, err := a.persistentBase("minecraft:arrow")
	if err != nil {
		return nil, err
	}
	if math.IsNaN(a.damage) || math.IsInf(a.damage, 0) || a.damage < 0 || a.piercing < 0 || a.piercing > 127 || a.pickup < 0 || a.pickup > 2 || a.inGroundTicks < 0 || a.inGroundTicks > 1201 {
		return nil, fmt.Errorf("invalid arrow state")
	}
	pickup := []int8{1, 2, 0}[a.pickup]
	root.PutDouble("damage", a.damage).PutByte("PierceLevel", int8(a.piercing)).PutByte("pickup", pickup).PutBool("crit", a.critical).PutInt("vitis:knockback", a.knockback).PutInt("Color", a.potionColor)
	root.PutInt("vitis:in_ground_ticks", a.inGroundTicks)
	stack, err := inventory.ItemStackToNBT(a.pickupItem)
	if err != nil {
		return nil, err
	}
	root.PutCompound("item", stack)
	return root, nil
}

// PersistentNBT captures a thrown projectile's owner and associated item.
func (t *ThrownEntity) PersistentNBT() (*nbt.Compound, error) {
	name := thrownPersistentName(t.thrownType)
	if name == "" {
		return nil, fmt.Errorf("unsupported thrown projectile")
	}
	root, err := t.persistentBase(name)
	if err != nil {
		return nil, err
	}
	root.PutInt("vitis:item_id", t.itemID)
	return root, nil
}

// PersistentNBT captures a trident's return timer and enchantment state.
func (t *Trident) PersistentNBT() (*nbt.Compound, error) {
	root, err := t.persistentBase("minecraft:trident")
	if err != nil {
		return nil, err
	}
	if math.IsNaN(t.damage) || math.IsInf(t.damage, 0) || t.damage < 0 || t.loyalty < 0 || t.loyalty > 3 || t.returnTimer < 0 {
		return nil, fmt.Errorf("invalid trident state")
	}
	root.PutDouble("damage", t.damage).PutInt("vitis:loyalty", t.loyalty).PutBool("vitis:channeling", t.channeling).PutInt("vitis:return_timer", t.returnTimer)
	if _, ok := root.GetCompound("item"); !ok {
		stack, err := inventory.ItemStackToNBT(inventory.NewSlot(item.IDByName("minecraft:trident"), 1))
		if err != nil {
			return nil, err
		}
		root.PutCompound("item", stack)
	}
	return root, nil
}

// DecodePersistent restores a projectile without publishing it to runtime managers.
func DecodePersistent(id int32, root *nbt.Compound) (entity.PersistentEntity, error) {
	name, _ := root.GetString("id")
	var result entity.PersistentEntity
	var base *Base
	switch name {
	case "minecraft:arrow":
		a := NewArrow(id, protocol.UUID{}, entity.Vec3{}, entity.Vec3{}, -1, protocol.UUID{})
		if err := entity.ValidatePersistentTags(root, map[string]byte{"damage": nbt.TagDouble, "PierceLevel": nbt.TagByte, "pickup": nbt.TagByte, "crit": nbt.TagByte, "Color": nbt.TagInt, "vitis:knockback": nbt.TagInt, "vitis:in_ground_ticks": nbt.TagInt}); err != nil {
			return nil, err
		}
		if v, ok := root.GetDouble("damage"); ok {
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
				return nil, fmt.Errorf("invalid arrow damage")
			}
			a.damage = v
		}
		if v, ok := root.GetByte("pickup"); ok {
			if v < 0 || v > 2 {
				return nil, fmt.Errorf("invalid arrow pickup")
			}
			a.pickup = []int32{ArrowPickupDisallowed, ArrowPickupAllowed, ArrowPickupCreative}[v]
		}
		if v, ok := root.GetByte("PierceLevel"); ok {
			if v < 0 {
				return nil, fmt.Errorf("invalid arrow piercing")
			}
			a.piercing = int32(v)
		}
		a.critical, _ = root.GetBool("crit")
		a.inGroundTicks, _ = root.GetInt("vitis:in_ground_ticks")
		if a.inGroundTicks < 0 || a.inGroundTicks > 1201 {
			return nil, fmt.Errorf("invalid grounded arrow time")
		}
		a.knockback, _ = root.GetInt("vitis:knockback")
		result, base = a, a.Base
	case "minecraft:trident":
		t := NewTrident(id, protocol.UUID{}, entity.Vec3{}, entity.Vec3{}, -1, protocol.UUID{})
		if err := entity.ValidatePersistentTags(root, map[string]byte{"damage": nbt.TagDouble, "vitis:loyalty": nbt.TagInt, "vitis:channeling": nbt.TagByte, "vitis:return_timer": nbt.TagInt}); err != nil {
			return nil, err
		}
		if v, ok := root.GetDouble("damage"); ok {
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
				return nil, fmt.Errorf("invalid trident damage")
			}
			t.damage = v
		}
		t.loyalty, _ = root.GetInt("vitis:loyalty")
		t.channeling, _ = root.GetBool("vitis:channeling")
		t.returnTimer, _ = root.GetInt("vitis:return_timer")
		if t.loyalty < 0 || t.loyalty > 3 || t.returnTimer < 0 {
			return nil, fmt.Errorf("invalid trident return state")
		}
		result, base = t, t.Base
	default:
		kind, ok := map[string]ThrownType{"minecraft:snowball": ThrownSnowball, "minecraft:egg": ThrownEgg, "minecraft:ender_pearl": ThrownEnderPearl, "minecraft:potion": ThrownSplashPotion}[name]
		if !ok {
			return nil, nil
		}
		t := NewThrownEntity(id, protocol.UUID{}, kind, entity.Vec3{}, entity.Vec3{}, -1, protocol.UUID{})
		if err := entity.ValidatePersistentTags(root, map[string]byte{"vitis:item_id": nbt.TagInt}); err != nil {
			return nil, err
		}
		t.itemID, _ = root.GetInt("vitis:item_id")
		result, base = t, t.Base
	}
	if err := base.Entity.RestorePersistentBase(root); err != nil {
		return nil, err
	}
	if err := entity.ValidatePersistentTags(root, map[string]byte{"life": nbt.TagShort, "inGround": nbt.TagByte, "item": nbt.TagCompound}); err != nil {
		return nil, err
	}
	if raw, ok := root.GetCompound("item"); ok {
		stack, err := inventory.ItemStackFromNBT(raw)
		if err != nil {
			return nil, err
		}
		if arrow, ok := result.(*Arrow); ok {
			if err := arrow.SetPickupItem(stack); err != nil {
				return nil, err
			}
		}
	}
	if arrow, ok := result.(*Arrow); ok {
		if color, present := root.GetInt("Color"); present {
			arrow.potionColor = color
		}
	}
	life, _ := root.GetShort("life")
	if life < 0 {
		return nil, fmt.Errorf("invalid projectile lifetime")
	}
	base.life = int32(life)
	base.inGround, _ = root.GetBool("inGround")
	if _, _, present := root.Get("Owner"); present {
		uuid, err := entity.UUIDFromNBT(root, "Owner")
		if err != nil {
			return nil, err
		}
		base.ownerUUID = uuid
	}
	return result, nil
}

// ResolvePersistentOwners rebinds projectile ownership after entities or players load.
func (m *Manager) ResolvePersistentOwners(owners map[protocol.UUID]int32) {
	resolve := func(b *Base) {
		b.ownerID = -1
		if id, ok := owners[b.ownerUUID]; ok {
			b.ownerID = id
		}
	}
	for _, a := range m.arrows {
		resolve(a.Base)
	}
	for _, t := range m.thrown {
		resolve(t.Base)
	}
	for _, t := range m.tridents {
		resolve(t.Base)
	}
}

// thrownPersistentName resolves the persistent identifier of a thrown entity.
func thrownPersistentName(kind ThrownType) string {
	return map[ThrownType]string{ThrownSnowball: "minecraft:snowball", ThrownEgg: "minecraft:egg", ThrownEnderPearl: "minecraft:ender_pearl", ThrownSplashPotion: "minecraft:potion"}[kind]
}
