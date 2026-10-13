package entity

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

// PersistentEntity exposes a detached snapshot of a world-owned runtime entity.
type PersistentEntity interface {
	BaseEntity() *Entity
	PersistentNBT() (*nbt.Compound, error)
}

// BaseEntity returns the common runtime entity.
func (e *Entity) BaseEntity() *Entity { return e }

// PersistentBaseNBT snapshots identity and movement while preserving unknown tags.
func (e *Entity) PersistentBaseNBT(name string) (*nbt.Compound, error) {
	if e == nil || !validVector(e.pos, 30000000) || !validVector(e.vel, 10) || !finite(float64(e.rot.X)) || !finite(float64(e.rot.Y)) {
		return nil, fmt.Errorf("invalid persistent entity movement")
	}
	root := e.persistent.Clone()
	if root == nil {
		root = nbt.NewCompound()
	}
	pos := nbt.NewList(nbt.TagDouble).Add(e.pos.X).Add(e.pos.Y).Add(e.pos.Z)
	motion := nbt.NewList(nbt.TagDouble).Add(e.vel.X).Add(e.vel.Y).Add(e.vel.Z)
	rot := nbt.NewList(nbt.TagFloat).Add(e.rot.X).Add(e.rot.Y)
	root.PutString("id", name).PutIntArray("UUID", UUIDToIntArray(e.uuid)).PutList("Pos", pos).PutList("Motion", motion).PutList("Rotation", rot).PutBool("OnGround", e.onGround)
	return root, nil
}

// RestorePersistentBase validates a snapshot before replacing common runtime state.
func (e *Entity) RestorePersistentBase(root *nbt.Compound) error {
	if root == nil {
		return fmt.Errorf("missing entity NBT")
	}
	uuid, err := UUIDFromNBT(root, "UUID")
	if err != nil {
		return err
	}
	position, err := persistentVector(root, "Pos", true)
	if err != nil || !validVector(position, 30000000) {
		return fmt.Errorf("invalid entity position")
	}
	velocity, err := persistentVector(root, "Motion", false)
	if err != nil {
		return err
	}
	if !validVector(velocity, 10) {
		return fmt.Errorf("entity motion exceeds persistent bounds")
	}
	rotation := Vec2{}
	if value, _, present := root.Get("Rotation"); present {
		list, ok := value.(*nbt.List)
		if !ok || list.ElementType() != nbt.TagFloat || list.Len() != 2 {
			return fmt.Errorf("invalid entity rotation")
		}
		x, xok := list.Elements()[0].(float32)
		y, yok := list.Elements()[1].(float32)
		if !xok || !yok || !finite(float64(x)) || !finite(float64(y)) {
			return fmt.Errorf("invalid entity rotation")
		}
		rotation = Vec2{x, y}
	}
	if err := ValidatePersistentTags(root, map[string]byte{"OnGround": nbt.TagByte}); err != nil {
		return err
	}
	e.uuid, e.pos, e.vel, e.rot = uuid, position, velocity, rotation
	e.onGround, _ = root.GetBool("OnGround")
	e.prevPos, e.prevRot = position, rotation
	e.chunkX, e.chunkZ = BlockToChunk(position.X), BlockToChunk(position.Z)
	e.persistent = root.Clone()
	return nil
}

// UUIDToIntArray encodes the standard four-int persistent UUID representation.
func UUIDToIntArray(uuid protocol.UUID) []int32 {
	return []int32{int32(uuid[0] >> 32), int32(uuid[0]), int32(uuid[1] >> 32), int32(uuid[1])}
}

// UUIDFromNBT reads an exact four-int persistent UUID.
func UUIDFromNBT(root *nbt.Compound, key string) (protocol.UUID, error) {
	value, tag, ok := root.Get(key)
	ints, typed := value.([]int32)
	if !ok || tag != nbt.TagIntArray || !typed || len(ints) != 4 {
		return protocol.UUID{}, fmt.Errorf("invalid entity %s", key)
	}
	return protocol.UUID{uint64(uint32(ints[0]))<<32 | uint64(uint32(ints[1])), uint64(uint32(ints[2]))<<32 | uint64(uint32(ints[3]))}, nil
}

