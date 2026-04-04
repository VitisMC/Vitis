package operator

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/vitismc/vitis/internal/protocol"
)

// Operator represents a server operator entry.
type Operator struct {
	UUID                protocol.UUID `json:"uuid"`
	Name                string        `json:"name"`
	Level               int           `json:"level"`
	BypassesPlayerLimit bool          `json:"bypassesPlayerLimit"`
}

// List manages the list of server operators with persistence to ops.json.
type List struct {
	mu       sync.RWMutex
	ops      map[protocol.UUID]*Operator
	filePath string
}

// NewList creates an operator list that persists to the given file path.
func NewList(filePath string) *List {
	return &List{
		ops:      make(map[protocol.UUID]*Operator),
		filePath: filePath,
	}
}

// Load reads the operator list from the JSON file.
// If the file does not exist, the list starts empty.
func (l *List) Load() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := os.ReadFile(l.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var entries []Operator
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	l.ops = make(map[protocol.UUID]*Operator, len(entries))
	for i := range entries {
		l.ops[entries[i].UUID] = &entries[i]
	}
	return nil
}

// Save writes the operator list to the JSON file.
func (l *List) Save() error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	entries := make([]Operator, 0, len(l.ops))
	for _, op := range l.ops {
		entries = append(entries, *op)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(l.filePath, data, 0644)
}

// Add adds or updates an operator entry and saves to disk.
func (l *List) Add(op Operator) error {
	if op.Level < 1 {
		op.Level = 1
	}
	if op.Level > 4 {
		op.Level = 4
	}

	l.mu.Lock()
	l.ops[op.UUID] = &op
	l.mu.Unlock()

	return l.Save()
}

// Remove removes an operator by UUID and saves to disk.
func (l *List) Remove(uuid protocol.UUID) error {
	l.mu.Lock()
	delete(l.ops, uuid)
	l.mu.Unlock()

	return l.Save()
}

// IsOp returns true if the given UUID is an operator.
func (l *List) IsOp(uuid protocol.UUID) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	_, ok := l.ops[uuid]
	return ok
}

// GetLevel returns the permission level for the given UUID.
// Returns 0 if the player is not an operator.
func (l *List) GetLevel(uuid protocol.UUID) int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if op, ok := l.ops[uuid]; ok {
		return op.Level
	}
	return 0
}

// Get returns the operator entry for the given UUID, or nil.
func (l *List) Get(uuid protocol.UUID) *Operator {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if op, ok := l.ops[uuid]; ok {
		cpy := *op
		return &cpy
	}
	return nil
}

// Count returns the number of operators.
func (l *List) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.ops)
}

// All returns a copy of all operator entries.
func (l *List) All() []Operator {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]Operator, 0, len(l.ops))
	for _, op := range l.ops {
		result = append(result, *op)
	}
	return result
}

// BypassesPlayerLimit returns true if the given UUID can bypass the player limit.
func (l *List) BypassesPlayerLimit(uuid protocol.UUID) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if op, ok := l.ops[uuid]; ok {
		return op.BypassesPlayerLimit
	}
	return false
}
