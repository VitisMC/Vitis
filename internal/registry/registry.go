package registry

import "fmt"

// IDRegistry provides O(1) bidirectional lookup between namespaced names and
// protocol IDs. It is immutable after construction.
type IDRegistry struct {
	name    string
	byName  map[string]int32
	byID    []string
}

// NewIDRegistry builds an IDRegistry from an ordered slice of namespaced names.
// The index in the slice becomes the protocol ID.
func NewIDRegistry(name string, names []string) *IDRegistry {
	byName := make(map[string]int32, len(names))
	for i, n := range names {
		byName[n] = int32(i)
	}
	dst := make([]string, len(names))
	copy(dst, names)
	return &IDRegistry{
		name:   name,
		byName: byName,
		byID:   dst,
	}
}

// Name returns the registry identifier (e.g. "minecraft:block").
func (r *IDRegistry) Name() string { return r.name }

// IDByName returns the protocol ID for the given namespaced name.
// Returns -1 if not found.
func (r *IDRegistry) IDByName(name string) int32 {
	if id, ok := r.byName[name]; ok {
		return id
	}
	return -1
}

// NameByID returns the namespaced name for the given protocol ID.
// Returns "" if out of range.
func (r *IDRegistry) NameByID(id int32) string {
	if id < 0 || int(id) >= len(r.byID) {
		return ""
	}
	return r.byID[id]
}

// Size returns the number of entries.
func (r *IDRegistry) Size() int { return len(r.byID) }

// Names returns a copy of all names ordered by protocol ID.
func (r *IDRegistry) Names() []string {
	out := make([]string, len(r.byID))
	copy(out, r.byID)
	return out
}

// Contains returns true if the name exists in this registry.
func (r *IDRegistry) Contains(name string) bool {
	_, ok := r.byName[name]
	return ok
}

// Validate checks internal consistency: non-empty, no duplicate names,
// contiguous IDs, all names are namespaced.
func (r *IDRegistry) Validate() error {
	if len(r.byID) == 0 {
		return fmt.Errorf("registry %q is empty", r.name)
	}
	if len(r.byName) != len(r.byID) {
		return fmt.Errorf("registry %q has %d names but %d IDs (duplicates?)",
			r.name, len(r.byName), len(r.byID))
	}
	for i, n := range r.byID {
		if n == "" {
			return fmt.Errorf("registry %q: entry %d has empty name", r.name, i)
		}
		if id, ok := r.byName[n]; !ok || id != int32(i) {
			return fmt.Errorf("registry %q: inconsistent mapping for %q at %d", r.name, n, i)
		}
	}
	return nil
}