// ValidatePersistentTags rejects present fields whose NBT types do not match the codec.
func ValidatePersistentTags(root *nbt.Compound, fields map[string]byte) error {
	for name, expected := range fields {
		if _, tag, present := root.Get(name); present && tag != expected {
			return fmt.Errorf("invalid entity %s type", name)
		}
	}
	return nil
}

// persistentVector decodes a finite three-double vector.
func persistentVector(root *nbt.Compound, key string, required bool) (Vec3, error) {
	value, _, present := root.Get(key)
	if !present && !required {
		return Vec3{}, nil
	}
	list, ok := value.(*nbt.List)
	if !ok || list.ElementType() != nbt.TagDouble || list.Len() != 3 {
		return Vec3{}, fmt.Errorf("invalid entity %s", key)
	}
	var result [3]float64
	for i, element := range list.Elements() {
		v, ok := element.(float64)
		if !ok || !finite(v) {
			return Vec3{}, fmt.Errorf("invalid entity %s", key)
		}
		result[i] = v
	}
	return Vec3{result[0], result[1], result[2]}, nil
}

// finite reports whether a floating-point value can be persisted safely.
func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

// validVector bounds persistent coordinates and rejects nonfinite components.
func validVector(v Vec3, limit float64) bool {
	return finite(v.X) && finite(v.Y) && finite(v.Z) && math.Abs(v.X) <= limit && math.Abs(v.Y) <= limit && math.Abs(v.Z) <= limit
}

