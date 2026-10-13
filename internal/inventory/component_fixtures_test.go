package inventory

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestComponentPersistentFixtures verifies populated codecs absent from item defaults.
func TestComponentPersistentFixtures(t *testing.T) {
	fixtures := componentPersistentFixtures()

	seen := map[string]bool{}
	for _, definition := range defaultDefinitions {
		for name := range definition.Components {
			seen[name] = true
		}
	}
	for name := range fixtures {
		seen["minecraft:"+name] = true
	}
	for id := int32(0); id < dc.DataComponentCount; id++ {
		if !seen[dc.DataComponentNameByID(id)] {
			t.Errorf("component has no fixture: %s", dc.DataComponentNameByID(id))
		}
	}
	for name, fixture := range fixtures {
		t.Run(name, func(t *testing.T) {
			assertComponentFixture(t, name, fixture)
		})
	}
}

// assertComponentFixture verifies a persistent, wire, and persistent round trip.
func assertComponentFixture(t *testing.T, name, fixture string) {
	t.Helper()
	id := dc.DataComponentIDByName("minecraft:" + name)
	tag, value, err := nbt.FromJSON([]byte(fixture))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := componentFromNBT(id, tag, value, 0)
	if err != nil {
		t.Fatal(err)
	}
	buffer := protocol.NewBuffer(256)
	if err := EncodeComponentPatch(buffer, ComponentPatch{Added: []DataComponent{{TypeID: id, Value: decoded}}}); err != nil {
		t.Fatal(err)
	}
	patch, err := DecodeComponentPatch(buffer)
	if err != nil || buffer.Remaining() != 0 {
		t.Fatalf("wire decode: %v", err)
	}
	tag, persistent, err := componentToNBT(id, patch.Added[0].Value, 0)
	if err != nil {
		t.Fatal(err)
	}
	assertPersistentFixture(t, fixture, persistent)
	again, err := componentFromNBT(id, tag, persistent, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !componentValuesEqual(decoded, again) {
		t.Fatalf("component data changed: before=%#v after=%#v", decoded, again)
	}
}

// assertPersistentFixture checks supplied fields before accepting a codec round trip.
func assertPersistentFixture(t *testing.T, fixture string, persistent any) {
	t.Helper()
	encoded, err := nbt.ToJSON(persistent)
	if err != nil {
		t.Fatal(err)
	}
	var expected, actual any
	if err := json.Unmarshal([]byte(fixture), &expected); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &actual); err != nil {
		t.Fatal(err)
	}
	if err := persistentFixtureSubset(expected, actual, ""); err != nil {
		t.Fatalf("persistent fields changed: %v; encoded=%s", err, encoded)
	}
}

