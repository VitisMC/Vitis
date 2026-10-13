// Package inventory owns authoritative stacks, menus, and item serialization.
package inventory

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	ench "github.com/vitismc/vitis/internal/data/generated/enchantment"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

// ItemStackFromNBT decodes a persistent stack without tying its data to a slot.
func ItemStackFromNBT(root *nbt.Compound) (ItemStack, error) { return itemStackFromNBT(root, 0) }

// itemStackFromNBT decodes one bounded persistent item graph.
func itemStackFromNBT(root *nbt.Compound, depth int) (ItemStack, error) {
	if root == nil {
		return EmptySlot(), nil
	}
	if depth > maxNestedItemStackDepth {
		return ItemStack{}, fmt.Errorf("item NBT nesting exceeds limit")
	}
	name, ok := root.GetString("id")
	if !ok {
		return ItemStack{}, fmt.Errorf("item NBT has no id")
	}
	id := item.IDByName(name)
	if id <= 0 {
		return ItemStack{}, fmt.Errorf("unknown item %q", name)
	}
	if _, tag, present := root.Get("count"); present && tag != nbt.TagInt {
		return ItemStack{}, fmt.Errorf("invalid count type")
	}
	if _, tag, present := root.Get("Count"); present && tag != nbt.TagByte {
		return ItemStack{}, fmt.Errorf("invalid legacy count type")
	}
	if _, tag, present := root.Get("components"); present && tag != nbt.TagCompound {
		return ItemStack{}, fmt.Errorf("invalid component patch type")
	}
	count := int32(1)
	if n, ok := root.GetInt("count"); ok {
		count = n
	} else if n, ok := root.GetByte("Count"); ok {
		count = int32(n)
	}
	if count < 1 || count > 99 {
		return ItemStack{}, fmt.Errorf("invalid count %d", count)
	}
	s := NewSlot(id, count)
	s.PersistentData = root.Clone()
	s.PersistentData.Delete("Slot")
	components, _ := root.GetCompound("components")
	for _, name := range components.Keys() {
		value, tag, _ := components.Get(name)
		if strings.HasPrefix(name, "!") {
			if _, _, exists := components.Get(strings.TrimPrefix(name, "!")); exists {
				return ItemStack{}, fmt.Errorf("conflicting component patch %s", name)
			}
			if tag != nbt.TagCompound {
				return ItemStack{}, fmt.Errorf("invalid component removal %s", name)
			}
			typeID := dc.DataComponentIDByName(strings.TrimPrefix(name, "!"))
			if typeID >= 0 {
				s.ComponentPatch.Removed = append(s.ComponentPatch.Removed, typeID)
			}
			continue
		}
		typeID := dc.DataComponentIDByName(name)
		if typeID < 0 {
			continue
		}
		decoded, err := componentFromNBT(typeID, tag, value, depth)
		if err != nil {
			return ItemStack{}, fmt.Errorf("component %s: %w", name, err)
		}
		if decoded != nil {
			s.SetComponent(typeID, decoded)
		}
	}
	if v, ok := s.Component(dc.ComponentDamage); ok {
		s.Damage = int32(v.(VarIntComponent))
	} else if v, ok := root.GetInt("Damage"); ok && !slices.Contains(s.ComponentPatch.Removed, dc.ComponentDamage) {
		s.SetDamage(v)
	}
	if v, ok := s.Component(dc.ComponentEnchantments); ok {
		if levels := v.(EnchantmentsComponent).Levels; len(levels) > 0 {
			s.Enchantments = levels
		}
	}
	if err := encodeComponentPatch(protocol.NewBuffer(256), s.componentPatchForEncoding(), depth); err != nil {
		return ItemStack{}, err
	}
	if count > s.MaxStackSize() {
		return ItemStack{}, fmt.Errorf("count exceeds effective stack size")
	}
	return s, nil
}

// ItemStackToNBT encodes a stack while retaining unknown persistent fields.
func ItemStackToNBT(s ItemStack) (*nbt.Compound, error) { return itemStackToNBT(s, 0) }