// DecodePersistentEntity restores implemented entity types without registering any state.
// A nil entity with no error denotes an unsupported type that must remain opaque.
func DecodePersistentEntity(id int32, root *nbt.Compound) (PersistentEntity, error) {
	name, ok := root.GetString("id")
	if !ok || !strings.Contains(name, ":") {
		return nil, fmt.Errorf("invalid entity id")
	}
	base := NewEntity(id, protocol.UUID{}, EntityTypeMob, Vec3{}, Vec2{})
	if err := base.RestorePersistentBase(root); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(name, "minecraft:") {
		return nil, nil
	}
	var result PersistentEntity
	switch name {
	case "minecraft:item":
		if err := ValidatePersistentTags(root, map[string]byte{"Item": nbt.TagCompound, "Age": nbt.TagShort, "PickupDelay": nbt.TagShort, "Health": nbt.TagShort, "vitis:age": nbt.TagInt}); err != nil {
			return nil, err
		}
		raw, ok := root.GetCompound("Item")
		if !ok {
			return nil, fmt.Errorf("missing entity item stack")
		}
		stack, err := inventory.ItemStackFromNBT(raw)
		if err != nil {
			return nil, err
		}
		item := NewItemEntity(id, base.uuid, base.pos, stack)
		if age, ok := root.GetShort("Age"); ok {
			item.age = int32(age)
			item.noDespawn = age == -32768
			if item.noDespawn {
				item.age = 0
				if saved, ok := root.GetInt("vitis:age"); ok {
					if saved < 0 {
						return nil, fmt.Errorf("invalid persistent item age")
					}
					item.age = saved
				}
			}
		}
		if delay, ok := root.GetShort("PickupDelay"); ok {
			item.pickupDelay = int32(delay)
		}
		if health, ok := root.GetShort("Health"); ok {
			item.health = float32(health)
		}
		if _, _, ok := root.Get("Thrower"); ok {
			item.thrower, err = UUIDFromNBT(root, "Thrower")
			if err != nil {
				return nil, err
			}
		}
		result = item
	case "minecraft:experience_orb":
		if err := ValidatePersistentTags(root, map[string]byte{"Age": nbt.TagShort, "Value": nbt.TagShort, "Health": nbt.TagShort, "Count": nbt.TagInt}); err != nil {
			return nil, err
		}
		if count, ok := root.GetInt("Count"); ok {
			if count < 1 {
				return nil, fmt.Errorf("invalid experience orb count")
			}
		}
		value, ok := root.GetShort("Value")
		if !ok || value <= 0 {
			return nil, fmt.Errorf("invalid experience orb value")
		}
		orb := NewXPOrb(id, base.uuid, base.pos, int32(value))
		if count, ok := root.GetInt("Count"); ok {
			orb.count = count
		}
		if health, ok := root.GetShort("Health"); ok {
			if health < 0 {
				return nil, fmt.Errorf("invalid experience orb health")
			}
			orb.health = int32(health)
		}
		age, _ := root.GetShort("Age")
		if age < 0 {
			return nil, fmt.Errorf("invalid experience orb age")
		}
		orb.age = int32(age)
		result = orb
	case "minecraft:tnt":
		if err := ValidatePersistentTags(root, map[string]byte{"fuse": nbt.TagShort, "explosion_power": nbt.TagFloat}); err != nil {
			return nil, err
		}
		fuse, _ := root.GetShort("fuse")
		tnt := NewTNTEntityWithFuse(id, base.uuid, base.pos, -1, int32(fuse))
		if _, _, present := root.Get("vitis:owner"); present {
			uuid, err := UUIDFromNBT(root, "vitis:owner")
			if err != nil {
				return nil, err
			}
			tnt.ownerUUID = uuid
		}
		if power, ok := root.GetFloat("explosion_power"); ok {
			if !finite(float64(power)) || power < 0 || power > 128 {
				return nil, fmt.Errorf("invalid TNT power")
			}
			tnt.power = float64(power)
		}
		result = tnt
	case "minecraft:falling_block":
		if err := ValidatePersistentTags(root, map[string]byte{"BlockState": nbt.TagCompound, "Time": nbt.TagInt}); err != nil {
			return nil, err
		}
		raw, ok := root.GetCompound("BlockState")
		if !ok {
			return nil, fmt.Errorf("missing falling block state")
		}
		state, err := DecodePersistentBlockState(raw)
		if err != nil {
			return nil, err
		}
		falling := NewFallingBlock(id, base.uuid, base.pos, state)
		time, _ := root.GetInt("Time")
		if time < 0 {
			return nil, fmt.Errorf("invalid falling block time")
		}
		falling.ticksExisted = int(time)
		result = falling
	default:
		mob := NewMobEntityByName(id, base.uuid, strings.TrimPrefix(name, "minecraft:"), base.pos)
		if mob == nil {
			return nil, nil
		}
		if err := mob.restoreSlimeSize(root); err != nil {
			return nil, err
		}
		if err := mob.restoreLivingNBT(root); err != nil {
			return nil, err
		}
		if err := ValidatePersistentTags(root, map[string]byte{"NoAI": nbt.TagByte, "vitis:age_ticks": nbt.TagLong}); err != nil {
			return nil, err
		}
		mob.noAI, _ = root.GetBool("NoAI")
		mob.ageTicks, _ = root.GetLong("vitis:age_ticks")
		result = mob
	}
	if err := result.BaseEntity().RestorePersistentBase(root); err != nil {
		return nil, err
	}
	return result, nil
}

// PersistentNBT captures a dropped item's full stack and lifetime.
func (i *ItemEntity) PersistentNBT() (*nbt.Compound, error) {
	root, err := i.PersistentBaseNBT("minecraft:item")
	if err != nil {
		return nil, err
	}
	stack, err := inventory.ItemStackToNBT(i.stack)
	if err != nil {
		return nil, err
	}
	if stack == nil || i.stack.Empty() {
		return nil, fmt.Errorf("cannot persist empty item entity")
	}
	age := i.age
	if i.noDespawn {
		if i.age < 0 {
			return nil, fmt.Errorf("invalid persistent item age")
		}
		age = -32768
		root.PutInt("vitis:age", i.age)
	} else {
		root.Delete("vitis:age")
	}
	if age < math.MinInt16 || age > math.MaxInt16 || i.pickupDelay < math.MinInt16 || i.pickupDelay > math.MaxInt16 || !finite(float64(i.health)) || i.health < 0 || i.health > math.MaxInt16 {
		return nil, fmt.Errorf("invalid item lifetime")
	}
	root.PutCompound("Item", stack).PutShort("Age", int16(age)).PutShort("PickupDelay", int16(i.pickupDelay)).PutShort("Health", int16(i.health))
	if i.thrower != (protocol.UUID{}) {
		root.PutIntArray("Thrower", UUIDToIntArray(i.thrower))
	} else {
		root.Delete("Thrower")
	}
	return root, nil
}

