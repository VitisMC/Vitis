package world

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/vitismc/vitis/internal/tick"
)

// Manager owns lifecycle and lookup of all worlds.
type Manager struct {
	mu sync.RWMutex

	worldsByName map[string]*World
	ordered      []*World

	nextWorldID atomic.Uint64
}

// NewManager creates an empty world manager.
func NewManager() *Manager {
	return &Manager{worldsByName: make(map[string]*World)}
}

// Create creates and registers a new world with defaults.
func (m *Manager) Create(name string) (*World, error) {
	return m.CreateWithConfig(Config{Name: name})
}

// CreateWithConfig creates and registers a new world from config.
func (m *Manager) CreateWithConfig(config Config) (*World, error) {
	if m == nil {
		return nil, fmt.Errorf("create world: nil manager")
	}
	if config.Name == "" {
		return nil, fmt.Errorf("create world: empty world name")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.worldsByName[config.Name]; exists {
		return nil, fmt.Errorf("create world name=%q: already exists", config.Name)
	}

	if config.ID == 0 {
		config.ID = m.nextWorldID.Add(1)
	}

	world, err := New(config)
	if err != nil {
		return nil, err
	}

	m.worldsByName[config.Name] = world
	m.ordered = append(m.ordered, world)
	return world, nil
}

// Get returns one world by name.
func (m *Manager) Get(name string) (*World, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	world, ok := m.worldsByName[name]
	return world, ok
}

// Remove removes one world by name.
func (m *Manager) Remove(name string) bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	world, ok := m.worldsByName[name]
	if !ok {
		return false
	}
	delete(m.worldsByName, name)

	for index := range m.ordered {
		if m.ordered[index] == world {
			last := len(m.ordered) - 1
			m.ordered[index] = m.ordered[last]
			m.ordered[last] = nil
			m.ordered = m.ordered[:last]
			break
		}
	}
	return true
}

// Tick advances manager-level work.
func (m *Manager) Tick() {
	if m == nil {
		return
	}
}

// Worlds returns a stable snapshot for tick loop traversal.
func (m *Manager) Worlds() []tick.WorldTicker {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]tick.WorldTicker, len(m.ordered))
	for index := range m.ordered {
		result[index] = m.ordered[index]
	}
	return result
}

// Count returns number of registered worlds.
func (m *Manager) Count() int {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.worldsByName)
}

// Close closes all registered worlds.
func (m *Manager) Close(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	worlds := m.snapshotWorlds()
	var closeErr error
	for index := range worlds {
		if worlds[index] == nil {
			continue
		}
		if err := worlds[index].Close(ctx); err != nil {
			closeErr = errors.Join(closeErr, err)
		}
	}
	return closeErr
}

func (m *Manager) snapshotWorlds() []*World {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	worlds := make([]*World, len(m.ordered))
	copy(worlds, m.ordered)
	return worlds
}