// itemStackToNBT writes one bounded persistent item graph.
func itemStackToNBT(s ItemStack, depth int) (*nbt.Compound, error) {
	if s.Empty() {
		return nil, nil
	}
	if depth > maxNestedItemStackDepth {
		return nil, fmt.Errorf("item NBT nesting exceeds limit")
	}
	name := item.NameByID(s.ItemID)
	if name == "" || s.ItemCount < 1 || s.ItemCount > 99 {
		return nil, fmt.Errorf("invalid item stack")
	}
	root := s.PersistentData.Clone()
	if root == nil {
		root = nbt.NewCompound()
	}
	root.Delete("Slot")
	root.Delete("Count")
	root.Delete("Damage")
	root.Delete("Enchantments")
	root.PutString("id", name).PutInt("count", s.ItemCount)
	components, _ := root.GetCompound("components")
	components = components.Clone()
	if components == nil {
		components = nbt.NewCompound()
	}
	patch := s.componentPatchForEncoding()
	if err := encodeComponentPatch(protocol.NewBuffer(256), patch, depth); err != nil {
		return nil, err
	}
	for _, entry := range patch.Added {
		name := dc.DataComponentNameByID(entry.TypeID)
		if name == "" {
			return nil, fmt.Errorf("unknown component %d", entry.TypeID)
		}
		tag, value, err := componentToNBT(entry.TypeID, entry.Value, depth)
		if err != nil {
			return nil, err
		}
		if entry.TypeID == dc.ComponentEnchantments || entry.TypeID == dc.ComponentStoredEnchantments {
			if previous, ok := components.GetCompound(name); ok {
				if oldLevels, ok := previous.GetCompound("levels"); ok {
					if current, ok := value.(*nbt.Compound); ok {
						levels, _ := current.GetCompound("levels")
						for name, level := range oldLevels.GetAllInts() {
							if ench.EnchantmentIDByName(name) < 0 {
								levels.PutInt(name, level)
							}
						}
					}
				}
			}
		}
		previous, oldTag, exists := components.Get(name)
		if exists {
			if oldValue, err := componentFromNBT(entry.TypeID, oldTag, previous, depth); err == nil {
				if _, known, err := componentToNBT(entry.TypeID, oldValue, depth); err == nil {
					value = mergeUnknownNBT(value, unknownNBT(previous, known))
				}
			}
		}
		components.PutTag(name, tag, value)
		components.Delete("!" + name)
	}
	for _, id := range patch.Removed {
		name := dc.DataComponentNameByID(id)
		components.Delete(name)
		components.PutCompound("!"+name, nbt.NewCompound())
	}
	if len(components.Keys()) > 0 {
		root.PutCompound("components", components)
	} else {
		root.Delete("components")
	}
	return root, nil
}