// PersistentNBT captures an experience orb's value and lifetime.
func (x *XPOrb) PersistentNBT() (*nbt.Compound, error) {
	root, err := x.PersistentBaseNBT("minecraft:experience_orb")
	if err != nil {
		return nil, err
	}
	if x.xpValue <= 0 || x.xpValue > math.MaxInt16 || x.age < 0 || x.age > math.MaxInt16 || x.count <= 0 || x.health < 0 || x.health > math.MaxInt16 {
		return nil, fmt.Errorf("invalid experience orb state")
	}
	root.PutShort("Value", int16(x.xpValue)).PutShort("Age", int16(x.age))
	root.PutInt("Count", x.count).PutShort("Health", int16(x.health))
	return root, nil
}

// PersistentNBT captures primed TNT's fuse and explosion power.
func (t *TNTEntity) PersistentNBT() (*nbt.Compound, error) {
	root, err := t.PersistentBaseNBT("minecraft:tnt")
	if err != nil {
		return nil, err
	}
	if t.fuse < math.MinInt16 || t.fuse > math.MaxInt16 || !finite(t.power) || t.power < 0 || t.power > 128 {
		return nil, fmt.Errorf("invalid TNT state")
	}
	root.PutShort("fuse", int16(t.fuse)).PutFloat("explosion_power", float32(t.power))
	if t.ownerUUID != (protocol.UUID{}) {
		root.PutIntArray("vitis:owner", UUIDToIntArray(t.ownerUUID))
	} else {
		root.Delete("vitis:owner")
	}
	return root, nil
}

// PersistentNBT captures a falling block's complete block state and lifetime.
func (f *FallingBlock) PersistentNBT() (*nbt.Compound, error) {
	root, err := f.PersistentBaseNBT("minecraft:falling_block")
	if err != nil {
		return nil, err
	}
	state, err := EncodePersistentBlockState(f.blockStateID)
	if err != nil {
		return nil, err
	}
	if previous, ok := root.GetCompound("BlockState"); ok {
		preserved := previous.Clone()
		for _, key := range state.Keys() {
			value, tag, _ := state.Get(key)
			preserved.PutTag(key, tag, value)
		}
		if _, _, present := state.Get("Properties"); !present {
			preserved.Delete("Properties")
		}
		state = preserved
	}
	if f.ticksExisted < 0 || f.ticksExisted > math.MaxInt32 {
		return nil, fmt.Errorf("invalid falling block lifetime")
	}
	root.PutCompound("BlockState", state).PutInt("Time", int32(f.ticksExisted))
	return root, nil
}

// EncodePersistentBlockState encodes a canonical block name and sorted properties.
func EncodePersistentBlockState(id int32) (*nbt.Compound, error) {
	state, err := block.BlockStateFromID(id)
	if err != nil {
		return nil, err
	}
	root := nbt.NewCompound().PutString("Name", state.Name())
	properties := state.Properties()
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) > 0 {
		raw := nbt.NewCompound()
		for _, name := range names {
			raw.PutString(name, properties[name])
		}
		root.PutCompound("Properties", raw)
	}
	return root, nil
}

// DecodePersistentBlockState resolves a persistent block state with validated properties.
func DecodePersistentBlockState(root *nbt.Compound) (int32, error) {
	name, ok := root.GetString("Name")
	if !ok {
		return 0, fmt.Errorf("missing block state name")
	}
	if err := ValidatePersistentTags(root, map[string]byte{"Properties": nbt.TagCompound}); err != nil {
		return 0, err
	}
	properties := make(map[string]string)
	if raw, ok := root.GetCompound("Properties"); ok {
		for _, key := range raw.Keys() {
			value, ok := raw.GetString(key)
			if !ok {
				return 0, fmt.Errorf("invalid block property %s", key)
			}
			properties[key] = value
		}
	}
	state, err := block.BlockStateFromProperties(name, properties)
	if err != nil {
		return 0, err
	}
	return state.ID(), nil
}
