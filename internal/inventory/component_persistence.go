package inventory

import (
	"fmt"
	"math"
	"slices"
	"strings"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/registry"
)

var componentRegistry = newComponentRegistry()

// newComponentRegistry loads immutable identifiers used by component codecs.
func newComponentRegistry() *registry.Manager {
	m, err := registry.NewManager()
	if err != nil {
		panic(fmt.Sprintf("component registry: %v", err))
	}
	return m
}

// persistentField translates wire field names into their persistent codec names.
func persistentField(name string) string {
	aliases := map[string]string{"uuid": "id", "hasTrail": "has_trail", "hasTwinkle": "has_twinkle", "fadeColors": "fade_colors", "flightDuration": "flight_duration", "filteredContent": "filtered", "content": "raw", "attributes": "modifiers", "colorId": "color", "decorations": "decorations", "showTooltip": "show_in_tooltip", "showInTooltip": "show_in_tooltip", "saturationModifier": "saturation", "canAlwaysEat": "can_always_eat", "cooldownGroup": "group", "correctDropForBlocks": "correct_for_drops", "defaultMiningSpeed": "default_mining_speed", "damagePerBlock": "damage_per_block", "cameraOverlay": "camera_overlay", "allowedEntities": "allowed_entities", "damageable": "damage_on_hurt", "model": "asset_id", "potionId": "potion", "customColor": "custom_color", "customEffects": "custom_effects", "customName": "custom_name", "rawTitle": "title", "generation": "generation", "useDuration": "use_duration", "soundEvent": "sound_event", "soundName": "sound_id", "fixedRange": "range", "assetName": "asset_name", "assetId": "asset_id", "ingredientId": "ingredient", "templateItemId": "template_item", "overrideArmorAssets": "override_armor_materials", "translationKey": "translation_key", "lengthInSeconds": "length_in_seconds", "comparatorOutput": "comparator_output", "ticksInHive": "ticks_in_hive", "minTicksInHive": "min_ticks_in_hive", "nbtData": "entity_data", "typeId": "type", "name": "id", "value": "amount", "operation": "operation", "blockSet": "blocks", "properties": "state", "showParticles": "show_particles", "showIcon": "show_icon", "hiddenEffect": "hidden_effect", "globalPosition": "target", "position": "pos"}
	if v, ok := aliases[name]; ok {
		return v
	}
	return name
}

// componentFieldName applies codec-specific exceptions to field names.
func componentFieldName(id int32, name, path string) string {
	if path == "" {
		if id == dc.ComponentConsumable {
			if name == "effects" {
				return "on_consume_effects"
			}
			if name == "makes_particles" {
				return "has_consume_particles"
			}
		}
		if id == dc.ComponentDeathProtection && name == "effects" {
			return "death_effects"
		}
		if id == dc.ComponentEquippable && name == "sound" {
			return "equip_sound"
		}
	}
	if name == "properties" && id != dc.ComponentCanPlaceOn && id != dc.ComponentCanBreak {
		return name
	}
	if id == dc.ComponentSuspiciousStewEffects && name == "effect" {
		return "id"
	}
	if id != dc.ComponentAttributeModifiers && (name == "name" || name == "value") {
		return name
	}
	if id == dc.ComponentPotionContents && name == "id" {
		return "id"
	}
	if id == dc.ComponentWrittenBookContent && name == "rawTitle" {
		return "title"
	}
	return persistentField(name)
}

// fieldRegistry resolves the registry associated with a component field.
func fieldRegistry(id int32, path string) string {
	last := path[strings.LastIndex(path, "/")+1:]
	if id == dc.ComponentPotDecorations && last == "entry" {
		return "minecraft:item"
	}
	switch last {
	case "typeId":
		return "minecraft:attribute"
	case "ingredientId", "templateItemId":
		return "minecraft:item"
	case "potionId":
		return "minecraft:potion"
	case "effect", "id":
		if id == dc.ComponentPotionContents || id == dc.ComponentSuspiciousStewEffects || id == dc.ComponentConsumable || id == dc.ComponentDeathProtection {
			return "minecraft:mob_effect"
		}
	case "material":
		return "minecraft:trim_material"
	case "pattern":
		if id == dc.ComponentTrim {
			return "minecraft:trim_pattern"
		}
		return "minecraft:banner_pattern"
	case "sound", "soundEvent":
		return "minecraft:sound_event"
	case "song":
		return "minecraft:jukebox_song"
	case "instrument":
		return "minecraft:instrument"
	}
	if path == "" && id == dc.ComponentInstrument {
		return "minecraft:instrument"
	}
	return ""
}