// componentFromNBT reads the persistent representation of a registered value.
func componentFromNBT(id int32, tag byte, value any, depth int) (ComponentValue, error) {
	switch id {
	case dc.ComponentCustomData, dc.ComponentEntityData, dc.ComponentBucketEntityData, dc.ComponentBlockEntityData:
		c, ok := value.(*nbt.Compound)
		if !ok {
			return nil, fmt.Errorf("expected compound")
		}
		return NBTComponent{Value: c.Clone()}, nil
	case dc.ComponentCustomName, dc.ComponentItemName:
		return flatTextFromNBT(value)
	case dc.ComponentLore:
		list, ok := value.(*nbt.List)
		if !ok {
			return nil, fmt.Errorf("expected lore list")
		}
		result := make(LoreComponent, list.Len())
		for i, v := range list.Elements() {
			text, err := flatTextFromNBT(v)
			if err != nil {
				return nil, err
			}
			result[i] = text
		}
		return result, nil
	case dc.ComponentUnbreakable:
		c, ok := value.(*nbt.Compound)
		if !ok {
			return nil, fmt.Errorf("expected unbreakable compound")
		}
		b, ok := c.GetBool("show_in_tooltip")
		if !ok {
			b = true
		}
		return BoolComponent(b), nil
	case dc.ComponentEnchantments, dc.ComponentStoredEnchantments:
		c, ok := value.(*nbt.Compound)
		if !ok {
			return nil, fmt.Errorf("expected enchantments")
		}
		levels, _ := c.GetCompound("levels")
		result := EnchantmentsComponent{Levels: map[int32]int32{}, ShowInTooltip: true}
		if b, ok := c.GetBool("show_in_tooltip"); ok {
			result.ShowInTooltip = b
		}
		for name, n := range levels.GetAllInts() {
			i := ench.EnchantmentIDByName(name)
			if i < 0 {
				continue
			}
			if n < 0 || n > 255 {
				return nil, fmt.Errorf("invalid enchantment %s", name)
			}
			result.Levels[i] = n
		}
		return result, nil
	case dc.ComponentHideAdditionalTooltip, dc.ComponentHideTooltip, dc.ComponentCreativeSlotLock, dc.ComponentGlider:
		if _, ok := value.(*nbt.Compound); !ok {
			return nil, fmt.Errorf("expected unit compound")
		}
		return UnitComponent{}, nil
	case dc.ComponentItemModel, dc.ComponentTooltipStyle, dc.ComponentNoteBlockSound:
		v, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected identifier")
		}
		return ResourceLocationComponent(v), nil
	case dc.ComponentEnchantmentGlintOverride:
		v, ok := value.(int8)
		if !ok {
			return nil, fmt.Errorf("expected boolean")
		}
		return BoolComponent(v != 0), nil
	case dc.ComponentDyedColor:
		c, ok := value.(*nbt.Compound)
		if !ok {
			return nil, fmt.Errorf("expected dyed color")
		}
		rgb, ok := c.GetInt("rgb")
		if !ok {
			return nil, fmt.Errorf("missing rgb")
		}
		show := true
		if b, ok := c.GetBool("show_in_tooltip"); ok {
			show = b
		}
		return DyedColorComponent{RGB: rgb, ShowInTooltip: show}, nil
	case dc.ComponentMapColor:
		v, ok := value.(int32)
		if !ok {
			return nil, fmt.Errorf("expected integer")
		}
		return IntComponent(v), nil
	case dc.ComponentBlockState:
		c, ok := value.(*nbt.Compound)
		if !ok {
			return nil, fmt.Errorf("expected properties")
		}
		return StringMapComponent(c.GetAllStrings()), nil
	case dc.ComponentUseRemainder:
		c, ok := value.(*nbt.Compound)
		if !ok {
			return nil, fmt.Errorf("expected remainder")
		}
		s, err := itemStackFromNBT(c, depth+1)
		return UseRemainderComponent{Stack: s}, err
	case dc.ComponentMaxStackSize, dc.ComponentMaxDamage, dc.ComponentDamage, dc.ComponentRepairCost, dc.ComponentMapId, dc.ComponentOminousBottleAmplifier:
		n, ok := value.(int32)
		if !ok {
			return nil, fmt.Errorf("expected integer")
		}
		if err := validateVarIntComponent(id, n); err != nil {
			return nil, err
		}
		return VarIntComponent(n), nil
	case dc.ComponentMapPostProcessing:
		name, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected map postprocessing name")
		}
		i := slices.Index([]string{"scale", "lock"}, name)
		if i < 0 {
			return nil, fmt.Errorf("invalid map postprocessing")
		}
		return VarIntComponent(i), nil
	case dc.ComponentEnchantable:
		c, ok := value.(*nbt.Compound)
		if !ok {
			return nil, fmt.Errorf("expected enchantable")
		}
		n, ok := c.GetInt("value")
		if !ok {
			return nil, fmt.Errorf("missing enchantable value")
		}
		return VarIntComponent(n), nil
	case dc.ComponentRarity:
		name, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected rarity")
		}
		i := slices.Index([]string{"common", "uncommon", "rare", "epic"}, name)
		if i < 0 {
			return nil, fmt.Errorf("invalid rarity")
		}
		return VarIntComponent(i), nil
	default:
		return extendedComponentFromNBT(id, tag, value, depth)
	}
}

