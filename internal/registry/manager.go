package registry

import (
	"fmt"
	"strings"

	genreg "github.com/vitismc/vitis/internal/data/generated/registry"
)

// Manager holds all Minecraft 1.21.4 registries and provides fast lookup.
// It is initialized once at startup and is immutable thereafter.
type Manager struct {
	registries map[string]*IDRegistry
	configData map[string][]ConfigEntry
}

// ConfigEntry holds a single configuration registry entry with pre-encoded NBT.
type ConfigEntry struct {
	Name string
	Data []byte
}

// NewManager builds the global registry manager from generated data.
func NewManager() (*Manager, error) {
	m := &Manager{
		registries: make(map[string]*IDRegistry, len(genreg.AllRegistryNames)+len(genreg.ConfigRegistryNames)),
		configData: make(map[string][]ConfigEntry, len(genreg.ConfigRegistryNames)),
	}

	if err := m.loadBuiltinRegistries(); err != nil {
		return nil, fmt.Errorf("load builtin registries: %w", err)
	}

	if err := m.loadConfigRegistries(); err != nil {
		return nil, fmt.Errorf("load config registries: %w", err)
	}

	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	return m, nil
}

func (m *Manager) loadBuiltinRegistries() error {
	idMap := builtinIDMap()
	for name, ids := range idMap {
		reg := NewIDRegistry(name, ids)
		m.registries[name] = reg
	}
	return nil
}

func (m *Manager) loadConfigRegistries() error {
	nbtData := genreg.ConfigRegistryData()

	for _, regName := range genreg.ConfigRegistryNames {
		entries, ok := nbtData[regName]
		if !ok {
			return fmt.Errorf("missing config NBT data for %q", regName)
		}

		names := make([]string, len(entries))
		configEntries := make([]ConfigEntry, len(entries))
		for i, e := range entries {
			names[i] = e.Name
			configEntries[i] = ConfigEntry{Name: e.Name, Data: e.Data}
		}

		if _, exists := m.registries[regName]; !exists {
			m.registries[regName] = NewIDRegistry(regName, names)
		}

		m.configData[regName] = configEntries
	}
	return nil
}

// Registry returns the IDRegistry for the given registry key (e.g. "minecraft:block").
func (m *Manager) Registry(name string) *IDRegistry {
	return m.registries[name]
}

// ConfigRegistryNames returns the ordered list of configuration registry keys.
func (m *Manager) ConfigRegistryNames() []string {
	return genreg.ConfigRegistryNames
}

// ConfigEntries returns the configuration entries with pre-encoded NBT for a registry.
func (m *Manager) ConfigEntries(registryName string) []ConfigEntry {
	return m.configData[registryName]
}

// IDByName looks up a protocol ID in any registry. Returns -1 if not found.
func (m *Manager) IDByName(registryName, entryName string) int32 {
	reg := m.registries[registryName]
	if reg == nil {
		return -1
	}
	return reg.IDByName(entryName)
}

// NameByID looks up a name in any registry. Returns "" if not found.
func (m *Manager) NameByID(registryName string, id int32) string {
	reg := m.registries[registryName]
	if reg == nil {
		return ""
	}
	return reg.NameByID(id)
}

// Validate checks all registries for internal consistency.
func (m *Manager) Validate() error {
	var errs []string
	for name, reg := range m.registries {
		if err := reg.Validate(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}

	for _, regName := range genreg.ConfigRegistryNames {
		entries := m.configData[regName]
		if len(entries) == 0 {
			errs = append(errs, fmt.Sprintf("config registry %q has no entries", regName))
		}
		for i, e := range entries {
			if len(e.Data) == 0 {
				errs = append(errs, fmt.Sprintf("config registry %q entry %d (%s) has empty NBT", regName, i, e.Name))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation failed:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// RegistryCount returns the total number of registries loaded.
func (m *Manager) RegistryCount() int {
	return len(m.registries)
}

// ConfigRegistryCount returns the number of configuration registries with NBT data.
func (m *Manager) ConfigRegistryCount() int {
	return len(m.configData)
}
