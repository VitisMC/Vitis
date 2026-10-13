package inventory

import (
	_ "embed"
	"encoding/json"
	"sync"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
)

//go:embed item_defaults.json
var defaultItemJSON []byte

// defaultDefinition contains immutable persistent component defaults.
type defaultDefinition struct{ Components map[string]json.RawMessage }

// defaultComponentKey identifies a component in the lazy immutable default cache.
type defaultComponentKey struct {
	ItemID int32
	TypeID int32
}

// cachedDefault retains either a validated default or a missing component.
type cachedDefault struct{ Value ComponentValue }

var defaultDefinitions = loadDefaultDefinitions()
var defaultComponentCache sync.Map

// loadDefaultDefinitions loads pinned item defaults without disk access at runtime.
func loadDefaultDefinitions() map[string]defaultDefinition {
	var result map[string]defaultDefinition
	if err := json.Unmarshal(defaultItemJSON, &result); err != nil {
		panic(err)
	}
	return result
}

// defaultItemComponent resolves and caches an immutable vanilla item default.
func defaultItemComponent(itemID, typeID int32) (ComponentValue, bool) {
	key := defaultComponentKey{itemID, typeID}
	if cached, ok := defaultComponentCache.Load(key); ok {
		v := cached.(cachedDefault).Value
		return cloneComponentValue(v), v != nil
	}
	raw, ok := defaultDefinitions[item.NameByID(itemID)].Components[dc.DataComponentNameByID(typeID)]
	if !ok {
		defaultComponentCache.Store(key, cachedDefault{})
		return nil, false
	}
	tag, value, err := nbt.FromJSON(raw)
	if err != nil {
		return nil, false
	}
	component, err := componentFromNBT(typeID, tag, value, 0)
	if err != nil {
		return nil, false
	}
	defaultComponentCache.Store(key, cachedDefault{component})
	return cloneComponentValue(component), component != nil
}
