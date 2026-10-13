package inventory

import (
	"bytes"
	"fmt"
	"maps"
	"math"
	"reflect"
	"slices"

	datacomponent "github.com/vitismc/vitis/internal/data/generated/data_component"
	itemdata "github.com/vitismc/vitis/internal/data/generated/item"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

const (
	maxComponentPatchEntries = datacomponent.DataComponentCount
	maxComponentNBTBytes     = 64 << 10
	maxComponentMapEntries   = 256
	maxNestedItemStackDepth  = 8
)

// ComponentValue is a typed value carried by a data-component patch entry.
type ComponentValue interface {
	isComponentValue()
}

// VarIntComponent is a VarInt-encoded component value.
type VarIntComponent int32

func (VarIntComponent) isComponentValue() {}

// IntComponent is a fixed-width signed integer component value.
type IntComponent int32

func (IntComponent) isComponentValue() {}

// BoolComponent is a boolean component value.
type BoolComponent bool

func (BoolComponent) isComponentValue() {}

// UnitComponent is a component with no payload.
type UnitComponent struct{}

func (UnitComponent) isComponentValue() {}

// ResourceLocationComponent is a namespaced identifier component value.
type ResourceLocationComponent string

func (ResourceLocationComponent) isComponentValue() {}

// NBTComponent is a network-NBT compound component value.
type NBTComponent struct {
	Value *nbt.Compound
}

func (NBTComponent) isComponentValue() {}

// RawNBTComponent is one arbitrary unnamed network NBT value.
type RawNBTComponent struct {
	Data []byte
}

func (RawNBTComponent) isComponentValue() {}

// LoreComponent contains bounded serialized chat components.
type LoreComponent []RawNBTComponent

func (LoreComponent) isComponentValue() {}

// DyedColorComponent contains an RGB value and tooltip visibility.
type DyedColorComponent struct {
	RGB           int32
	ShowInTooltip bool
}

func (DyedColorComponent) isComponentValue() {}

// EnchantmentsComponent contains registry IDs, levels, and tooltip visibility.
type EnchantmentsComponent struct {
	Levels        map[int32]int32
	ShowInTooltip bool
}

func (EnchantmentsComponent) isComponentValue() {}

// StringMapComponent is a bounded string-to-string component value.
type StringMapComponent map[string]string

func (StringMapComponent) isComponentValue() {}

// CustomModelDataComponent contains model selector vectors.
type CustomModelDataComponent struct {
	Floats  []float32
	Flags   []bool
	Strings []string
	Colors  []int32
}

func (CustomModelDataComponent) isComponentValue() {}

// FoodComponent contains the network-synchronized food properties.
type FoodComponent struct {
	Nutrition    int32
	Saturation   float32
	CanAlwaysEat bool
}

func (FoodComponent) isComponentValue() {}

// UseRemainderComponent contains the stack left after the item is consumed.
type UseRemainderComponent struct {
	Stack ItemStack
}

func (UseRemainderComponent) isComponentValue() {}

// PotDecorationsComponent contains up to four registry item IDs.
type PotDecorationsComponent []int32

func (PotDecorationsComponent) isComponentValue() {}

// UseCooldownComponent contains cooldown duration and its optional group.
type UseCooldownComponent struct {
	Seconds  float32
	HasGroup bool
	Group    string
}

func (UseCooldownComponent) isComponentValue() {}

// FireworkExplosionComponent contains one firework explosion definition.
type FireworkExplosionComponent struct {
	Shape      int32
	Colors     []int32
	FadeColors []int32
	Trail      bool
	Twinkle    bool
}

func (FireworkExplosionComponent) isComponentValue() {}

// FireworksComponent contains rocket flight duration and explosions.
type FireworksComponent struct {
	FlightDuration int32
	Explosions     []FireworkExplosionComponent
}

func (FireworksComponent) isComponentValue() {}

// DataComponent is one added component and its original wire representation.
type DataComponent struct {
	TypeID int32
	Value  ComponentValue
	Raw    []byte
}

// ComponentPatch contains added and removed item data components.
type ComponentPatch struct {
	Added   []DataComponent
	Removed []int32
}

func (s ItemStack) componentPatchForEncoding() ComponentPatch {
	patch := ComponentPatch{
		Added:   append([]DataComponent(nil), s.ComponentPatch.Added...),
		Removed: append([]int32(nil), s.ComponentPatch.Removed...),
	}
	damageFound := false
	enchantmentsFound := false
	for i := range patch.Added {
		switch patch.Added[i].TypeID {
		case datacomponent.ComponentDamage:
			damageFound = true
		case datacomponent.ComponentEnchantments:
			enchantmentsFound = true
		}
	}
	if !damageFound && s.Damage != 0 {
		patch.Added = append(patch.Added, DataComponent{TypeID: datacomponent.ComponentDamage, Value: VarIntComponent(s.Damage)})
	}
	if !enchantmentsFound && s.Enchantments != nil {
		patch.Added = append(patch.Added, DataComponent{
			TypeID: datacomponent.ComponentEnchantments,
			Value:  EnchantmentsComponent{Levels: maps.Clone(s.Enchantments), ShowInTooltip: true},
		})
	}
	if (s.Damage != 0 && !damageFound) || (s.Enchantments != nil && !enchantmentsFound) {
		removed := patch.Removed[:0]
		for _, typeID := range patch.Removed {
			if (typeID == datacomponent.ComponentDamage && s.Damage != 0) ||
				(typeID == datacomponent.ComponentEnchantments && s.Enchantments != nil) {
				continue
			}
			removed = append(removed, typeID)
		}
		patch.Removed = removed
	}
	return patch
}

// Clone returns an independent component patch.
func (p ComponentPatch) Clone() ComponentPatch {
	clone := ComponentPatch{
		Added:   make([]DataComponent, len(p.Added)),
		Removed: append([]int32(nil), p.Removed...),
	}
	for i, component := range p.Added {
		clone.Added[i] = component
		clone.Added[i].Raw = bytes.Clone(component.Raw)
		clone.Added[i].Value = cloneComponentValue(component.Value)
	}
	return clone
}

// Equal reports whether two patches have identical semantics and wire values.
func (p ComponentPatch) Equal(other ComponentPatch) bool {
	if len(p.Added) != len(other.Added) || len(p.Removed) != len(other.Removed) {
		return false
	}
	for _, entry := range p.Added {
		found := false
		for _, candidate := range other.Added {
			if entry.TypeID == candidate.TypeID {
				if !componentValuesEqual(entry.Value, candidate.Value) {
					return false
				}
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, id := range p.Removed {
		if !slices.Contains(other.Removed, id) {
			return false
		}
	}
	return true
}

// componentValuesEqual ignores incidental ordering and original wire encodings.
func componentValuesEqual(a, b ComponentValue) bool {
	if av, ok := a.(NBTComponent); ok {
		bv, ok := b.(NBTComponent)
		return ok && nbtValueEqual(av.Value, bv.Value)
	}
	switch av := a.(type) {
	case StructuredComponent:
		bv, ok := b.(StructuredComponent)
		return ok && structuredValuesEqual(av.Data, bv.Data)
	case RawNBTComponent:
		bv, ok := b.(RawNBTComponent)
		if !ok {
			return false
		}
		ta, va, ea := nbt.NewDecoder(av.Data).ReadRootTag()
		tb, vb, eb := nbt.NewDecoder(bv.Data).ReadRootTag()
		return ea == nil && eb == nil && ta == tb && nbtValueEqual(va, vb)
	case LoreComponent:
		bv, ok := b.(LoreComponent)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !componentValuesEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	case UseRemainderComponent:
		bv, ok := b.(UseRemainderComponent)
		return ok && av.Stack.ItemCount == bv.Stack.ItemCount && av.Stack.StackableWith(bv.Stack)
	}
	return reflect.DeepEqual(a, b)
}

func cloneComponentValue(value ComponentValue) ComponentValue {
	switch typed := value.(type) {
	case StructuredComponent:
		return StructuredComponent{Data: cloneStructuredValue(typed.Data)}
	case NBTComponent:
		return NBTComponent{Value: typed.Value.Clone()}
	case RawNBTComponent:
		typed.Data = bytes.Clone(typed.Data)
		return typed
	case LoreComponent:
		clone := make(LoreComponent, len(typed))
		for i := range typed {
			clone[i] = RawNBTComponent{Data: bytes.Clone(typed[i].Data)}
		}
		return clone
	case EnchantmentsComponent:
		levels := make(map[int32]int32, len(typed.Levels))
		for id, level := range typed.Levels {
			levels[id] = level
		}
		typed.Levels = levels
		return typed
	case UseRemainderComponent:
		typed.Stack = typed.Stack.Clone()
		return typed
	case PotDecorationsComponent:
		return append(PotDecorationsComponent(nil), typed...)
	case StringMapComponent:
		clone := make(StringMapComponent, len(typed))
		for key, entry := range typed {
			clone[key] = entry
		}
		return clone
	case CustomModelDataComponent:
		typed.Floats = append([]float32(nil), typed.Floats...)
		typed.Flags = append([]bool(nil), typed.Flags...)
		typed.Strings = append([]string(nil), typed.Strings...)
		typed.Colors = append([]int32(nil), typed.Colors...)
		return typed
	case FireworkExplosionComponent:
		typed.Colors = append([]int32(nil), typed.Colors...)
		typed.FadeColors = append([]int32(nil), typed.FadeColors...)
		return typed
	case FireworksComponent:
		typed.Explosions = append([]FireworkExplosionComponent(nil), typed.Explosions...)
		for i := range typed.Explosions {
			typed.Explosions[i] = cloneComponentValue(typed.Explosions[i]).(FireworkExplosionComponent)
		}
		return typed
	default:
		return typed
	}
}

// DecodeItemStack decodes a bounded 1.21.4 optional item stack.
func DecodeItemStack(buf *protocol.Buffer) (ItemStack, error) {
	return decodeItemStack(buf, 0)
}

// decodeItemStack rejects recursive component graphs before they can exhaust the stack.
func decodeItemStack(buf *protocol.Buffer, depth int) (ItemStack, error) {
	if depth > maxNestedItemStackDepth {
		return ItemStack{}, fmt.Errorf("nested item stack depth exceeds %d", maxNestedItemStackDepth)
	}
	var stack ItemStack
	count, err := buf.ReadVarInt()
	if err != nil {
		return stack, err
	}
	if count < 0 {
		return stack, fmt.Errorf("negative item count: %d", count)
	}
	if count == 0 {
		return EmptySlot(), nil
	}
	if count > 99 {
		return stack, fmt.Errorf("item count out of range: %d", count)
	}
	itemID, err := buf.ReadVarInt()
	if err != nil {
		return stack, err
	}
	if itemID <= 0 || itemID >= itemdata.ItemCount {
		return stack, fmt.Errorf("item ID out of range: %d", itemID)
	}

	patchStart := buf.ReadIndex()
	patch, err := decodeComponentPatch(buf, depth)
	if err != nil {
		return ItemStack{}, err
	}
	patchEnd := buf.ReadIndex()
	stack = ItemStack{
		ItemCount:      count,
		ItemID:         itemID,
		ComponentsAdd:  int32(len(patch.Added)),
		ComponentsRem:  int32(len(patch.Removed)),
		RawComponents:  bytes.Clone(buf.Bytes()[patchStart:patchEnd]),
		ComponentPatch: patch,
	}
	for _, component := range patch.Added {
		switch component.TypeID {
		case datacomponent.ComponentDamage:
			stack.Damage = int32(component.Value.(VarIntComponent))
		case datacomponent.ComponentEnchantments:
			stack.Enchantments = cloneComponentValue(component.Value).(EnchantmentsComponent).Levels
		}
	}
	if count > stack.MaxStackSize() {
		return ItemStack{}, fmt.Errorf("count exceeds effective stack size")
	}
	return stack, nil
}

// DecodeComponentPatch decodes and validates one data-component patch.
func DecodeComponentPatch(buf *protocol.Buffer) (ComponentPatch, error) {
	return decodeComponentPatch(buf, 0)
}

// decodeComponentPatch decodes a patch within a bounded item-stack graph.
func decodeComponentPatch(buf *protocol.Buffer, depth int) (ComponentPatch, error) {
	addedCount, err := buf.ReadVarInt()
	if err != nil {
		return ComponentPatch{}, err
	}
	removedCount, err := buf.ReadVarInt()
	if err != nil {
		return ComponentPatch{}, err
	}
	if addedCount < 0 || removedCount < 0 || addedCount > maxComponentPatchEntries || removedCount > maxComponentPatchEntries || addedCount+removedCount > maxComponentPatchEntries {
		return ComponentPatch{}, fmt.Errorf("component patch counts out of range: added=%d removed=%d", addedCount, removedCount)
	}

	patch := ComponentPatch{
		Added:   make([]DataComponent, 0, addedCount),
		Removed: make([]int32, 0, removedCount),
	}
	seen := make(map[int32]struct{}, addedCount+removedCount)
	for i := int32(0); i < addedCount; i++ {
		typeID, readErr := readComponentType(buf, seen)
		if readErr != nil {
			return ComponentPatch{}, fmt.Errorf("added component %d: %w", i, readErr)
		}
		start := buf.ReadIndex()
		value, readErr := decodeComponentValue(buf, typeID, depth)
		if readErr != nil {
			return ComponentPatch{}, fmt.Errorf("component %d (%s): %w", typeID, datacomponent.DataComponentNameByID(typeID), readErr)
		}
		patch.Added = append(patch.Added, DataComponent{
			TypeID: typeID,
			Value:  value,
			Raw:    bytes.Clone(buf.Bytes()[start:buf.ReadIndex()]),
		})
	}
	for i := int32(0); i < removedCount; i++ {
		typeID, readErr := readComponentType(buf, seen)
		if readErr != nil {
			return ComponentPatch{}, fmt.Errorf("removed component %d: %w", i, readErr)
		}
		patch.Removed = append(patch.Removed, typeID)
	}
	return patch, nil
}

func readComponentType(buf *protocol.Buffer, seen map[int32]struct{}) (int32, error) {
	typeID, err := buf.ReadVarInt()
	if err != nil {
		return 0, err
	}
	if typeID < 0 || typeID >= datacomponent.DataComponentCount {
		return 0, fmt.Errorf("component type ID out of range: %d", typeID)
	}
	if _, duplicate := seen[typeID]; duplicate {
		return 0, fmt.Errorf("duplicate component type ID: %d", typeID)
	}
	seen[typeID] = struct{}{}
	return typeID, nil
}

func decodeComponentValue(buf *protocol.Buffer, typeID int32, depth int) (ComponentValue, error) {
	switch typeID {
	case datacomponent.ComponentCustomData, datacomponent.ComponentEntityData,
		datacomponent.ComponentBucketEntityData, datacomponent.ComponentBlockEntityData:
		remaining := buf.RemainingBytes()
		decoder := componentNBTDecoder(remaining)
		compound, err := decoder.ReadRootCompound()
		if err != nil {
			return nil, err
		}
		if decoder.Pos() > maxComponentNBTBytes {
			return nil, fmt.Errorf("component NBT exceeds %d bytes", maxComponentNBTBytes)
		}
		if _, err := buf.ReadBytes(decoder.Pos()); err != nil {
			return nil, err
		}
		return NBTComponent{Value: compound}, nil
	case datacomponent.ComponentMaxStackSize, datacomponent.ComponentMaxDamage,
		datacomponent.ComponentDamage, datacomponent.ComponentRepairCost,
		datacomponent.ComponentEnchantable, datacomponent.ComponentMapId,
		datacomponent.ComponentOminousBottleAmplifier, datacomponent.ComponentRarity,
		datacomponent.ComponentMapPostProcessing,
		datacomponent.ComponentBaseColor:
		value, err := buf.ReadVarInt()
		if err != nil {
			return nil, err
		}
		if err := validateVarIntComponent(typeID, value); err != nil {
			return nil, err
		}
		return VarIntComponent(value), nil
	case datacomponent.ComponentUnbreakable, datacomponent.ComponentEnchantmentGlintOverride:
		value, err := buf.ReadBool()
		return BoolComponent(value), err
	case datacomponent.ComponentHideAdditionalTooltip, datacomponent.ComponentHideTooltip,
		datacomponent.ComponentCreativeSlotLock, datacomponent.ComponentGlider:
		return UnitComponent{}, nil
	case datacomponent.ComponentItemModel, datacomponent.ComponentTooltipStyle, datacomponent.ComponentNoteBlockSound:
		value, err := buf.ReadStringLimit(32767)
		return ResourceLocationComponent(value), err
	case datacomponent.ComponentEnchantments, datacomponent.ComponentStoredEnchantments:
		return decodeEnchantments(buf)
	case datacomponent.ComponentCustomName, datacomponent.ComponentItemName:
		return decodeRawNBT(buf)
	case datacomponent.ComponentLore:
		count, err := readBoundedListCount(buf)
		if err != nil {
			return nil, err
		}
		lore := make(LoreComponent, count)
		for i := range lore {
			value, readErr := decodeRawNBT(buf)
			if readErr != nil {
				return nil, fmt.Errorf("lore line %d: %w", i, readErr)
			}
			lore[i] = value.(RawNBTComponent)
		}
		return lore, nil
	case datacomponent.ComponentDyedColor:
		rgb, err := buf.ReadInt32()
		if err != nil {
			return nil, err
		}
		show, err := buf.ReadBool()
		return DyedColorComponent{RGB: rgb, ShowInTooltip: show}, err
	case datacomponent.ComponentMapColor:
		value, err := buf.ReadInt32()
		return IntComponent(value), err
	case datacomponent.ComponentBlockState:
		return decodeStringMap(buf)
	case datacomponent.ComponentCustomModelData:
		return decodeCustomModelData(buf)
	case datacomponent.ComponentFood:
		nutrition, err := buf.ReadVarInt()
		if err != nil {
			return nil, err
		}
		saturation, err := buf.ReadFloat32()
		if err != nil {
			return nil, err
		}
		alwaysEat, err := buf.ReadBool()
		if nutrition < 0 || !finiteFloat(saturation) || saturation < 0 {
			return nil, fmt.Errorf("invalid food properties")
		}
		return FoodComponent{Nutrition: nutrition, Saturation: saturation, CanAlwaysEat: alwaysEat}, err
	case datacomponent.ComponentUseRemainder:
		stack, err := decodeItemStack(buf, depth+1)
		if err != nil {
			return nil, err
		}
		if stack.Empty() {
			return nil, fmt.Errorf("use remainder must contain an item")
		}
		return UseRemainderComponent{Stack: stack}, nil
	case datacomponent.ComponentPotDecorations:
		count, err := buf.ReadVarInt()
		if err != nil {
			return nil, err
		}
		if count < 0 || count > 4 {
			return nil, fmt.Errorf("pot decorations count out of range: %d", count)
		}
		items := make(PotDecorationsComponent, count)
		for i := range items {
			items[i], err = buf.ReadVarInt()
			if err != nil {
				return nil, err
			}
			if items[i] < 0 || items[i] >= itemdata.ItemCount {
				return nil, fmt.Errorf("pot decoration item ID out of range: %d", items[i])
			}
		}
		return items, nil
	case datacomponent.ComponentUseCooldown:
		seconds, err := buf.ReadFloat32()
		if err != nil {
			return nil, err
		}
		if !finiteFloat(seconds) || seconds < 0 {
			return nil, fmt.Errorf("invalid cooldown duration: %v", seconds)
		}
		hasGroup, err := buf.ReadBool()
		if err != nil {
			return nil, err
		}
		value := UseCooldownComponent{Seconds: seconds, HasGroup: hasGroup}
		if hasGroup {
			value.Group, err = buf.ReadStringLimit(32767)
		}
		return value, err
	case datacomponent.ComponentFireworkExplosion:
		return decodeFireworkExplosion(buf)
	case datacomponent.ComponentFireworks:
		flight, err := buf.ReadVarInt()
		if err != nil || flight < 0 || flight > 255 {
			return nil, fmt.Errorf("invalid firework flight duration: %d", flight)
		}
		count, err := readBoundedListCount(buf)
		if err != nil {
			return nil, err
		}
		value := FireworksComponent{FlightDuration: flight, Explosions: make([]FireworkExplosionComponent, count)}
		for i := range value.Explosions {
			explosion, readErr := decodeFireworkExplosion(buf)
			if readErr != nil {
				return nil, fmt.Errorf("firework explosion %d: %w", i, readErr)
			}
			value.Explosions[i] = explosion.(FireworkExplosionComponent)
		}
		return value, nil
	default:
		return decodeStructuredComponent(buf, typeID, depth)
	}
}

func decodeRawNBT(buf *protocol.Buffer) (ComponentValue, error) {
	decoder := componentNBTDecoder(buf.RemainingBytes())
	if _, _, err := decoder.ReadRootTag(); err != nil {
		return nil, err
	}
	if decoder.Pos() <= 0 || decoder.Pos() > maxComponentNBTBytes {
		return nil, fmt.Errorf("component NBT size out of range: %d", decoder.Pos())
	}
	raw, err := buf.ReadBytes(decoder.Pos())
	if err != nil {
		return nil, err
	}
	return RawNBTComponent{Data: bytes.Clone(raw)}, nil
}

func decodeFireworkExplosion(buf *protocol.Buffer) (ComponentValue, error) {
	shape, err := buf.ReadVarInt()
	if err != nil || shape < 0 || shape > 4 {
		return nil, fmt.Errorf("invalid firework shape: %d", shape)
	}
	colors, err := decodeIntList(buf)
	if err != nil {
		return nil, err
	}
	fadeColors, err := decodeIntList(buf)
	if err != nil {
		return nil, err
	}
	trail, err := buf.ReadBool()
	if err != nil {
		return nil, err
	}
	twinkle, err := buf.ReadBool()
	return FireworkExplosionComponent{Shape: shape, Colors: colors, FadeColors: fadeColors, Trail: trail, Twinkle: twinkle}, err
}

func decodeIntList(buf *protocol.Buffer) ([]int32, error) {
	count, err := readBoundedListCount(buf)
	if err != nil {
		return nil, err
	}
	values := make([]int32, count)
	for i := range values {
		values[i], err = buf.ReadInt32()
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}

func validateVarIntComponent(typeID, value int32) error {
	switch typeID {
	case datacomponent.ComponentMaxStackSize:
		if value < 1 || value > 99 {
			return fmt.Errorf("max stack size out of range: %d", value)
		}
	case datacomponent.ComponentMaxDamage, datacomponent.ComponentEnchantable:
		if value <= 0 {
			return fmt.Errorf("component value must be positive: %d", value)
		}
	case datacomponent.ComponentDamage, datacomponent.ComponentRepairCost, datacomponent.ComponentMapId:
		if value < 0 {
			return fmt.Errorf("component value must be non-negative: %d", value)
		}
	case datacomponent.ComponentOminousBottleAmplifier:
		if value < 0 || value > 4 {
			return fmt.Errorf("ominous bottle amplifier out of range: %d", value)
		}
	case datacomponent.ComponentRarity:
		if value < 0 || value > 3 {
			return fmt.Errorf("rarity out of range: %d", value)
		}
	case datacomponent.ComponentMapPostProcessing:
		if value < 0 || value > 1 {
			return fmt.Errorf("map post-processing value out of range: %d", value)
		}
	case datacomponent.ComponentBaseColor:
		if value < 0 || value > 15 {
			return fmt.Errorf("dye color out of range: %d", value)
		}
	}
	return nil
}

func finiteFloat(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}

func decodeCustomModelData(buf *protocol.Buffer) (ComponentValue, error) {
	value := CustomModelDataComponent{}
	count, err := readBoundedListCount(buf)
	if err != nil {
		return nil, err
	}
	value.Floats = make([]float32, count)
	for i := range value.Floats {
		value.Floats[i], err = buf.ReadFloat32()
		if err != nil || !finiteFloat(value.Floats[i]) {
			return nil, fmt.Errorf("invalid custom model float at %d", i)
		}
	}
	count, err = readBoundedListCount(buf)
	if err != nil {
		return nil, err
	}
	value.Flags = make([]bool, count)
	for i := range value.Flags {
		value.Flags[i], err = buf.ReadBool()
		if err != nil {
			return nil, err
		}
	}
	count, err = readBoundedListCount(buf)
	if err != nil {
		return nil, err
	}
	value.Strings = make([]string, count)
	for i := range value.Strings {
		value.Strings[i], err = buf.ReadStringLimit(32767)
		if err != nil {
			return nil, err
		}
	}
	count, err = readBoundedListCount(buf)
	if err != nil {
		return nil, err
	}
	value.Colors = make([]int32, count)
	for i := range value.Colors {
		value.Colors[i], err = buf.ReadInt32()
		if err != nil {
			return nil, err
		}
	}
	return value, nil
}

func readBoundedListCount(buf *protocol.Buffer) (int, error) {
	count, err := buf.ReadVarInt()
	if err != nil {
		return 0, err
	}
	if count < 0 || count > maxComponentMapEntries {
		return 0, fmt.Errorf("component list size out of range: %d", count)
	}
	return int(count), nil
}

func decodeEnchantments(buf *protocol.Buffer) (ComponentValue, error) {
	count, err := buf.ReadVarInt()
	if err != nil {
		return nil, err
	}
	if count < 0 || count > maxComponentMapEntries {
		return nil, fmt.Errorf("enchantment count out of range: %d", count)
	}
	levels := make(map[int32]int32, count)
	for i := int32(0); i < count; i++ {
		id, readErr := buf.ReadVarInt()
		if readErr != nil {
			return nil, readErr
		}
		level, readErr := buf.ReadVarInt()
		if readErr != nil {
			return nil, readErr
		}
		if id < 0 || level < 0 || level > 255 {
			return nil, fmt.Errorf("invalid enchantment id=%d level=%d", id, level)
		}
		if _, duplicate := levels[id]; duplicate {
			return nil, fmt.Errorf("duplicate enchantment ID: %d", id)
		}
		levels[id] = level
	}
	show, err := buf.ReadBool()
	return EnchantmentsComponent{Levels: levels, ShowInTooltip: show}, err
}

func decodeStringMap(buf *protocol.Buffer) (ComponentValue, error) {
	count, err := buf.ReadVarInt()
	if err != nil {
		return nil, err
	}
	if count < 0 || count > maxComponentMapEntries {
		return nil, fmt.Errorf("component map size out of range: %d", count)
	}
	values := make(StringMapComponent, count)
	for i := int32(0); i < count; i++ {
		key, readErr := buf.ReadStringLimit(128)
		if readErr != nil {
			return nil, readErr
		}
		value, readErr := buf.ReadStringLimit(128)
		if readErr != nil {
			return nil, readErr
		}
		if _, duplicate := values[key]; duplicate {
			return nil, fmt.Errorf("duplicate component map key %q", key)
		}
		values[key] = value
	}
	return values, nil
}

// EncodeComponentPatch encodes one validated data-component patch.
func EncodeComponentPatch(buf *protocol.Buffer, patch ComponentPatch) error {
	return encodeComponentPatch(buf, patch, 0)
}

// encodeComponentPatch validates a patch within a bounded item-stack graph.
func encodeComponentPatch(buf *protocol.Buffer, patch ComponentPatch, depth int) error {
	if len(patch.Added)+len(patch.Removed) > maxComponentPatchEntries {
		return fmt.Errorf("component patch has too many entries")
	}
	tmp := protocol.NewBuffer(128)
	tmp.WriteVarInt(int32(len(patch.Added)))
	tmp.WriteVarInt(int32(len(patch.Removed)))
	seen := make(map[int32]struct{}, len(patch.Added)+len(patch.Removed))
	for _, component := range patch.Added {
		if err := validateComponentType(component.TypeID, seen); err != nil {
			return err
		}
		tmp.WriteVarInt(component.TypeID)
		payload := protocol.NewBuffer(64)
		if len(component.Raw) > 0 {
			payload.WriteBytes(component.Raw)
		} else if value, ok := component.Value.(StructuredComponent); ok {
			if err := encodeStructuredComponent(payload, component.TypeID, value, depth); err != nil {
				return err
			}
		} else {
			if err := validateComponentValueType(component.TypeID, component.Value); err != nil {
				return err
			}
			if err := encodeComponentValue(payload, component.Value, depth); err != nil {
				return fmt.Errorf("encode component %d: %w", component.TypeID, err)
			}
		}
		if _, err := decodeComponentValue(payload, component.TypeID, depth); err != nil {
			return fmt.Errorf("validate component %d (%s): %w", component.TypeID, datacomponent.DataComponentNameByID(component.TypeID), err)
		}
		if payload.Remaining() != 0 {
			return fmt.Errorf("component %d has %d trailing payload bytes", component.TypeID, payload.Remaining())
		}
		tmp.WriteBytes(payload.Bytes())
	}
	for _, typeID := range patch.Removed {
		if err := validateComponentType(typeID, seen); err != nil {
			return err
		}
		tmp.WriteVarInt(typeID)
	}
	buf.WriteBytes(tmp.Bytes())
	return nil
}

func validateComponentValueType(typeID int32, value ComponentValue) error {
	valid := false
	switch typeID {
	case datacomponent.ComponentCustomData, datacomponent.ComponentEntityData,
		datacomponent.ComponentBucketEntityData, datacomponent.ComponentBlockEntityData:
		_, valid = value.(NBTComponent)
	case datacomponent.ComponentCustomName, datacomponent.ComponentItemName:
		_, valid = value.(RawNBTComponent)
	case datacomponent.ComponentLore:
		_, valid = value.(LoreComponent)
	case datacomponent.ComponentMaxStackSize, datacomponent.ComponentMaxDamage,
		datacomponent.ComponentDamage, datacomponent.ComponentRepairCost,
		datacomponent.ComponentEnchantable, datacomponent.ComponentMapId,
		datacomponent.ComponentOminousBottleAmplifier, datacomponent.ComponentRarity,
		datacomponent.ComponentMapPostProcessing,
		datacomponent.ComponentBaseColor:
		_, valid = value.(VarIntComponent)
	case datacomponent.ComponentUnbreakable, datacomponent.ComponentEnchantmentGlintOverride:
		_, valid = value.(BoolComponent)
	case datacomponent.ComponentHideAdditionalTooltip, datacomponent.ComponentHideTooltip,
		datacomponent.ComponentCreativeSlotLock, datacomponent.ComponentGlider:
		_, valid = value.(UnitComponent)
	case datacomponent.ComponentItemModel, datacomponent.ComponentTooltipStyle, datacomponent.ComponentNoteBlockSound:
		_, valid = value.(ResourceLocationComponent)
	case datacomponent.ComponentEnchantments, datacomponent.ComponentStoredEnchantments:
		_, valid = value.(EnchantmentsComponent)
	case datacomponent.ComponentDyedColor:
		_, valid = value.(DyedColorComponent)
	case datacomponent.ComponentMapColor:
		_, valid = value.(IntComponent)
	case datacomponent.ComponentBlockState:
		_, valid = value.(StringMapComponent)
	case datacomponent.ComponentCustomModelData:
		_, valid = value.(CustomModelDataComponent)
	case datacomponent.ComponentFood:
		_, valid = value.(FoodComponent)
	case datacomponent.ComponentUseRemainder:
		_, valid = value.(UseRemainderComponent)
	case datacomponent.ComponentPotDecorations:
		_, valid = value.(PotDecorationsComponent)
	case datacomponent.ComponentUseCooldown:
		_, valid = value.(UseCooldownComponent)
	case datacomponent.ComponentFireworkExplosion:
		_, valid = value.(FireworkExplosionComponent)
	case datacomponent.ComponentFireworks:
		_, valid = value.(FireworksComponent)
	}
	if !valid {
		if _, ok := value.(StructuredComponent); ok && componentSchemas[typeID] != nil {
			return nil
		}
		return fmt.Errorf("component %d (%s) has incompatible value %T", typeID, datacomponent.DataComponentNameByID(typeID), value)
	}
	return nil
}

func validateComponentType(typeID int32, seen map[int32]struct{}) error {
	if typeID < 0 || typeID >= datacomponent.DataComponentCount {
		return fmt.Errorf("component type ID out of range: %d", typeID)
	}
	if _, duplicate := seen[typeID]; duplicate {
		return fmt.Errorf("duplicate component type ID: %d", typeID)
	}
	seen[typeID] = struct{}{}
	return nil
}

func encodeComponentValue(buf *protocol.Buffer, value ComponentValue, depth int) error {
	switch typed := value.(type) {
	case VarIntComponent:
		buf.WriteVarInt(int32(typed))
	case IntComponent:
		buf.WriteInt32(int32(typed))
	case BoolComponent:
		buf.WriteBool(bool(typed))
	case UnitComponent:
	case ResourceLocationComponent:
		return buf.WriteString(string(typed))
	case NBTComponent:
		encoder := nbt.NewEncoder(256)
		if err := encoder.WriteRootCompound(typed.Value); err != nil {
			return err
		}
		if len(encoder.Bytes()) > maxComponentNBTBytes {
			return fmt.Errorf("component NBT exceeds %d bytes", maxComponentNBTBytes)
		}
		buf.WriteBytes(encoder.Bytes())
	case RawNBTComponent:
		if len(typed.Data) == 0 || len(typed.Data) > maxComponentNBTBytes {
			return fmt.Errorf("component NBT size out of range: %d", len(typed.Data))
		}
		decoder := nbt.NewDecoder(typed.Data)
		if _, _, err := decoder.ReadRootTag(); err != nil || decoder.Pos() != len(typed.Data) {
			return fmt.Errorf("invalid component NBT")
		}
		buf.WriteBytes(typed.Data)
	case LoreComponent:
		if len(typed) > maxComponentMapEntries {
			return fmt.Errorf("too many lore lines")
		}
		buf.WriteVarInt(int32(len(typed)))
		for _, line := range typed {
			if err := encodeComponentValue(buf, line, depth); err != nil {
				return err
			}
		}
	case DyedColorComponent:
		buf.WriteInt32(typed.RGB)
		buf.WriteBool(typed.ShowInTooltip)
	case EnchantmentsComponent:
		if len(typed.Levels) > maxComponentMapEntries {
			return fmt.Errorf("too many enchantments")
		}
		buf.WriteVarInt(int32(len(typed.Levels)))
		ids := make([]int32, 0, len(typed.Levels))
		for id := range typed.Levels {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		for _, id := range ids {
			level := typed.Levels[id]
			if id < 0 || level < 0 || level > 255 {
				return fmt.Errorf("invalid enchantment id=%d level=%d", id, level)
			}
			buf.WriteVarInt(id)
			buf.WriteVarInt(level)
		}
		buf.WriteBool(typed.ShowInTooltip)
	case StringMapComponent:
		if len(typed) > maxComponentMapEntries {
			return fmt.Errorf("component map too large")
		}
		buf.WriteVarInt(int32(len(typed)))
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			entry := typed[key]
			if err := buf.WriteString(key); err != nil {
				return err
			}
			if err := buf.WriteString(entry); err != nil {
				return err
			}
		}
	case CustomModelDataComponent:
		if len(typed.Floats) > maxComponentMapEntries || len(typed.Flags) > maxComponentMapEntries || len(typed.Strings) > maxComponentMapEntries || len(typed.Colors) > maxComponentMapEntries {
			return fmt.Errorf("custom model data list too large")
		}
		buf.WriteVarInt(int32(len(typed.Floats)))
		for _, entry := range typed.Floats {
			if !finiteFloat(entry) {
				return fmt.Errorf("invalid custom model float")
			}
			buf.WriteFloat32(entry)
		}
		buf.WriteVarInt(int32(len(typed.Flags)))
		for _, entry := range typed.Flags {
			buf.WriteBool(entry)
		}
		buf.WriteVarInt(int32(len(typed.Strings)))
		for _, entry := range typed.Strings {
			if err := buf.WriteString(entry); err != nil {
				return err
			}
		}
		buf.WriteVarInt(int32(len(typed.Colors)))
		for _, entry := range typed.Colors {
			buf.WriteInt32(entry)
		}
	case FoodComponent:
		if typed.Nutrition < 0 || typed.Saturation < 0 || !finiteFloat(typed.Saturation) {
			return fmt.Errorf("invalid food properties")
		}
		buf.WriteVarInt(typed.Nutrition)
		buf.WriteFloat32(typed.Saturation)
		buf.WriteBool(typed.CanAlwaysEat)
	case UseRemainderComponent:
		if typed.Stack.Empty() {
			return fmt.Errorf("use remainder must contain an item")
		}
		return encodeItemStack(buf, typed.Stack, depth+1)
	case PotDecorationsComponent:
		if len(typed) > 4 {
			return fmt.Errorf("too many pot decorations: %d", len(typed))
		}
		buf.WriteVarInt(int32(len(typed)))
		for _, itemID := range typed {
			if itemID < 0 || itemID >= itemdata.ItemCount {
				return fmt.Errorf("pot decoration item ID out of range: %d", itemID)
			}
			buf.WriteVarInt(itemID)
		}
	case UseCooldownComponent:
		if typed.Seconds < 0 || !finiteFloat(typed.Seconds) {
			return fmt.Errorf("invalid cooldown duration")
		}
		buf.WriteFloat32(typed.Seconds)
		buf.WriteBool(typed.HasGroup)
		if typed.HasGroup {
			return buf.WriteString(typed.Group)
		}
	case FireworkExplosionComponent:
		return encodeFireworkExplosion(buf, typed)
	case FireworksComponent:
		if typed.FlightDuration < 0 || typed.FlightDuration > 255 || len(typed.Explosions) > maxComponentMapEntries {
			return fmt.Errorf("invalid fireworks component")
		}
		buf.WriteVarInt(typed.FlightDuration)
		buf.WriteVarInt(int32(len(typed.Explosions)))
		for _, explosion := range typed.Explosions {
			if err := encodeFireworkExplosion(buf, explosion); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported component value %T", value)
	}
	return nil
}

func encodeFireworkExplosion(buf *protocol.Buffer, value FireworkExplosionComponent) error {
	if value.Shape < 0 || value.Shape > 4 || len(value.Colors) > maxComponentMapEntries || len(value.FadeColors) > maxComponentMapEntries {
		return fmt.Errorf("invalid firework explosion")
	}
	buf.WriteVarInt(value.Shape)
	buf.WriteVarInt(int32(len(value.Colors)))
	for _, color := range value.Colors {
		buf.WriteInt32(color)
	}
	buf.WriteVarInt(int32(len(value.FadeColors)))
	for _, color := range value.FadeColors {
		buf.WriteInt32(color)
	}
	buf.WriteBool(value.Trail)
	buf.WriteBool(value.Twinkle)
	return nil
}

// componentNBTDecoder bounds one tag before allocating untrusted compound contents.
func componentNBTDecoder(data []byte) *nbt.Decoder {
	if len(data) > maxComponentNBTBytes {
		data = data[:maxComponentNBTBytes]
	}
	return nbt.NewDecoderWithLimits(data, nbt.DecoderLimits{MaxBytes: maxComponentNBTBytes, MaxDepth: 64, MaxNodes: 16384, MaxCollectionLength: 16384})
}
