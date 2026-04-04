package config

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// ReloadHook is called during reload before the new config is published.
type ReloadHook func(oldConfig *Config, newConfig *Config) error

// Manager stores the active configuration and supports hot reload.
type Manager struct {
	loader *Loader

	current atomic.Pointer[Config]
	applyMu sync.Mutex

	hooksMu sync.RWMutex
	hooks   []ReloadHook
}

var fallbackConfig = Default()

var globalManager = NewManager(DefaultConfigPath)

// NewManager creates a configuration manager bound to the provided file path.
func NewManager(path string) *Manager {
	m := &Manager{loader: NewLoader(path)}
	m.current.Store(Default())
	return m
}

// Load loads configuration from file and atomically publishes it.
func (m *Manager) Load() (*Config, error) {
	if m == nil {
		return nil, fmt.Errorf("load config manager: nil manager")
	}
	cfg, err := m.loader.Load()
	if err != nil {
		return nil, err
	}
	return m.apply(cfg)
}

// Reload reloads configuration from the same file and atomically publishes it.
func (m *Manager) Reload() (*Config, error) {
	if m == nil {
		return nil, fmt.Errorf("reload config manager: nil manager")
	}
	cfg, err := m.loader.Reload()
	if err != nil {
		return nil, err
	}
	return m.apply(cfg)
}

// RegisterReloadHook registers a callback to be invoked on each successful parse before publish.
func (m *Manager) RegisterReloadHook(hook ReloadHook) error {
	if m == nil {
		return fmt.Errorf("register reload hook: nil manager")
	}
	if hook == nil {
		return fmt.Errorf("register reload hook: nil hook")
	}

	m.hooksMu.Lock()
	m.hooks = append(m.hooks, hook)
	m.hooksMu.Unlock()
	return nil
}

// Get returns the currently active configuration pointer.
func (m *Manager) Get() *Config {
	if m == nil {
		return fallbackConfig
	}
	cfg := m.current.Load()
	if cfg == nil {
		return fallbackConfig
	}
	return cfg
}

// GetNetwork returns the currently active network section.
func (m *Manager) GetNetwork() NetworkConfig {
	return m.Get().Network
}

// GetServer returns the currently active server section.
func (m *Manager) GetServer() ServerConfig {
	return m.Get().Server
}

// GetTick returns the currently active tick section.
func (m *Manager) GetTick() TickConfig {
	return m.Get().Tick
}

// GetWorld returns the currently active world section.
func (m *Manager) GetWorld() WorldConfig {
	return m.Get().World
}

// GetLogging returns the currently active logging section.
func (m *Manager) GetLogging() LoggingConfig {
	return m.Get().Logging
}

// GetPerformance returns the currently active performance section.
func (m *Manager) GetPerformance() PerformanceConfig {
	return m.Get().Performance
}

// Path returns the underlying configuration file path.
func (m *Manager) Path() string {
	if m == nil {
		return DefaultConfigPath
	}
	return m.loader.Path()
}

// Global returns the default global configuration manager.
func Global() *Manager {
	return globalManager
}

// Get returns active configuration from the global manager.
func Get() *Config {
	return globalManager.Get()
}

// GetNetwork returns active network configuration from the global manager.
func GetNetwork() NetworkConfig {
	return globalManager.GetNetwork()
}

// Reload reloads configuration on the global manager.
func Reload() (*Config, error) {
	return globalManager.Reload()
}

func (m *Manager) apply(cfg *Config) (*Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("apply config: nil config")
	}

	m.applyMu.Lock()
	defer m.applyMu.Unlock()

	old := m.current.Load()
	hooks := m.snapshotHooks()

	var hookErr error
	for _, hook := range hooks {
		if err := hook(old, cfg); err != nil {
			hookErr = errors.Join(hookErr, err)
		}
	}
	if hookErr != nil {
		return nil, fmt.Errorf("apply reload hooks: %w", hookErr)
	}

	m.current.Store(cfg)
	return cfg, nil
}

func (m *Manager) snapshotHooks() []ReloadHook {
	m.hooksMu.RLock()
	defer m.hooksMu.RUnlock()
	if len(m.hooks) == 0 {
		return nil
	}
	hooks := make([]ReloadHook, len(m.hooks))
	copy(hooks, m.hooks)
	return hooks
}