// setRegistry resolves the entries accepted by a registry-set component.
func setRegistry(id int32, path string) string {
	switch id {
	case dc.ComponentEquippable:
		return "minecraft:entity_type"
	case dc.ComponentRepairable:
		return "minecraft:item"
	case dc.ComponentConsumable, dc.ComponentDeathProtection:
		return "minecraft:mob_effect"
	default:
		return "minecraft:block"
	}
}

// extendedComponentFromNBT converts compound and nested persistent components.
func extendedComponentFromNBT(id int32, tag byte, value any, depth int) (ComponentValue, error) {
	if id == dc.ComponentBaseColor {
		name, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected dye name")
		}
		i := slices.Index(dyeNames, name)
		if i < 0 {
			return nil, fmt.Errorf("invalid dye")
		}
		return VarIntComponent(i), nil
	}
	schema := componentSchemas[id]
	if schema == nil {
		return nil, fmt.Errorf("missing persistent codec")
	}
	value, err := wrapPersistentComponent(id, value)
	if err != nil {
		return nil, err
	}
	budget := 16384
	decoded, err := wireFromNBT(schema, value, id, "", depth, 0, &budget)
	if err != nil {
		return nil, err
	}
	// Reuse typed values for components with hand-written gameplay codecs.
	buf := protocol.NewBuffer(128)
	v := StructuredComponent{Data: decoded}
	if err := encodeStructuredComponent(buf, id, v, depth); err != nil {
		return nil, err
	}
	if id == dc.ComponentContainer || id == dc.ComponentBundleContents || id == dc.ComponentChargedProjectiles {
		return v, nil
	}
	return decodeComponentValue(buf, id, depth)
}

// extendedComponentToNBT converts a component through its validated wire fields.
func extendedComponentToNBT(id int32, value ComponentValue, depth int) (byte, any, error) {
	if id == dc.ComponentBaseColor {
		v, ok := value.(VarIntComponent)
		if !ok || v < 0 || int(v) >= len(dyeNames) {
			return 0, nil, fmt.Errorf("invalid dye")
		}
		return nbt.TagString, dyeNames[v], nil
	}
	var structured StructuredComponent
	if v, ok := value.(StructuredComponent); ok {
		structured = v
	} else {
		buf := protocol.NewBuffer(128)
		if err := encodeComponentValue(buf, value, depth); err != nil {
			return 0, nil, err
		}
		decoded, err := decodeStructuredComponent(buf, id, depth)
		if err != nil {
			return 0, nil, err
		}
		structured = decoded.(StructuredComponent)
	}
	budget := 16384
	tag, result, err := wireToNBT(componentSchemas[id], structured.Data, id, "", depth, 0, &budget)
	if err != nil {
		return 0, nil, err
	}
	return unwrapPersistentComponent(id, tag, result)
}

var dyeNames = []string{"white", "orange", "magenta", "light_blue", "yellow", "lime", "pink", "gray", "light_gray", "cyan", "purple", "blue", "brown", "green", "red", "black"}

