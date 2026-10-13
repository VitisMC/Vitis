package loot

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
)

const compiledLootTablesKey = "loot_tables"

// Runtime exposes loot tables from the current atomic resource snapshot.
type Runtime struct {
	resources *datapack.Manager
}

// TableSet contains all active loot tables indexed by resource location.
type TableSet struct {
	tables   map[string]*Table
	smelting map[string]string
}

// NewRuntime registers loot table compilation with the resource manager.
func NewRuntime(resources *datapack.Manager) (*Runtime, error) {
	if resources == nil {
		return nil, fmt.Errorf("create loot runtime: nil resource manager")
	}
	runtime := &Runtime{resources: resources}
	if err := resources.RegisterCompiler(compiledLootTablesKey, compileTables); err != nil {
		return nil, err
	}
	return runtime, nil
}

// Table returns an immutable table from the active resource snapshot.
func (r *Runtime) Table(id string) *Table {
	set := r.current()
	if set == nil {
		return nil
	}
	return set.tables[id]
}

// GetBlockTable returns a block loot table from the active resource snapshot.
func (r *Runtime) GetBlockTable(blockName string) *Table {
	id := strings.TrimPrefix(blockName, "minecraft:")
	return r.Table("minecraft:blocks/" + id)
}

// GetBlockDrops evaluates a block table or returns the block as a safe fallback.
func (r *Runtime) GetBlockDrops(blockName string, ctx *Context) []Drop {
	table := r.GetBlockTable(blockName)
	if table == nil {
		return []Drop{{ItemID: blockName, Count: 1}}
	}
	return table.GetDrops(ctx)
}

// Roll evaluates a named loot table with nested-table and item-tag resolution.
func (r *Runtime) Roll(tableID string, ctx *Context) []Drop {
	drops := r.roll(tableID, ctx, true)
	for index := range drops {
		drops[index].Stack = inventory.ItemStack{}
	}
	return drops
}

// RollStacks evaluates complete stacks while retaining the legacy drop adapter.
func (r *Runtime) RollStacks(tableID string, ctx *Context) []inventory.ItemStack {
	var stacks []inventory.ItemStack
	for _, drop := range r.roll(tableID, ctx, true) {
		stack := drop.Stack
		if stack.Empty() {
			stack = inventory.NewSlot(item.IDByName(drop.ItemID), int32(drop.Count))
		}
		if stack.Empty() || stack.ItemID <= 0 {
			continue
		}
		for stack.ItemCount > 0 {
			part := stack.Clone()
			part.ItemCount = min(stack.ItemCount, stack.MaxStackSize())
			if part.ItemCount <= 0 {
				break
			}
			stacks = append(stacks, part)
			stack.ItemCount -= part.ItemCount
		}
	}
	return stacks
}

// roll resolves all dependencies from one immutable resource snapshot.
func (r *Runtime) roll(tableID string, ctx *Context, full bool) []Drop {
	if ctx == nil {
		return nil
	}
	if r == nil || r.resources == nil {
		return nil
	}
	snapshot := r.resources.Current()
	if snapshot == nil {
		return nil
	}
	compiled, _ := snapshot.Compiled(compiledLootTablesKey)
	set, _ := compiled.(*TableSet)
	if set == nil {
		return nil
	}
	table := set.tables[tableID]
	if table == nil {
		return nil
	}
	resolved := *ctx
	resolved.fullStacks = full
	resolved.tableResolver = func(id string, child *Context) []Drop {
		nested := set.tables[id]
		if nested == nil {
			return nil
		}
		return nested.GetDrops(child)
	}
	resolved.tagResolver = func(id string) []string {
		values, _ := snapshot.ResolveTag("item", id)
		return values
	}
	resolved.registryTagResolver = func(registry, id string) []string { values, _ := snapshot.ResolveTag(registry, id); return values }
	resolved.smeltResolver = func(id string) string { return set.smelting[id] }
	return table.GetDrops(&resolved)
}

// TableCount returns the number of active loot tables.
func (r *Runtime) TableCount() int {
	set := r.current()
	if set == nil {
		return 0
	}
	return len(set.tables)
}

// current returns the compiled tables from the current snapshot.
func (r *Runtime) current() *TableSet {
	if r == nil || r.resources == nil {
		return nil
	}
	compiled, ok := r.resources.Current().Compiled(compiledLootTablesKey)
	if !ok {
		return nil
	}
	set, _ := compiled.(*TableSet)
	return set
}

// compileTables validates and indexes all loot table resources.
func compileTables(snapshot *datapack.Snapshot) (any, error) {
	set := &TableSet{tables: make(map[string]*Table), smelting: make(map[string]string)}
	for _, name := range snapshot.Names() {
		if strings.Contains(name, "/recipe/") && strings.HasSuffix(name, ".json") {
			data, _ := snapshot.Get(name)
			var recipe struct {
				Type       string `json:"type"`
				Ingredient string `json:"ingredient"`
				Result     struct {
					ID string `json:"id"`
				} `json:"result"`
			}
			if json.Unmarshal(data, &recipe) == nil && recipe.Type == "minecraft:smelting" && item.IDByName(recipe.Ingredient) > 0 && item.IDByName(recipe.Result.ID) > 0 {
				set.smelting[recipe.Ingredient] = recipe.Result.ID
			}
		}
		id, ok := lootTableIDFromPath(name)
		if !ok {
			continue
		}
		data, _ := snapshot.Get(name)
		var table Table
		if err := json.Unmarshal(data, &table); err != nil {
			return nil, fmt.Errorf("loot table %s: %w", id, err)
		}
		if !knownTableTypes[table.Type] {
			return nil, fmt.Errorf("loot table %s: unsupported type %q", id, table.Type)
		}
		if err := validatePools(snapshot, id, table.Pools); err != nil {
			return nil, err
		}
		if err := validateFunctionDamageConditions(snapshot, table.Functions); err != nil {
			return nil, fmt.Errorf("loot table %s: %w", id, err)
		}
		copy := table
		set.tables[id] = &copy
	}
	return set, nil
}

