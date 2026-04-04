package loot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Manager struct {
	mu          sync.RWMutex
	blockTables map[string]*Table
	dataDir     string
}

func NewManager(dataDir string) *Manager {
	return &Manager{
		blockTables: make(map[string]*Table),
		dataDir:     dataDir,
	}
}

func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	blockDir := filepath.Join(m.dataDir, "loot_tables", "blocks")
	entries, err := os.ReadDir(blockDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		name := "minecraft:" + strings.TrimSuffix(entry.Name(), ".json")
		table, err := m.loadTable(filepath.Join(blockDir, entry.Name()))
		if err != nil {
			continue
		}

		m.blockTables[name] = table
	}

	return nil
}

func (m *Manager) loadTable(path string) (*Table, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var table Table
	if err := json.Unmarshal(data, &table); err != nil {
		return nil, err
	}

	return &table, nil
}

func (m *Manager) GetBlockTable(blockName string) *Table {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.blockTables[blockName]
}

func (m *Manager) BlockTableCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.blockTables)
}

func (m *Manager) GetBlockDrops(blockName string, ctx *Context) []Drop {
	table := m.GetBlockTable(blockName)
	if table == nil {
		return []Drop{{ItemID: blockName, Count: 1}}
	}
	return table.GetDrops(ctx)
}