// wrapPersistentComponent reconciles list and scalar persistent root codecs.
func wrapPersistentComponent(id int32, value any) (any, error) {
	wrappers := map[int32]string{dc.ComponentChargedProjectiles: "projectiles", dc.ComponentBundleContents: "contents", dc.ComponentSuspiciousStewEffects: "effects", dc.ComponentBannerPatterns: "layers", dc.ComponentBees: "bees", dc.ComponentPotDecorations: "decorations"}
	if key, ok := wrappers[id]; ok {
		return nbt.NewCompound().PutTag(componentFieldName(id, key, ""), nbtTagType(value), value), nil
	}
	if id == dc.ComponentContainer {
		list, ok := value.(*nbt.List)
		if !ok || list.Len() > 256 {
			return nil, fmt.Errorf("invalid container contents")
		}
		contents := nbt.NewList(nbt.TagCompound)
		slots := map[int32]*nbt.Compound{}
		highest := int32(-1)
		for _, x := range list.Elements() {
			c, ok := x.(*nbt.Compound)
			if !ok {
				return nil, fmt.Errorf("invalid container entry")
			}
			i, ok := c.GetInt("slot")
			stack, _ := c.GetCompound("item")
			if !ok || i < 0 || i >= 256 || stack == nil || len(stack.Keys()) == 0 {
				return nil, fmt.Errorf("invalid container slot or item")
			}
			if _, duplicate := slots[i]; duplicate {
				return nil, fmt.Errorf("duplicate container slot %d", i)
			}
			slots[i] = stack
			highest = max(highest, i)
		}
		for i := int32(0); i <= highest; i++ {
			c := slots[i]
			if c == nil {
				c = nbt.NewCompound()
			}
			contents.Add(c)
		}
		return nbt.NewCompound().PutList("contents", contents), nil
	}
	if id == dc.ComponentDamageResistant {
		if c, ok := value.(*nbt.Compound); ok {
			name, _ := c.GetString("types")
			return strings.TrimPrefix(name, "#"), nil
		}
	}
	if id == dc.ComponentJukeboxPlayable {
		if name, ok := value.(string); ok {
			return nbt.NewCompound().PutString("song", name), nil
		}
	}
	return value, nil
}

// unwrapPersistentComponent restores the vanilla root representation.
func unwrapPersistentComponent(id int32, tag byte, value any) (byte, any, error) {
	wrappers := map[int32]string{dc.ComponentChargedProjectiles: "projectiles", dc.ComponentBundleContents: "contents", dc.ComponentSuspiciousStewEffects: "effects", dc.ComponentBannerPatterns: "layers", dc.ComponentBees: "bees", dc.ComponentPotDecorations: "decorations"}
	if key, ok := wrappers[id]; ok {
		c, ok := value.(*nbt.Compound)
		if !ok {
			return 0, nil, fmt.Errorf("invalid list root")
		}
		v, t, ok := c.Get(componentFieldName(id, key, ""))
		if !ok {
			return 0, nil, fmt.Errorf("missing list")
		}
		return t, v, nil
	}
	if id == dc.ComponentContainer {
		c := value.(*nbt.Compound)
		list, _ := c.GetList("contents")
		result := nbt.NewList(nbt.TagCompound)
		for i, x := range list.Elements() {
			stack := x.(*nbt.Compound)
			if len(stack.Keys()) > 0 {
				result.Add(nbt.NewCompound().PutInt("slot", int32(i)).PutCompound("item", stack))
			}
		}
		return nbt.TagList, result, nil
	}
	if id == dc.ComponentDamageResistant {
		return nbt.TagCompound, nbt.NewCompound().PutString("types", "#"+value.(string)), nil
	}
	return tag, value, nil
}

// defaultWireField supplies defaults defined by vanilla persistent codecs.
func defaultWireField(id int32, name string, s *wireSchema) any {
	if s.kind == "string" && name == "rawTitle" {
		return ""
	}
	if s.kind == "option" || s.kind == "anonOptionalNbt" {
		return nil
	}
	if s.kind == "array" {
		return nbt.NewList(nbt.TagCompound)
	}
	if s.kind == "bool" {
		switch name {
		case "showTooltip", "showInTooltip", "showParticles", "showIcon", "tracked", "makes_particles", "dispensable", "swappable", "damageable":
			return int8(1)
		}
		return int8(0)
	}
	if s.kind == "f32" {
		switch name {
		case "consume_seconds":
			return float32(1.6)
		case "defaultMiningSpeed", "probability":
			return float32(1)
		case "diameter":
			return float32(16)
		}
		return float32(0)
	}
	if s.kind == "f64" {
		return float64(0)
	}
	if s.kind == "mapper" {
		if name == "animation" {
			return "eat"
		}
		return s.mappings[0]
	}
	if s.kind == "registryEntryHolder" && name == "sound" {
		sound := "minecraft:entity.generic.eat"
		if id == dc.ComponentEquippable {
			sound = "minecraft:item.armor.equip_generic"
		}
		return sound
	}
	if s.kind == "varint" || s.kind == "i32" {
		if name == "damagePerBlock" {
			return int32(1)
		}
		return int32(0)
	}
	if s.kind == "container" {
		return nbt.NewCompound()
	}
	if s.kind == "string" {
		return ""
	}
	return nil
}