// persistentFixtureSubset compares supplied values while allowing codec defaults.
func persistentFixtureSubset(expected, actual any, path string) error {
	switch expected := expected.(type) {
	case map[string]any:
		fields, ok := actual.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: expected object, got %T", path, actual)
		}
		for key, value := range expected {
			if err := persistentFixtureSubset(value, fields[key], path+"."+key); err != nil {
				return err
			}
		}
	case []any:
		values, ok := actual.([]any)
		if !ok || len(values) != len(expected) {
			return fmt.Errorf("%s: array changed", path)
		}
		for i, value := range expected {
			if err := persistentFixtureSubset(value, values[i], fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case float64:
		value, ok := actual.(float64)
		if !ok || math.Abs(value-expected) > 1e-6*math.Max(1, math.Abs(expected)) {
			return fmt.Errorf("%s: expected %v, got %v", path, expected, actual)
		}
	case string:
		if values, ok := actual.([]any); ok && len(values) == 1 && (strings.HasSuffix(path, ".blocks") || strings.HasSuffix(path, ".items") || strings.HasSuffix(path, ".effects") || strings.HasSuffix(path, ".allowed_entities")) {
			return persistentFixtureSubset(expected, values[0], path)
		}
		value, ok := actual.(string)
		if !ok || value != expected {
			var expectedJSON, actualJSON any
			if !ok || json.Unmarshal([]byte(expected), &expectedJSON) != nil || json.Unmarshal([]byte(value), &actualJSON) != nil || !reflect.DeepEqual(expectedJSON, actualJSON) {
				return fmt.Errorf("%s: expected %q, got %v", path, expected, actual)
			}
		}
	default:
		if !reflect.DeepEqual(expected, actual) {
			return fmt.Errorf("%s: expected %v, got %v", path, expected, actual)
		}
	}
	return nil
}

// TestStackabilityUsesEffectiveComponents checks defaults, deletion, and unknown data.
func TestStackabilityUsesEffectiveComponents(t *testing.T) {
	a := NewSlot(1, 1)
	b := a.Clone()
	b.SetComponent(dc.ComponentMaxStackSize, VarIntComponent(a.MaxStackSize()))
	if !a.StackableWith(b) {
		t.Fatal("explicit default prevented stacking")
	}
	b.RemoveComponent(dc.ComponentMaxStackSize)
	if a.StackableWith(b) {
		t.Fatal("removed default was ignored")
	}
}

// componentPersistentFixtures supplies populated values for codec and fuzz coverage.
func componentPersistentFixtures() map[string]string {
	return map[string]string{
		"attribute_modifiers": `{"modifiers":[{"type":"minecraft:attack_damage","id":"minecraft:custom","amount":1.5,"operation":"add_value","slot":"mainhand"}],"show_in_tooltip":false}`,
		"consumable":          `{"consume_seconds":2.5,"animation":"drink","sound":"minecraft:entity.generic.drink","has_consume_particles":false,"on_consume_effects":[{"type":"minecraft:apply_effects","effects":[{"id":"minecraft:speed","amplifier":2,"duration":120}],"probability":0.5},{"type":"minecraft:remove_effects","effects":["minecraft:poison"]},{"type":"minecraft:teleport_randomly","diameter":8.0},{"type":"minecraft:play_sound","sound":"minecraft:entity.generic.eat"}]}`,
		"death_protection":    `{"death_effects":[{"type":"minecraft:clear_all_effects"}]}`,
		"equippable":          `{"slot":"head","equip_sound":"minecraft:item.armor.equip_leather","asset_id":"minecraft:leather","camera_overlay":"minecraft:misc/pumpkinblur","allowed_entities":["minecraft:player"],"dispensable":false,"swappable":false,"damage_on_hurt":false}`,
		"tool":                `{"rules":[{"blocks":["minecraft:stone"],"speed":3.5,"correct_for_drops":true}],"default_mining_speed":2.0,"damage_per_block":2}`,
		"instrument":          `"minecraft:ponder_goat_horn"`,
		"unbreakable":         `{"show_in_tooltip":false}`, "tooltip_style": `"minecraft:custom"`, "note_block_sound": `"minecraft:block.note_block.harp"`, "lodestone_tracker": `{"target":{"dimension":"minecraft:overworld","pos":[0,64,0]},"tracked":true}`,
		"custom_data": `{"nested":{"value":42}}`, "custom_name": `"{\"text\":\"Named\",\"bold\":true}"`, "lore": `["{\"text\":\"First\"}","\"Second\""]`,
		"can_place_on":            `{"predicates":[{"blocks":"#minecraft:planks","state":{"axis":"y"},"nbt":{"a":1}}]}`,
		"can_break":               `{"predicates":[{"blocks":["minecraft:stone"]}]}`,
		"custom_model_data":       `{"floats":[1.5,2.0],"flags":[true,false],"strings":["model"],"colors":[255]}`,
		"hide_additional_tooltip": `{}`, "hide_tooltip": `{}`, "creative_slot_lock": `{}`, "enchantment_glint_override": `true`, "intangible_projectile": `{}`,
		"use_remainder":       `{"id":"minecraft:glass_bottle","count":1,"components":{"minecraft:custom_name":"\"Bottle\""}}`,
		"stored_enchantments": `{"levels":{"minecraft:sharpness":5},"show_in_tooltip":false}`, "dyed_color": `{"rgb":16711680}`, "map_color": `16711680`, "map_id": `7`,
		"map_decorations": `{"target":{"type":"minecraft:red_x","x":1.0,"z":2.0,"rotation":0.0}}`, "map_post_processing": `"scale"`,
		"charged_projectiles": `[{"id":"minecraft:arrow","count":1}]`, "bundle_contents": `[{"id":"minecraft:stone","count":2}]`,
		"potion_contents":         `{"potion":"minecraft:strong_healing","custom_effects":[{"id":"minecraft:speed","amplifier":1,"duration":100,"show_particles":false}]}`,
		"suspicious_stew_effects": `[{"id":"minecraft:night_vision","duration":100}]`,
		"writable_book_content":   `{"pages":[{"raw":"plain"},{"raw":"raw","filtered":"safe"}]}`,
		"written_book_content":    `{"title":{"raw":"Title","filtered":"Safe"},"author":"Tester","generation":1,"pages":[{"raw":"{\"text\":\"Page\"}"},{"raw":"\"Raw\"","filtered":"\"Filtered\""}],"resolved":true}`,
		"trim":                    `{"material":"minecraft:gold","pattern":"minecraft:sentry"}`, "debug_stick_state": `{"minecraft:oak_log":"axis"}`, "entity_data": `{"id":"minecraft:pig"}`, "bucket_entity_data": `{"Health":10.0}`, "block_entity_data": `{"id":"minecraft:chest"}`,
		"recipes": `["minecraft:stick"]`, "firework_explosion": `{"shape":"star","colors":[11743532],"fade_colors":[2437522],"has_trail":true,"has_twinkle":true}`,
		"fireworks":       `{"flight_duration":2,"explosions":[{"shape":"large_ball","colors":[255],"fade_colors":[65535],"has_trail":true}]}`,
		"profile":         `{"name":"Tester","id":[1,2,3,4],"properties":[{"name":"textures","value":"texture","signature":"signature"}]}`,
		"banner_patterns": `[{"pattern":"minecraft:cross","color":"red"}]`, "base_color": `"blue"`,
		"pot_decorations": `["minecraft:brick","minecraft:archer_pottery_sherd","minecraft:brick","minecraft:brick"]`,
		"container":       `[{"slot":2,"item":{"id":"minecraft:stone","count":2}}]`, "block_state": `{"axis":"y"}`,
		"bees": `[{"entity_data":{"id":"minecraft:bee"},"ticks_in_hive":10,"min_ticks_in_hive":20}]`, "lock": `{"components":{"minecraft:custom_name":"\"Key\""}}`, "container_loot": `{"loot_table":"minecraft:chests/simple_dungeon","seed":1}`,
	}
}

// inlineComponentFixtures supplies anonymous registry holder values.
func inlineComponentFixtures() map[string]string {
	return map[string]string{
		"instrument":       `{"sound_event":{"sound_id":"minecraft:custom","range":10.0},"use_duration":7.5,"range":200.0,"description":{"text":"Instrument"}}`,
		"jukebox_playable": `{"song":{"sound_event":"minecraft:music_disc.cat","description":{"text":"Song"},"length_in_seconds":42.0,"comparator_output":7},"show_in_tooltip":false}`,
		"trim":             `{"material":{"asset_name":"custom","ingredient":"minecraft:gold_ingot","override_armor_materials":{"minecraft:gold":"custom_dark"},"description":{"text":"Material"}},"pattern":{"asset_id":"minecraft:custom","template_item":"minecraft:sentry_armor_trim_smithing_template","description":{"text":"Pattern"},"decal":true}}`,
		"banner_patterns":  `[{"pattern":{"asset_id":"minecraft:custom","translation_key":"block.minecraft.banner.custom"},"color":"red"}]`,
	}
}

// TestInlineComponentHolders checks anonymous registry entries rather than only IDs.
func TestInlineComponentHolders(t *testing.T) {
	for name, fixture := range inlineComponentFixtures() {
		t.Run(name, func(t *testing.T) { assertComponentFixture(t, name, fixture) })
	}
}

// TestInlineComponentValueBounds rejects invalid anonymous holder fields.
func TestInlineComponentValueBounds(t *testing.T) {
	fixtures := map[string]string{
		"instrument":       `{"sound_event":"minecraft:block.note_block.harp","use_duration":0.0,"range":20.0,"description":{"text":"Invalid"}}`,
		"jukebox_playable": `{"song":{"sound_event":"minecraft:music_disc.cat","description":{"text":"Invalid"},"length_in_seconds":42.0,"comparator_output":16}}`,
		"death_protection": `{"death_effects":[{"type":"minecraft:teleport_randomly","diameter":0.0}]}`,
	}
	for name, fixture := range fixtures {
		t.Run(name, func(t *testing.T) {
			tag, value, err := nbt.FromJSON([]byte(fixture))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := componentFromNBT(dc.DataComponentIDByName("minecraft:"+name), tag, value, 0); err == nil {
				t.Fatal("invalid component value accepted")
			}
		})
	}
}
