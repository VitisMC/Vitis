// Package advancement loads and tracks data driven player advancements.
package advancement

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	genreg "github.com/vitismc/vitis/internal/data/generated/registry"
	"github.com/vitismc/vitis/internal/datapack"
)

const compiledAdvancementsKey = "advancements"

// Runtime exposes definitions from the current atomic resource snapshot.
type Runtime struct {
	resources *datapack.Manager
}

// Definition is one validated advancement definition.
type Definition struct {
	ID             string               `json:"-"`
	Parent         string               `json:"parent,omitempty"`
	Display        json.RawMessage      `json:"display,omitempty"`
	Criteria       map[string]Criterion `json:"criteria"`
	Requirements   [][]string           `json:"requirements,omitempty"`
	Rewards        Rewards              `json:"rewards,omitempty"`
	SendsTelemetry bool                 `json:"sends_telemetry_event,omitempty"`
}

// Criterion describes an advancement trigger and its predicate.
type Criterion struct {
	Trigger    string          `json:"trigger"`
	Conditions json.RawMessage `json:"conditions,omitempty"`
}

// Rewards describes rewards granted when an advancement completes.
type Rewards struct {
	Experience int      `json:"experience,omitempty"`
	Loot       []string `json:"loot,omitempty"`
	Recipes    []string `json:"recipes,omitempty"`
	Function   string   `json:"function,omitempty"`
}

// DefinitionSet contains all active definitions in parent first order.
type DefinitionSet struct {
	definitions map[string]*Definition
	ordered     []string
}

// Progress stores granted criteria for one player.
type Progress struct {
	mu      sync.RWMutex
	granted map[string]map[string]time.Time
}

// NewRuntime registers advancement compilation with the resource manager.
func NewRuntime(resources *datapack.Manager) (*Runtime, error) {
	if resources == nil {
		return nil, fmt.Errorf("create advancement runtime: nil resource manager")
	}
	runtime := &Runtime{resources: resources}
	if err := resources.RegisterCompiler(compiledAdvancementsKey, compileDefinitions); err != nil {
		return nil, err
	}
	return runtime, nil
}

// Count returns the number of active advancement definitions.
func (r *Runtime) Count() int {
	set := r.current()
	if set == nil {
		return 0
	}
	return len(set.definitions)
}

// Definition returns an immutable active definition.
func (r *Runtime) Definition(id string) (*Definition, bool) {
	set := r.current()
	if set == nil {
		return nil, false
	}
	definition, ok := set.definitions[id]
	return definition, ok
}

// IDs returns advancement identifiers in parent first order.
func (r *Runtime) IDs() []string {
	set := r.current()
	if set == nil {
		return nil
	}
	return append([]string(nil), set.ordered...)
}

// NewProgress creates empty progress for one player.
func NewProgress() *Progress {
	return &Progress{granted: make(map[string]map[string]time.Time)}
}