// wireFromNBT converts persistent tag values into bounded wire fields.
func wireFromNBT(s *wireSchema, value any, id int32, path string, itemDepth, level int, budget *int) (any, error) {
	*budget--
	if *budget < 0 || level > 64 {
		return nil, fmt.Errorf("persistent component exceeds limits")
	}
	next := func(child *wireSchema, v any, p string) (any, error) {
		return wireFromNBT(child, v, id, p, itemDepth, level+1, budget)
	}
	switch s.kind {
	case "varint", "i32":
		if name, ok := value.(string); ok {
			reg := fieldRegistry(id, path)
			if reg != "" {
				n := componentRegistry.IDByName(reg, name)
				if n < 0 {
					return nil, fmt.Errorf("unknown %s entry %s", reg, name)
				}
				return n, nil
			}
			if strings.HasSuffix(path, "colorId") {
				n := slices.Index(dyeNames, name)
				if n >= 0 {
					return int32(n), nil
				}
			}
		}
		n, ok := integerNBT(value)
		if !ok {
			return nil, fmt.Errorf("expected integer at %s", path)
		}
		return n, nil
	case "i64":
		v, ok := value.(int64)
		if !ok {
			return nil, fmt.Errorf("expected long")
		}
		return v, nil
	case "position":
		if a, ok := persistentIntArray(value, 3); ok {
			return ((int64(a[0]) & 0x3ffffff) << 38) | ((int64(a[2]) & 0x3ffffff) << 12) | (int64(a[1]) & 0xfff), nil
		}
		return nil, fmt.Errorf("invalid position")
	case "f32":
		switch v := value.(type) {
		case float32:
			return v, nil
		case float64:
			return float32(v), nil
		case int32:
			return float32(v), nil
		}
		return nil, fmt.Errorf("expected float")
	case "f64":
		switch v := value.(type) {
		case float64:
			return v, nil
		case float32:
			return float64(v), nil
		case int32:
			return float64(v), nil
		}
		return nil, fmt.Errorf("expected double")
	case "bool":
		v, ok := value.(int8)
		if !ok {
			return nil, fmt.Errorf("expected boolean")
		}
		return v != 0, nil
	case "string":
		v, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected string at %s", path)
		}
		return v, nil
	case "UUID":
		a, ok := persistentIntArray(value, 4)
		if !ok {
			return nil, fmt.Errorf("invalid UUID")
		}
		b := protocol.NewBuffer(16)
		for _, v := range a {
			b.WriteInt32(v)
		}
		return b.Bytes(), nil
	case "void":
		return nil, nil
	case "Slot":
		if c, ok := value.(*nbt.Compound); ok {
			if len(c.Keys()) == 0 {
				if id != dc.ComponentContainer {
					return nil, fmt.Errorf("empty nested item")
				}
				return EmptySlot(), nil
			}
			return itemStackFromNBT(c, itemDepth+1)
		}
		return nil, fmt.Errorf("expected nested item")
	case "anonymousNbt", "anonOptionalNbt":
		if value == nil {
			return nil, nil
		}
		return NetworkTag{nbtTagType(value), value}, nil
	case "option":
		if value == nil {
			return nil, nil
		}
		return next(s.element, value, path)
	case "array":
		if ints, ok := value.([]int32); ok {
			list := nbt.NewList(nbt.TagInt)
			for _, n := range ints {
				list.Add(n)
			}
			value = list
		}
		if c, ok := value.(*nbt.Compound); ok && id == dc.ComponentTrim && strings.HasSuffix(path, "/overrideArmorAssets") {
			entries := nbt.NewList(nbt.TagCompound)
			for _, key := range c.Keys() {
				value, ok := c.GetString(key)
				if !ok {
					return nil, fmt.Errorf("invalid trim asset override")
				}
				entries.Add(nbt.NewCompound().PutString("key", key).PutString("value", value))
			}
			value = entries
		}
		if c, ok := value.(*nbt.Compound); ok && strings.HasSuffix(path, "/properties") {
			value = blockPropertyList(c)
		}
		list, ok := value.(*nbt.List)
		if !ok || list.Len() > maxComponentMapEntries {
			return nil, fmt.Errorf("invalid list at %s", path)
		}
		result := make([]any, list.Len())
		for i, x := range list.Elements() {
			v, err := next(s.element, x, path+"/entry")
			if err != nil {
				return nil, err
			}
			result[i] = v
		}
		return result, nil
	case "mapper":
		name, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected enum at %s", path)
		}
		if strings.HasSuffix(path, "operation") {
			name = strings.NewReplacer("add_value", "add", "add_multiplied_base", "multiply_base", "add_multiplied_total", "multiply_total").Replace(name)
		}
		if strings.HasSuffix(path, "slot") {
			name = strings.NewReplacer("mainhand", "main_hand", "offhand", "off_hand").Replace(name)
		}
		if strings.HasSuffix(path, "/type") {
			name = strings.TrimPrefix(name, "minecraft:")
		}
		if _, ok := s.reverse[name]; !ok {
			return nil, fmt.Errorf("invalid enum %s", name)
		}
		return name, nil
	case "container":
		if (id == dc.ComponentWritableBookContent || id == dc.ComponentWrittenBookContent) && strings.HasSuffix(path, "/pages/entry") {
			value = filterablePage(value)
		}
		c, ok := value.(*nbt.Compound)
		if !ok {
			return nil, fmt.Errorf("expected compound at %s", path)
		}
		m := map[string]any{}
		for _, field := range s.fields {
			if field.anon {
				child := field.schema
				if child.kind == "switch" {
					child = child.alternatives[fmt.Sprint(m[child.compare])]
				}
				if child == nil {
					return nil, fmt.Errorf("invalid union at %s", path)
				}
				v, err := next(child, c, path)
				if err != nil {
					return nil, err
				}
				if fields, ok := v.(map[string]any); ok {
					for key, x := range fields {
						m[key] = x
					}
				}
				continue
			}
			name := componentFieldName(id, field.name, path)
			v, _, exists := c.Get(name)
			if !exists {
				v = defaultWireField(id, field.name, field.schema)
			}
			if field.name == "value" && strings.HasSuffix(path, "/properties/entry") && (id == dc.ComponentCanPlaceOn || id == dc.ComponentCanBreak) {
				v = c
			}
			if field.name == "details" && (id == dc.ComponentPotionContents || id == dc.ComponentSuspiciousStewEffects || id == dc.ComponentConsumable || id == dc.ComponentDeathProtection) {
				v = c
			}
			if id == dc.ComponentWrittenBookContent && (field.name == "rawTitle" || field.name == "filteredTitle") {
				title, _, _ := c.Get("title")
				if tc, ok := title.(*nbt.Compound); ok {
					key := "raw"
					if field.name == "filteredTitle" {
						key = "filtered"
					}
					v, _, _ = tc.Get(key)
				} else if field.name == "rawTitle" {
					v = title
				}
			}
			if id == dc.ComponentWrittenBookContent && strings.HasSuffix(path, "/pages/entry") && (field.name == "content" || field.name == "filteredContent") && v != nil {
				text, err := flatTextFromNBT(v)
				if err != nil {
					return nil, err
				}
				_, decoded, err := nbt.NewDecoder(text.Data).ReadRootTag()
				if err != nil {
					return nil, err
				}
				v = decoded
			}
			if id == dc.ComponentJukeboxPlayable && field.name == "hasHolder" {
				v = int8(1)
			}
			child := field.schema
			if child.kind == "switch" {
				child = child.alternatives[fmt.Sprint(m[child.compare])]
			}
			if child == nil {
				return nil, fmt.Errorf("invalid union at %s", path)
			}
			decoded, err := wireFromNBT(child, v, id, path+"/"+field.name, itemDepth, level+1, budget)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			m[field.name] = decoded
		}
		return m, nil
	case "switch":
		c, ok := value.(*nbt.Compound)
		if !ok {
			return nil, fmt.Errorf("expected union")
		}
		v, _, _ := c.Get(componentFieldName(id, s.compare, path))
		key := fmt.Sprint(v)
		if s.compare == "hasHolder" {
			key = "true"
		}
		if s.compare == "type" {
			key = strings.TrimPrefix(key, "minecraft:")
		}
		child := s.alternatives[key]
		if child == nil {
			return nil, fmt.Errorf("invalid persistent union %s", key)
		}
		return next(child, value, path)
	case "registryEntryHolder":
		if name, ok := value.(string); ok {
			reg := fieldRegistry(id, path)
			n := componentRegistry.IDByName(reg, name)
			if n < 0 {
				return nil, fmt.Errorf("unknown %s entry %s", reg, name)
			}
			return RegistryHolder{ID: n}, nil
		}
		data, err := next(s.element, value, path)
		return RegistryHolder{ID: -1, Data: data}, err
	case "registryEntryHolderSet":
		reg := setRegistry(id, path)
		result := RegistrySet{}
		if name, ok := value.(string); ok {
			if strings.HasPrefix(name, "#") {
				result.Tag = strings.TrimPrefix(name, "#")
				return result, nil
			}
			n := componentRegistry.IDByName(reg, name)
			if n < 0 {
				return nil, fmt.Errorf("unknown set entry %s", name)
			}
			result.IDs = []int32{n}
			return result, nil
		}
		list, ok := value.(*nbt.List)
		if !ok || list.Len() > maxComponentMapEntries {
			return nil, fmt.Errorf("invalid set")
		}
		for _, x := range list.Elements() {
			name, ok := x.(string)
			if !ok {
				return nil, fmt.Errorf("invalid set identifier")
			}
			n := componentRegistry.IDByName(reg, name)
			if n < 0 {
				return nil, fmt.Errorf("unknown set entry %s", name)
			}
			result.IDs = append(result.IDs, n)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported persistent codec %s", s.kind)
	}
}