// componentToNBT converts a runtime component to vanilla persistent NBT.
func componentToNBT(id int32, value ComponentValue, depth int) (byte, any, error) {
	if id == dc.ComponentBaseColor {
		return extendedComponentToNBT(id, value, depth)
	}
	switch v := value.(type) {
	case VarIntComponent:
		if id == dc.ComponentMapPostProcessing {
			if v < 0 || v > 1 {
				return 0, nil, fmt.Errorf("invalid map postprocessing")
			}
			return nbt.TagString, []string{"scale", "lock"}[int(v)], nil
		}
		if id == dc.ComponentRarity {
			return nbt.TagString, []string{"common", "uncommon", "rare", "epic"}[int(v)], nil
		}
		if id == dc.ComponentEnchantable {
			return nbt.TagCompound, nbt.NewCompound().PutInt("value", int32(v)), nil
		}
		return nbt.TagInt, int32(v), nil
	case IntComponent:
		return nbt.TagInt, int32(v), nil
	case BoolComponent:
		if id == dc.ComponentUnbreakable {
			return nbt.TagCompound, nbt.NewCompound().PutBool("show_in_tooltip", bool(v)), nil
		}
		var b int8
		if v {
			b = 1
		}
		return nbt.TagByte, b, nil
	case UnitComponent:
		return nbt.TagCompound, nbt.NewCompound(), nil
	case ResourceLocationComponent:
		return nbt.TagString, string(v), nil
	case NBTComponent:
		return nbt.TagCompound, v.Value.Clone(), nil
	case RawNBTComponent:
		return flatTextToNBT(v)
	case LoreComponent:
		list := nbt.NewList(nbt.TagString)
		for _, line := range v {
			t, n, err := flatTextToNBT(line)
			if err != nil {
				return 0, nil, err
			}
			if list.Len() == 0 {
				list = nbt.NewList(t)
			}
			if list.ElementType() != t {
				return 0, nil, fmt.Errorf("mixed lore NBT types")
			}
			list.Add(n)
		}
		return nbt.TagList, list, nil
	case EnchantmentsComponent:
		levels := nbt.NewCompound()
		ids := make([]int32, 0, len(v.Levels))
		for i := range v.Levels {
			ids = append(ids, i)
		}
		slices.Sort(ids)
		for _, i := range ids {
			if i < 0 || int(i) >= len(ench.Enchantments) {
				return 0, nil, fmt.Errorf("invalid enchantment")
			}
			levels.PutInt(ench.Enchantments[i].Name, v.Levels[i])
		}
		return nbt.TagCompound, nbt.NewCompound().PutCompound("levels", levels).PutBool("show_in_tooltip", v.ShowInTooltip), nil
	case DyedColorComponent:
		return nbt.TagCompound, nbt.NewCompound().PutInt("rgb", v.RGB).PutBool("show_in_tooltip", v.ShowInTooltip), nil
	case StringMapComponent:
		c := nbt.NewCompound()
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			c.PutString(k, v[k])
		}
		return nbt.TagCompound, c, nil
	case UseRemainderComponent:
		c, err := itemStackToNBT(v.Stack, depth+1)
		return nbt.TagCompound, c, err
	default:
		return extendedComponentToNBT(id, value, depth)
	}
}

// persistentExtrasEqual compares unknown item fields independently of tag order.
func persistentExtrasEqual(a, b *nbt.Compound) bool {
	extras := func(c *nbt.Compound) *nbt.Compound {
		c = c.Clone()
		if c == nil {
			return nbt.NewCompound()
		}
		for _, key := range []string{"id", "count", "Count", "Slot", "Damage", "Enchantments"} {
			c.Delete(key)
		}
		if components, ok := c.GetCompound("components"); ok {
			for _, name := range components.Keys() {
				if dc.DataComponentIDByName(strings.TrimPrefix(name, "!")) >= 0 {
					if strings.HasPrefix(name, "!") {
						components.Delete(name)
						continue
					}
					value, tag, _ := components.Get(name)
					id := dc.DataComponentIDByName(name)
					if decoded, err := componentFromNBT(id, tag, value, 0); err == nil {
						if _, known, err := componentToNBT(id, decoded, 0); err == nil {
							extra := unknownNBT(value, known)
							if extra == nil {
								components.Delete(name)
							} else {
								components.PutTag(name, tag, extra)
							}
						}
					}
				}
			}
			if len(components.Keys()) == 0 {
				c.Delete("components")
			}
		}
		return c
	}
	return nbtValueEqual(extras(a), extras(b))
}

