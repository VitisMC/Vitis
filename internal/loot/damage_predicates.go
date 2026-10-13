package loot

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vitismc/vitis/internal/datapack"
)

// damageTag identifies a required or excluded damage type tag.
type damageTag struct {
	ID       string `json:"id"`
	Expected *bool  `json:"expected"`
}

// decodeDamageTags rejects malformed tag requirements before evaluation or publication.
func decodeDamageTags(raw json.RawMessage) ([]damageTag, error) {
	var tags []damageTag
	if json.Unmarshal(raw, &tags) != nil || strings.TrimSpace(string(raw)) == "null" {
		return nil, fmt.Errorf("invalid damage type tag predicates")
	}
	for _, tag := range tags {
		if tag.ID == "" || strings.HasPrefix(tag.ID, "#") || tag.Expected == nil {
			return nil, fmt.Errorf("invalid damage type tag predicate %q", tag.ID)
		}
	}
	return tags, nil
}

// matchesDamageTags evaluates required and excluded tags on the actual death cause.
func matchesDamageTags(raw json.RawMessage, ctx *Context) bool {
	tags, err := decodeDamageTags(raw)
	if err != nil {
		return false
	}
	if len(tags) > 0 && (ctx.DamageType == "" || ctx.registryTagResolver == nil) {
		return false
	}
	for _, tag := range tags {
		if matchesRegistryName("damage_type", "#"+tag.ID, ctx.DamageType, ctx) != *tag.Expected {
			return false
		}
	}
	return true
}

// validateDamageConditions resolves death-cause tags against the candidate resource snapshot.
func validateDamageConditions(snapshot *datapack.Snapshot, conditions []Condition) error {
	for _, condition := range conditions {
		if condition.Condition == "minecraft:damage_source_properties" {
			var fields map[string]json.RawMessage
			if json.Unmarshal(condition.Predicate, &fields) != nil || fields == nil {
				return fmt.Errorf("invalid damage source predicate")
			}
			if raw, ok := fields["is_direct"]; ok {
				var direct bool
				if strings.TrimSpace(string(raw)) == "null" || json.Unmarshal(raw, &direct) != nil {
					return fmt.Errorf("invalid direct damage predicate")
				}
			}
			if raw, ok := fields["tags"]; ok {
				tags, err := decodeDamageTags(raw)
				if err != nil {
					return err
				}
				for _, tag := range tags {
					if _, err := snapshot.ResolveTag("damage_type", tag.ID); err != nil {
						return fmt.Errorf("damage type tag %q: %w", tag.ID, err)
					}
				}
			}
		}
		if err := validateDamageConditions(snapshot, condition.Terms); err != nil {
			return err
		}
		if condition.Term != nil {
			if err := validateDamageConditions(snapshot, []Condition{*condition.Term}); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateFunctionDamageConditions checks conditional functions at every table level.
func validateFunctionDamageConditions(snapshot *datapack.Snapshot, functions []Function) error {
	for _, function := range functions {
		if err := validateDamageConditions(snapshot, function.Conditions); err != nil {
			return err
		}
	}
	return nil
}