// Grant marks one criterion and reports whether state and completion changed.
func (p *Progress) Grant(definition *Definition, criterion string, at time.Time) (bool, bool, error) {
	if p == nil || definition == nil {
		return false, false, fmt.Errorf("grant advancement criterion: nil state")
	}
	if _, ok := definition.Criteria[criterion]; !ok {
		return false, false, fmt.Errorf("advancement %s has no criterion %q", definition.ID, criterion)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	criteria := p.granted[definition.ID]
	if criteria == nil {
		criteria = make(map[string]time.Time)
		p.granted[definition.ID] = criteria
	}
	if _, exists := criteria[criterion]; exists {
		return false, requirementsComplete(definition.Requirements, criteria), nil
	}
	criteria[criterion] = at
	return true, requirementsComplete(definition.Requirements, criteria), nil
}

// IsComplete reports whether all requirement groups are satisfied.
func (p *Progress) IsComplete(definition *Definition) bool {
	if p == nil || definition == nil {
		return false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return requirementsComplete(definition.Requirements, p.granted[definition.ID])
}

// current returns the active compiled definition set.
func (r *Runtime) current() *DefinitionSet {
	if r == nil || r.resources == nil {
		return nil
	}
	compiled, ok := r.resources.Current().Compiled(compiledAdvancementsKey)
	if !ok {
		return nil
	}
	set, _ := compiled.(*DefinitionSet)
	return set
}

// compileDefinitions validates resources and computes parent first ordering.
func compileDefinitions(snapshot *datapack.Snapshot) (any, error) {
	set := &DefinitionSet{definitions: make(map[string]*Definition)}
	triggers := make(map[string]bool, len(genreg.TriggerType))
	for _, trigger := range genreg.TriggerType {
		triggers[trigger] = true
	}
	for _, name := range snapshot.Names() {
		id, ok := advancementIDFromPath(name)
		if !ok {
			continue
		}
		data, _ := snapshot.Get(name)
		definition := &Definition{ID: id}
		if err := json.Unmarshal(data, definition); err != nil {
			return nil, fmt.Errorf("advancement %s: %w", id, err)
		}
		if len(definition.Criteria) == 0 {
			return nil, fmt.Errorf("advancement %s: no criteria", id)
		}
		for criterion, value := range definition.Criteria {
			if criterion == "" || !triggers[value.Trigger] {
				return nil, fmt.Errorf("advancement %s criterion %q: unsupported trigger %q", id, criterion, value.Trigger)
			}
		}
		if len(definition.Requirements) == 0 {
			criteria := make([]string, 0, len(definition.Criteria))
			for criterion := range definition.Criteria {
				criteria = append(criteria, criterion)
			}
			sort.Strings(criteria)
			for _, criterion := range criteria {
				definition.Requirements = append(definition.Requirements, []string{criterion})
			}
		}
		if err := validateRequirements(definition); err != nil {
			return nil, err
		}
		set.definitions[id] = definition
	}
	state := make(map[string]uint8, len(set.definitions))
	ids := make([]string, 0, len(set.definitions))
	for id := range set.definitions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var visit func(string) error
	visit = func(id string) error {
		switch state[id] {
		case 1:
			return fmt.Errorf("advancement parent cycle at %s", id)
		case 2:
			return nil
		}
		state[id] = 1
		definition := set.definitions[id]
		if definition.Parent != "" {
			if _, ok := set.definitions[definition.Parent]; !ok {
				return fmt.Errorf("advancement %s: missing parent %s", id, definition.Parent)
			}
			if err := visit(definition.Parent); err != nil {
				return err
			}
		}
		state[id] = 2
		set.ordered = append(set.ordered, id)
		return nil
	}
	for _, id := range ids {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	return set, nil
}

// validateRequirements ensures every criterion appears exactly once.
func validateRequirements(definition *Definition) error {
	seen := make(map[string]bool, len(definition.Criteria))
	for groupIndex, group := range definition.Requirements {
		if len(group) == 0 {
			return fmt.Errorf("advancement %s: empty requirement group %d", definition.ID, groupIndex)
		}
		for _, criterion := range group {
			if _, ok := definition.Criteria[criterion]; !ok {
				return fmt.Errorf("advancement %s: unknown required criterion %q", definition.ID, criterion)
			}
			if seen[criterion] {
				return fmt.Errorf("advancement %s: duplicate required criterion %q", definition.ID, criterion)
			}
			seen[criterion] = true
		}
	}
	if len(seen) != len(definition.Criteria) {
		return fmt.Errorf("advancement %s: requirements omit criteria", definition.ID)
	}
	return nil
}

// requirementsComplete evaluates AND groups containing OR criteria.
func requirementsComplete(requirements [][]string, granted map[string]time.Time) bool {
	if len(requirements) == 0 {
		return false
	}
	for _, group := range requirements {
		matched := false
		for _, criterion := range group {
			if _, ok := granted[criterion]; ok {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// advancementIDFromPath converts a canonical data path to a resource location.
func advancementIDFromPath(name string) (string, bool) {
	parts := strings.Split(name, "/")
	if len(parts) < 4 || parts[0] != "data" || parts[2] != "advancement" || !strings.HasSuffix(parts[len(parts)-1], ".json") {
		return "", false
	}
	resourcePath := strings.TrimSuffix(strings.Join(parts[3:], "/"), ".json")
	if parts[1] == "" || resourcePath == "" {
		return "", false
	}
	return parts[1] + ":" + resourcePath, true
}