// wireToNBT converts wire fields back to their vanilla persistent representation.
func wireToNBT(s *wireSchema, value any, id int32, path string, itemDepth, level int, budget *int) (byte, any, error) {
	*budget--
	if *budget < 0 || level > 64 {
		return 0, nil, fmt.Errorf("component exceeds limits")
	}
	next := func(child *wireSchema, v any, p string) (byte, any, error) {
		return wireToNBT(child, v, id, p, itemDepth, level+1, budget)
	}
	switch s.kind {
	case "varint", "i32":
		v, ok := value.(int32)
		if !ok {
			return 0, nil, fmt.Errorf("invalid integer")
		}
		reg := fieldRegistry(id, path)
		if reg != "" {
			name := componentRegistry.NameByID(reg, v)
			if name == "" {
				return 0, nil, fmt.Errorf("unknown registry id %d", v)
			}
			return nbt.TagString, name, nil
		}
		if strings.HasSuffix(path, "colorId") {
			if v < 0 || int(v) >= len(dyeNames) {
				return 0, nil, fmt.Errorf("invalid dye")
			}
			return nbt.TagString, dyeNames[v], nil
		}
		return nbt.TagInt, v, nil
	case "i64":
		return nbt.TagLong, value, nil
	case "position":
		v := value.(int64)
		return nbt.TagIntArray, []int32{int32(v >> 38), int32(v << 52 >> 52), int32(v << 26 >> 38)}, nil
	case "f32":
		return nbt.TagFloat, value, nil
	case "f64":
		return nbt.TagDouble, value, nil
	case "bool":
		var v int8
		if value.(bool) {
			v = 1
		}
		return nbt.TagByte, v, nil
	case "string":
		return nbt.TagString, value, nil
	case "UUID":
		buf := protocol.WrapBuffer(value.([]byte))
		a := make([]int32, 4)
		for i := range a {
			a[i], _ = buf.ReadInt32()
		}
		return nbt.TagIntArray, a, nil
	case "void":
		return nbt.TagCompound, nbt.NewCompound(), nil
	case "Slot":
		v, err := itemStackToNBT(value.(ItemStack), itemDepth+1)
		if v == nil {
			v = nbt.NewCompound()
		}
		return nbt.TagCompound, v, err
	case "anonymousNbt", "anonOptionalNbt":
		if value == nil {
			return 0, nil, nil
		}
		v := value.(NetworkTag)
		return v.Type, v.Value, nil
	case "option":
		if value == nil {
			return 0, nil, nil
		}
		return next(s.element, value, path)
	case "array":
		if id == dc.ComponentTrim && strings.HasSuffix(path, "/overrideArmorAssets") {
			assets := nbt.NewCompound()
			for _, entry := range value.([]any) {
				fields := entry.(map[string]any)
				assets.PutString(fields["key"].(string), fields["value"].(string))
			}
			return nbt.TagCompound, assets, nil
		}
		list := nbt.NewList(nbt.TagCompound)
		for _, x := range value.([]any) {
			t, v, err := next(s.element, x, path+"/entry")
			if err != nil {
				return 0, nil, err
			}
			if list.Len() == 0 {
				list = nbt.NewList(t)
			}
			if list.ElementType() != t {
				return 0, nil, fmt.Errorf("mixed persistent list")
			}
			list.Add(v)
		}
		return nbt.TagList, list, nil
	case "mapper":
		name := value.(string)
		if (id == dc.ComponentConsumable || id == dc.ComponentDeathProtection) && strings.HasSuffix(path, "/type") {
			name = "minecraft:" + name
		}
		if strings.HasSuffix(path, "operation") {
			name = strings.NewReplacer("multiply_base", "add_multiplied_base", "multiply_total", "add_multiplied_total", "add", "add_value").Replace(name)
		}
		if strings.HasSuffix(path, "slot") {
			name = strings.NewReplacer("main_hand", "mainhand", "off_hand", "offhand").Replace(name)
		}
		return nbt.TagString, name, nil
	case "container":
		m := value.(map[string]any)
		c := nbt.NewCompound()
		for _, field := range s.fields {
			v := m[field.name]
			if field.anon {
				v = m
			}
			child := field.schema
			if child.kind == "switch" {
				child = child.alternatives[fmt.Sprint(m[child.compare])]
			}
			if child == nil {
				return 0, nil, fmt.Errorf("invalid union at %s", path)
			}
			t, x, err := wireToNBT(child, v, id, path+"/"+field.name, itemDepth, level+1, budget)
			if err != nil {
				return 0, nil, err
			}
			if x == nil {
				continue
			}
			if field.anon {
				nested := x.(*nbt.Compound)
				for _, key := range nested.Keys() {
					v, t, _ := nested.Get(key)
					c.PutTag(key, t, v)
				}
			} else if field.name == "value" && strings.HasSuffix(path, "/properties/entry") && (id == dc.ComponentCanPlaceOn || id == dc.ComponentCanBreak) {
				if nested, ok := x.(*nbt.Compound); ok {
					for _, key := range nested.Keys() {
						value, tag, _ := nested.Get(key)
						c.PutTag(key, tag, value)
					}
				}
			} else if field.name == "details" && (id == dc.ComponentPotionContents || id == dc.ComponentSuspiciousStewEffects || id == dc.ComponentConsumable || id == dc.ComponentDeathProtection) {
				if nested, ok := x.(*nbt.Compound); ok {
					for _, key := range nested.Keys() {
						value, tag, _ := nested.Get(key)
						c.PutTag(key, tag, value)
					}
				}
			} else if !(id == dc.ComponentJukeboxPlayable && field.name == "hasHolder") {
				if id == dc.ComponentWrittenBookContent && strings.HasSuffix(path, "/pages/entry") && (field.name == "content" || field.name == "filteredContent") {
					json, err := nbt.ToJSON(x)
					if err != nil {
						return 0, nil, err
					}
					t, x = nbt.TagString, string(json)
				}
				c.PutTag(componentFieldName(id, field.name, path), t, x)
			}
		}
		if id == dc.ComponentCanPlaceOn || id == dc.ComponentCanBreak {
			if fields, ok := c.GetList("state"); ok {
				state := nbt.NewCompound()
				for _, entry := range fields.Elements() {
					property := entry.(*nbt.Compound)
					name, _ := property.GetString("name")
					exact, _ := property.GetBool("isExactMatch")
					if exact {
						value, _ := property.GetString("exactValue")
						state.PutString(name, value)
					} else {
						lo, _ := property.GetString("minValue")
						hi, _ := property.GetString("maxValue")
						state.PutCompound(name, nbt.NewCompound().PutString("min", lo).PutString("max", hi))
					}
				}
				c.PutCompound("state", state)
			}
		}
		if id == dc.ComponentWrittenBookContent && path == "" {
			raw, _, _ := c.Get("title")
			title := nbt.NewCompound().PutTag("raw", nbt.TagString, raw)
			if filtered, _, ok := c.Get("filteredTitle"); ok {
				title.PutTag("filtered", nbt.TagString, filtered)
			}
			c.PutCompound("title", title)
			c.Delete("filteredTitle")
		}
		return nbt.TagCompound, c, nil
	case "switch":
		return 0, nil, fmt.Errorf("union requires parent context")
	case "registryEntryHolder":
		v := value.(RegistryHolder)
		if v.ID >= 0 {
			reg := fieldRegistry(id, path)
			name := componentRegistry.NameByID(reg, v.ID)
			if name == "" {
				return 0, nil, fmt.Errorf("unknown registry entry")
			}
			return nbt.TagString, name, nil
		}
		return next(s.element, v.Data, path)
	case "registryEntryHolderSet":
		v := value.(RegistrySet)
		if v.Tag != "" {
			return nbt.TagString, "#" + v.Tag, nil
		}
		list := nbt.NewList(nbt.TagString)
		reg := setRegistry(id, path)
		for _, i := range v.IDs {
			name := componentRegistry.NameByID(reg, i)
			if name == "" {
				return 0, nil, fmt.Errorf("unknown registry set id")
			}
			list.Add(name)
		}
		return nbt.TagList, list, nil
	default:
		return 0, nil, fmt.Errorf("unsupported persistent codec %s", s.kind)
	}
}