// nbtValueEqual compares NBT structures without treating compound order as data.
func nbtValueEqual(a, b any) bool {
	switch av := a.(type) {
	case *nbt.Compound:
		bv, ok := b.(*nbt.Compound)
		if !ok || len(av.Keys()) != len(bv.Keys()) {
			return false
		}
		for _, key := range av.Keys() {
			x, t, _ := av.Get(key)
			y, u, ok := bv.Get(key)
			if !ok || t != u || !nbtValueEqual(x, y) {
				return false
			}
		}
		return true
	case *nbt.List:
		bv, ok := b.(*nbt.List)
		if !ok || av.ElementType() != bv.ElementType() || av.Len() != bv.Len() {
			return false
		}
		for i, v := range av.Elements() {
			if !nbtValueEqual(v, bv.Elements()[i]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a, b)
	}
}

// unknownNBT extracts persistent fields not represented by the known codec.
func unknownNBT(original, known any) any {
	if old, ok := original.(*nbt.List); ok {
		decoded, ok := known.(*nbt.List)
		if !ok || old.ElementType() != nbt.TagCompound || decoded.ElementType() != nbt.TagCompound {
			return nil
		}
		extras := nbt.NewList(nbt.TagCompound)
		found := false
		for i, entry := range decoded.Elements() {
			previous := persistentListEntry(old, entry, i)
			extra, _ := unknownNBT(previous, entry).(*nbt.Compound)
			if extra == nil {
				extra = nbt.NewCompound()
			} else {
				found = true
			}
			extras.Add(extra)
		}
		if found {
			return extras
		}
		return nil
	}
	old, ok := original.(*nbt.Compound)
	if !ok {
		return nil
	}
	decoded, ok := known.(*nbt.Compound)
	if !ok {
		return nil
	}
	result := old.Clone()
	for _, key := range decoded.Keys() {
		value, tag, exists := old.Get(key)
		if !exists {
			continue
		}
		expected, _, _ := decoded.Get(key)
		if extra := unknownNBT(value, expected); extra != nil {
			result.PutTag(key, tag, extra)
		} else {
			result.Delete(key)
		}
	}
	if len(result.Keys()) == 0 {
		return nil
	}
	return result
}

// mergeUnknownNBT keeps uninterpreted fields while replacing known component data.
func mergeUnknownNBT(known, extra any) any {
	if data, ok := known.(*nbt.List); ok {
		unknown, ok := extra.(*nbt.List)
		if !ok || data.ElementType() != nbt.TagCompound || unknown.ElementType() != nbt.TagCompound {
			return known
		}
		result := nbt.NewList(nbt.TagCompound)
		entries := unknown.Elements()
		for i, value := range data.Elements() {
			var previous any
			if i < len(entries) {
				previous = entries[i]
			}
			result.Add(mergeUnknownNBT(value, previous))
		}
		return result
	}
	data, ok := known.(*nbt.Compound)
	if !ok {
		return known
	}
	unknown, ok := extra.(*nbt.Compound)
	if !ok {
		return known
	}
	result := unknown.Clone()
	for _, key := range data.Keys() {
		value, tag, _ := data.Get(key)
		previous, _, _ := unknown.Get(key)
		result.PutTag(key, tag, mergeUnknownNBT(value, previous))
	}
	return result
}

// structuredValuesEqual compares recursive wire values by item and NBT semantics.
func structuredValuesEqual(a, b any) bool {
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for key, value := range av {
			other, ok := bv[key]
			if !ok || !structuredValuesEqual(value, other) {
				return false
			}
		}
		return true
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !structuredValuesEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	case NetworkTag:
		bv, ok := b.(NetworkTag)
		return ok && av.Type == bv.Type && nbtValueEqual(av.Value, bv.Value)
	case RegistryHolder:
		bv, ok := b.(RegistryHolder)
		return ok && av.ID == bv.ID && structuredValuesEqual(av.Data, bv.Data)
	case ItemStack:
		bv, ok := b.(ItemStack)
		return ok && av.ItemCount == bv.ItemCount && av.StackableWith(bv)
	default:
		return reflect.DeepEqual(a, b)
	}
}

// persistentListEntry matches compound entries by stable slot or resource keys.
func persistentListEntry(list *nbt.List, expected any, index int) any {
	compound, ok := expected.(*nbt.Compound)
	if !ok {
		return nil
	}
	for _, key := range []string{"slot", "id", "pattern"} {
		value, tag, exists := compound.Get(key)
		if !exists {
			continue
		}
		for _, entry := range list.Elements() {
			if candidate, ok := entry.(*nbt.Compound); ok {
				other, kind, exists := candidate.Get(key)
				if exists && kind == tag && nbtValueEqual(value, other) {
					return candidate
				}
			}
		}
		return nil
	}
	entries := list.Elements()
	if index < len(entries) {
		return entries[index]
	}
	return nil
}
