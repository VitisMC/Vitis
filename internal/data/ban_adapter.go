package data

import (
	"github.com/vitismc/vitis/internal/command"
	"github.com/vitismc/vitis/internal/protocol"
)

// BanListAdapter wraps BanList to satisfy command.BanProvider.
type BanListAdapter struct {
	BL *BanList
}

// Ban adds a player ban.
func (a *BanListAdapter) Ban(uuid protocol.UUID, name, reason, source, expires string) error {
	return a.BL.Ban(uuid, name, reason, source, expires)
}

// Pardon removes a player ban by name.
func (a *BanListAdapter) Pardon(name string) (bool, error) {
	return a.BL.Pardon(name)
}

// IsBanned returns true if the UUID has an active ban.
func (a *BanListAdapter) IsBanned(uuid protocol.UUID) bool {
	return a.BL.IsBanned(uuid)
}

// GetBanReason returns the ban reason for a UUID.
func (a *BanListAdapter) GetBanReason(uuid protocol.UUID) string {
	return a.BL.GetBanReason(uuid)
}

// All returns all ban entries as BanEntryInfo.
func (a *BanListAdapter) All() []command.BanEntryInfo {
	entries := a.BL.All()
	result := make([]command.BanEntryInfo, len(entries))
	for i, e := range entries {
		result[i] = command.BanEntryInfo{Name: e.Name, Reason: e.Reason}
	}
	return result
}