// nbtTagType identifies the NBT type of a codec value.
func nbtTagType(value any) byte {
	switch value.(type) {
	case int8:
		return nbt.TagByte
	case int16:
		return nbt.TagShort
	case int32:
		return nbt.TagInt
	case int64:
		return nbt.TagLong
	case float32:
		return nbt.TagFloat
	case float64:
		return nbt.TagDouble
	case string:
		return nbt.TagString
	case []byte:
		return nbt.TagByteArray
	case []int32:
		return nbt.TagIntArray
	case []int64:
		return nbt.TagLongArray
	case *nbt.List:
		return nbt.TagList
	case *nbt.Compound:
		return nbt.TagCompound
	default:
		return nbt.TagEnd
	}
}

// integerNBT converts an integral NBT number without narrowing silently.
func integerNBT(value any) (int32, bool) {
	switch v := value.(type) {
	case int32:
		return v, true
	case int8:
		return int32(v), true
	case int16:
		return int32(v), true
	case float64:
		if v >= math.MinInt32 && v <= math.MaxInt32 && v == math.Trunc(v) {
			return int32(v), true
		}
	}
	return 0, false
}

// itemIdentifier resolves a numeric item ID for persistent list codecs.
func itemIdentifier(id int32) string { return item.NameByID(id) }

