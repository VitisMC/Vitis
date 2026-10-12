package data

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/storage"
)

// WhitelistEntry represents a single whitelisted player.
type WhitelistEntry struct {
	UUID protocol.UUID `json:"uuid"`
	Name string        `json:"name"`
}

// Whitelist manages a list of whitelisted players with JSON persistence.
type Whitelist struct {
	mu       sync.RWMutex
	entries  map[protocol.UUID]*WhitelistEntry
	filePath string
	enabled  bool
}

// NewWhitelist creates a whitelist that persists to the given file path.
func NewWhitelist(filePath string) *Whitelist {
	return &Whitelist{
		entries:  make(map[protocol.UUID]*WhitelistEntry),
		filePath: filePath,
	}
}

// Load reads the whitelist from the JSON file.
func (w *Whitelist) Load() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	data, err := os.ReadFile(w.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var entries []WhitelistEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	w.entries = make(map[protocol.UUID]*WhitelistEntry, len(entries))
	for i := range entries {
		w.entries[entries[i].UUID] = &entries[i]
	}
	return nil
}

// Save writes the whitelist to the JSON file.
func (w *Whitelist) Save() error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	entries := make([]WhitelistEntry, 0, len(w.entries))
	for _, e := range w.entries {
		entries = append(entries, *e)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	return storage.WriteFileAtomic(w.filePath, data, 0644)
}

// Add adds a player to the whitelist and saves to disk.
// Returns true if the player was newly added, false if already present.
func (w *Whitelist) Add(uuid protocol.UUID, name string) (bool, error) {
	w.mu.Lock()
	if _, exists := w.entries[uuid]; exists {
		w.mu.Unlock()
		return false, nil
	}
	w.entries[uuid] = &WhitelistEntry{UUID: uuid, Name: name}
	w.mu.Unlock()

	return true, w.Save()
}

// Remove removes a player from the whitelist and saves to disk.
// Returns true if the player was removed, false if not found.
func (w *Whitelist) Remove(uuid protocol.UUID) (bool, error) {
	w.mu.Lock()
	if _, exists := w.entries[uuid]; !exists {
		w.mu.Unlock()
		return false, nil
	}
	delete(w.entries, uuid)
	w.mu.Unlock()

	return true, w.Save()
}

// RemoveByName removes a player from the whitelist by name.
// Returns true if the player was removed, false if not found.
func (w *Whitelist) RemoveByName(name string) (bool, error) {
	w.mu.Lock()
	var found protocol.UUID
	ok := false
	for uuid, e := range w.entries {
		if e.Name == name {
			found = uuid
			ok = true
			break
		}
	}
	if !ok {
		w.mu.Unlock()
		return false, nil
	}
	delete(w.entries, found)
	w.mu.Unlock()

	return true, w.Save()
}

// IsWhitelisted returns true if the given UUID is on the whitelist.
func (w *Whitelist) IsWhitelisted(uuid protocol.UUID) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, ok := w.entries[uuid]
	return ok
}

// Enabled returns whether the whitelist is actively enforced.
func (w *Whitelist) Enabled() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.enabled
}

// SetEnabled enables or disables whitelist enforcement.
func (w *Whitelist) SetEnabled(v bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.enabled = v
}

// Count returns the number of whitelisted players.
func (w *Whitelist) Count() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.entries)
}

// All returns a copy of all whitelist entries.
func (w *Whitelist) All() []WhitelistEntry {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make([]WhitelistEntry, 0, len(w.entries))
	for _, e := range w.entries {
		result = append(result, *e)
	}
	return result
}

// Names returns all whitelisted player names.
func (w *Whitelist) Names() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	names := make([]string, 0, len(w.entries))
	for _, e := range w.entries {
		names = append(names, e.Name)
	}
	return names
}
