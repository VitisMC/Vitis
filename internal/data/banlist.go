package data

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/vitismc/vitis/internal/protocol"
)

// BanEntry represents a single ban record.
type BanEntry struct {
	UUID    protocol.UUID `json:"uuid"`
	Name    string        `json:"name"`
	Reason  string        `json:"reason"`
	Source  string        `json:"source"`
	Created string        `json:"created"`
	Expires string        `json:"expires"`
}

// IsExpired returns true if the ban has an expiry and it has passed.
func (b *BanEntry) IsExpired() bool {
	if b.Expires == "" || b.Expires == "forever" {
		return false
	}
	t, err := time.Parse(time.RFC3339, b.Expires)
	if err != nil {
		return false
	}
	return time.Now().After(t)
}

// BanList manages banned players with JSON persistence.
type BanList struct {
	mu       sync.RWMutex
	entries  map[protocol.UUID]*BanEntry
	filePath string
}

// NewBanList creates a ban list that persists to the given file path.
func NewBanList(filePath string) *BanList {
	return &BanList{
		entries:  make(map[protocol.UUID]*BanEntry),
		filePath: filePath,
	}
}

// Load reads the ban list from the JSON file.
func (bl *BanList) Load() error {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	data, err := os.ReadFile(bl.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var entries []BanEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	bl.entries = make(map[protocol.UUID]*BanEntry, len(entries))
	for i := range entries {
		bl.entries[entries[i].UUID] = &entries[i]
	}
	return nil
}

// Save writes the ban list to the JSON file.
func (bl *BanList) Save() error {
	bl.mu.RLock()
	defer bl.mu.RUnlock()

	entries := make([]BanEntry, 0, len(bl.entries))
	for _, e := range bl.entries {
		entries = append(entries, *e)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(bl.filePath, data, 0644)
}

// Ban adds a player ban and saves to disk.
func (bl *BanList) Ban(uuid protocol.UUID, name, reason, source string, expires string) error {
	bl.mu.Lock()
	bl.entries[uuid] = &BanEntry{
		UUID:    uuid,
		Name:    name,
		Reason:  reason,
		Source:  source,
		Created: time.Now().Format(time.RFC3339),
		Expires: expires,
	}
	bl.mu.Unlock()

	return bl.Save()
}

// Pardon removes a player ban by name and saves to disk.
// Returns true if a ban was removed.
func (bl *BanList) Pardon(name string) (bool, error) {
	bl.mu.Lock()
	var found protocol.UUID
	ok := false
	for uuid, e := range bl.entries {
		if strings.EqualFold(e.Name, name) {
			found = uuid
			ok = true
			break
		}
	}
	if !ok {
		bl.mu.Unlock()
		return false, nil
	}
	delete(bl.entries, found)
	bl.mu.Unlock()

	return true, bl.Save()
}

// PardonByUUID removes a player ban by UUID and saves to disk.
func (bl *BanList) PardonByUUID(uuid protocol.UUID) (bool, error) {
	bl.mu.Lock()
	if _, exists := bl.entries[uuid]; !exists {
		bl.mu.Unlock()
		return false, nil
	}
	delete(bl.entries, uuid)
	bl.mu.Unlock()

	return true, bl.Save()
}

// IsBanned returns true if the given UUID has an active (non-expired) ban.
func (bl *BanList) IsBanned(uuid protocol.UUID) bool {
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	e, ok := bl.entries[uuid]
	if !ok {
		return false
	}
	if e.IsExpired() {
		return false
	}
	return true
}

// GetBan returns the ban entry for a UUID, or nil.
func (bl *BanList) GetBan(uuid protocol.UUID) *BanEntry {
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	if e, ok := bl.entries[uuid]; ok {
		cpy := *e
		return &cpy
	}
	return nil
}

// GetBanReason returns the ban reason for a UUID, or empty string.
func (bl *BanList) GetBanReason(uuid protocol.UUID) string {
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	if e, ok := bl.entries[uuid]; ok {
		return e.Reason
	}
	return ""
}

// Count returns the number of active bans.
func (bl *BanList) Count() int {
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	return len(bl.entries)
}

// All returns all ban entries.
func (bl *BanList) All() []BanEntry {
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	result := make([]BanEntry, 0, len(bl.entries))
	for _, e := range bl.entries {
		result = append(result, *e)
	}
	return result
}

// IPBanEntry represents a single IP ban record.
type IPBanEntry struct {
	IP      string `json:"ip"`
	Reason  string `json:"reason"`
	Source  string `json:"source"`
	Created string `json:"created"`
	Expires string `json:"expires"`
}

// IsExpired returns true if the IP ban has an expiry and it has passed.
func (b *IPBanEntry) IsExpired() bool {
	if b.Expires == "" || b.Expires == "forever" {
		return false
	}
	t, err := time.Parse(time.RFC3339, b.Expires)
	if err != nil {
		return false
	}
	return time.Now().After(t)
}

// IPBanList manages banned IPs with JSON persistence.
type IPBanList struct {
	mu       sync.RWMutex
	entries  map[string]*IPBanEntry
	filePath string
}

// NewIPBanList creates an IP ban list that persists to the given file path.
func NewIPBanList(filePath string) *IPBanList {
	return &IPBanList{
		entries:  make(map[string]*IPBanEntry),
		filePath: filePath,
	}
}

// Load reads the IP ban list from the JSON file.
func (ibl *IPBanList) Load() error {
	ibl.mu.Lock()
	defer ibl.mu.Unlock()

	data, err := os.ReadFile(ibl.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var entries []IPBanEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	ibl.entries = make(map[string]*IPBanEntry, len(entries))
	for i := range entries {
		ibl.entries[entries[i].IP] = &entries[i]
	}
	return nil
}

// Save writes the IP ban list to the JSON file.
func (ibl *IPBanList) Save() error {
	ibl.mu.RLock()
	defer ibl.mu.RUnlock()

	entries := make([]IPBanEntry, 0, len(ibl.entries))
	for _, e := range ibl.entries {
		entries = append(entries, *e)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(ibl.filePath, data, 0644)
}

// Ban adds an IP ban and saves to disk.
func (ibl *IPBanList) Ban(ip, reason, source, expires string) error {
	ibl.mu.Lock()
	ibl.entries[ip] = &IPBanEntry{
		IP:      ip,
		Reason:  reason,
		Source:  source,
		Created: time.Now().Format(time.RFC3339),
		Expires: expires,
	}
	ibl.mu.Unlock()

	return ibl.Save()
}

// Pardon removes an IP ban and saves to disk.
func (ibl *IPBanList) Pardon(ip string) (bool, error) {
	ibl.mu.Lock()
	if _, exists := ibl.entries[ip]; !exists {
		ibl.mu.Unlock()
		return false, nil
	}
	delete(ibl.entries, ip)
	ibl.mu.Unlock()

	return true, ibl.Save()
}

// IsBanned returns true if the given IP has an active (non-expired) ban.
func (ibl *IPBanList) IsBanned(ip string) bool {
	ibl.mu.RLock()
	defer ibl.mu.RUnlock()
	e, ok := ibl.entries[ip]
	if !ok {
		return false
	}
	if e.IsExpired() {
		return false
	}
	return true
}

// GetBanReason returns the ban reason for an IP, or empty string.
func (ibl *IPBanList) GetBanReason(ip string) string {
	ibl.mu.RLock()
	defer ibl.mu.RUnlock()
	if e, ok := ibl.entries[ip]; ok {
		return e.Reason
	}
	return ""
}

// Count returns the number of IP bans.
func (ibl *IPBanList) Count() int {
	ibl.mu.RLock()
	defer ibl.mu.RUnlock()
	return len(ibl.entries)
}

// All returns all IP ban entries.
func (ibl *IPBanList) All() []IPBanEntry {
	ibl.mu.RLock()
	defer ibl.mu.RUnlock()
	result := make([]IPBanEntry, 0, len(ibl.entries))
	for _, e := range ibl.entries {
		result = append(result, *e)
	}
	return result
}