// filterablePage normalizes the alternate string page representation.
func filterablePage(value any) *nbt.Compound {
	if page, ok := value.(*nbt.Compound); ok {
		return page
	}
	return nbt.NewCompound().PutTag("raw", nbtTagType(value), value)
}

// blockPropertyList expands the persistent property map into wire predicates.
func blockPropertyList(properties *nbt.Compound) *nbt.List {
	list := nbt.NewList(nbt.TagCompound)
	for _, name := range properties.Keys() {
		value, tag, _ := properties.Get(name)
		property := nbt.NewCompound().PutString("name", name)
		if tag == nbt.TagString {
			property.PutBool("isExactMatch", true).PutString("exactValue", value.(string))
		} else if bounds, ok := value.(*nbt.Compound); ok {
			lo, _ := bounds.GetString("min")
			hi, _ := bounds.GetString("max")
			property.PutBool("isExactMatch", false).PutString("minValue", lo).PutString("maxValue", hi)
		}
		list.Add(property)
	}
	return list
}

// persistentIntArray accepts NBT integer arrays and JSON-derived integer lists.
func persistentIntArray(value any, size int) ([]int32, bool) {
	if data, ok := value.([]int32); ok {
		return data, len(data) == size
	}
	list, ok := value.(*nbt.List)
	if !ok || list.Len() != size {
		return nil, false
	}
	data := make([]int32, size)
	for i, value := range list.Elements() {
		n, ok := integerNBT(value)
		if !ok {
			return nil, false
		}
		data[i] = n
	}
	return data, true
}