// validatePools validates entries that identify items or item tags.
func validatePools(snapshot *datapack.Snapshot, tableID string, pools []Pool) error {
	for poolIndex := range pools {
		if err := validateDamageConditions(snapshot, pools[poolIndex].Conditions); err != nil {
			return fmt.Errorf("loot table %s pool %d: %w", tableID, poolIndex, err)
		}
		if err := validateFunctionDamageConditions(snapshot, pools[poolIndex].Functions); err != nil {
			return fmt.Errorf("loot table %s pool %d: %w", tableID, poolIndex, err)
		}
		if len(pools[poolIndex].Entries) == 0 {
			return fmt.Errorf("loot table %s pool %d: no entries", tableID, poolIndex)
		}
		for entryIndex := range pools[poolIndex].Entries {
			if err := validateEntry(snapshot, &pools[poolIndex].Entries[entryIndex]); err != nil {
				return fmt.Errorf("loot table %s pool %d entry %d: %w", tableID, poolIndex, entryIndex, err)
			}
		}
	}
	return nil
}

// validateEntry validates one entry tree.
func validateEntry(snapshot *datapack.Snapshot, entry *Entry) error {
	if err := validateDamageConditions(snapshot, entry.Conditions); err != nil {
		return err
	}
	if err := validateFunctionDamageConditions(snapshot, entry.Functions); err != nil {
		return err
	}
	if !knownEntryTypes[EntryType(entry.Type)] {
		return fmt.Errorf("unsupported entry type %q", entry.Type)
	}
	switch EntryType(entry.Type) {
	case EntryTypeItem:
		if item.IDByName(entry.Name) <= 0 {
			return fmt.Errorf("unknown item %q", entry.Name)
		}
	case EntryTypeTag:
		if _, err := snapshot.ResolveTag("item", entry.Name); err != nil {
			return fmt.Errorf("item tag %q: %w", entry.Name, err)
		}
	case EntryTypeLootTable:
		if entry.Value.Inline != nil {
			if !knownTableTypes[entry.Value.Inline.Type] && entry.Value.Inline.Type != "" {
				return fmt.Errorf("unsupported inline table type %q", entry.Value.Inline.Type)
			}
			if err := validatePools(snapshot, "<inline>", entry.Value.Inline.Pools); err != nil {
				return err
			}
			if err := validateFunctionDamageConditions(snapshot, entry.Value.Inline.Functions); err != nil {
				return err
			}
			break
		}
		if entry.Value.ID == "" {
			return fmt.Errorf("empty nested loot table")
		}
		namespace, local := splitResourceLocation(entry.Value.ID)
		if _, ok := snapshot.Get("data/" + namespace + "/loot_table/" + local + ".json"); !ok {
			return fmt.Errorf("missing nested loot table %q", entry.Value.ID)
		}
	}
	for index := range entry.Children {
		if err := validateEntry(snapshot, &entry.Children[index]); err != nil {
			return fmt.Errorf("child %d: %w", index, err)
		}
	}
	return nil
}

// splitResourceLocation applies the default minecraft namespace.
func splitResourceLocation(id string) (string, string) {
	if namespace, local, ok := strings.Cut(id, ":"); ok {
		return namespace, local
	}
	return "minecraft", id
}

// lootTableIDFromPath converts a canonical data path to a resource location.
func lootTableIDFromPath(name string) (string, bool) {
	parts := strings.Split(name, "/")
	if len(parts) < 4 || parts[0] != "data" || parts[2] != "loot_table" || !strings.HasSuffix(parts[len(parts)-1], ".json") {
		return "", false
	}
	resourcePath := strings.TrimSuffix(strings.Join(parts[3:], "/"), ".json")
	if parts[1] == "" || resourcePath == "" {
		return "", false
	}
	return parts[1] + ":" + resourcePath, true
}

var knownTableTypes = map[TableType]bool{
	"minecraft:archaeology": true,
	"minecraft:barter":      true,
	"minecraft:block":       true,
	"minecraft:chest":       true,
	"minecraft:entity":      true,
	"minecraft:equipment":   true,
	"minecraft:fishing":     true,
	"minecraft:gift":        true,
	"minecraft:shearing":    true,
}

var knownEntryTypes = map[EntryType]bool{
	EntryTypeItem:         true,
	EntryTypeAlternatives: true,
	EntryTypeSequence:     true,
	EntryTypeGroup:        true,
	EntryTypeEmpty:        true,
	EntryTypeLootTable:    true,
	EntryTypeTag:          true,
	EntryTypeDynamic:      true,
}
