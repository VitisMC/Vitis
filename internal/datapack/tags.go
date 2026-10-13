package datapack

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrTagNotFound indicates a missing referenced tag.
var ErrTagNotFound = errors.New("tag not found")

// ResolveTag expands a tag across pack layers and nested tag references.
func (s *Snapshot) ResolveTag(registry, id string) ([]string, error) {
	if s == nil {
		return nil, fmt.Errorf("resolve tag: nil snapshot")
	}
	if !validResourceSubpath(registry) {
		return nil, fmt.Errorf("invalid tag registry %q", registry)
	}
	return s.resolveTag(registry, id, make(map[string]bool))
}

// resolveTag resolves one tag and detects reference cycles.
func (s *Snapshot) resolveTag(registry, id string, visiting map[string]bool) ([]string, error) {
	name, err := tagResourcePath(registry, id)
	if err != nil {
		return nil, err
	}
	if visiting[name] {
		return nil, fmt.Errorf("cyclic tag reference %q", id)
	}
	layers := s.resources[name]
	if len(layers) == 0 {
		return nil, fmt.Errorf("%w: %q in %q", ErrTagNotFound, id, registry)
	}
	visiting[name] = true
	defer delete(visiting, name)
	values := make([]string, 0)
	seen := make(map[string]bool)
	for _, layer := range layers {
		var tag struct {
			Replace bool              `json:"replace"`
			Values  []json.RawMessage `json:"values"`
		}
		if err := json.Unmarshal(layer.Data, &tag); err != nil {
			return nil, fmt.Errorf("parse tag %q in pack %q: %w", id, layer.Pack, err)
		}
		if tag.Values == nil {
			return nil, fmt.Errorf("tag %q in pack %q has no values", id, layer.Pack)
		}
		if tag.Replace {
			values = values[:0]
			clear(seen)
		}
		for _, raw := range tag.Values {
			entry, required, err := parseTagEntry(raw)
			if err != nil {
				return nil, fmt.Errorf("tag %q in pack %q: %w", id, layer.Pack, err)
			}
			if strings.HasPrefix(entry, "#") {
				nestedID := strings.TrimPrefix(entry, "#")
				nestedPath, err := tagResourcePath(registry, nestedID)
				if err != nil {
					return nil, err
				}
				if !required && len(s.resources[nestedPath]) == 0 {
					continue
				}
				nested, err := s.resolveTag(registry, nestedID, visiting)
				if err != nil {
					return nil, err
				}
				for _, value := range nested {
					if !seen[value] {
						seen[value] = true
						values = append(values, value)
					}
				}
				continue
			}
			if !seen[entry] {
				seen[entry] = true
				values = append(values, entry)
			}
		}
	}
	return values, nil
}

// parseTagEntry decodes one string or object tag entry.
func parseTagEntry(raw json.RawMessage) (string, bool, error) {
	var id string
	if err := json.Unmarshal(raw, &id); err == nil {
		return normalizeTagEntry(id, true)
	}
	var entry struct {
		ID       string `json:"id"`
		Required *bool  `json:"required"`
	}
	if err := json.Unmarshal(raw, &entry); err != nil {
		return "", false, fmt.Errorf("invalid tag entry %s", raw)
	}
	required := entry.Required == nil || *entry.Required
	return normalizeTagEntry(entry.ID, required)
}

// normalizeTagEntry applies the default namespace and validates an entry.
func normalizeTagEntry(id string, required bool) (string, bool, error) {
	prefix := ""
	if strings.HasPrefix(id, "#") {
		prefix = "#"
		id = strings.TrimPrefix(id, "#")
	}
	if !strings.Contains(id, ":") {
		id = "minecraft:" + id
	}
	if !validResourceID(id) {
		return "", false, fmt.Errorf("invalid tag entry %q", prefix+id)
	}
	return prefix + id, required, nil
}

// tagResourcePath converts a registry and tag ID to a canonical pack path.
func tagResourcePath(registry, id string) (string, error) {
	if !strings.Contains(id, ":") {
		id = "minecraft:" + id
	}
	if !validResourceID(id) {
		return "", fmt.Errorf("invalid tag ID %q", id)
	}
	namespace, local, _ := strings.Cut(id, ":")
	return "data/" + namespace + "/tags/" + registry + "/" + local + ".json", nil
}

// validResourceID checks a namespaced identifier.
func validResourceID(id string) bool {
	namespace, local, ok := strings.Cut(id, ":")
	return ok && validIdentifier(namespace, false) && validResourceSubpath(local)
}

// validResourceSubpath checks a slash-delimited resource path.
func validResourceSubpath(value string) bool {
	if value == "" {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if !validIdentifier(part, true) {
			return false
		}
	}
	return true
}
