package registry

import (
	cfgpacket "github.com/vitismc/vitis/internal/protocol/packets/configuration"
)

// BuildRegistryDataPackets returns RegistryData packets for all configuration
// registries. When clientKnowsVanilla is true, entries are sent without NBT
// data (HasData=false) so the client uses its built-in vanilla data — this
// matches vanilla server behaviour after a successful Known Packs handshake.
// When false, full pre-encoded NBT is included in every entry.
func (m *Manager) BuildRegistryDataPackets(clientKnowsVanilla bool) []*cfgpacket.RegistryData {
	configNames := m.ConfigRegistryNames()
	packets := make([]*cfgpacket.RegistryData, 0, len(configNames))

	for _, regName := range configNames {
		entries := m.configData[regName]
		if len(entries) == 0 {
			continue
		}

		pktEntries := make([]cfgpacket.RegistryEntry, len(entries))
		for i, e := range entries {
			if clientKnowsVanilla {
				pktEntries[i] = cfgpacket.RegistryEntry{
					EntryID: e.Name,
					HasData: false,
				}
			} else {
				pktEntries[i] = cfgpacket.RegistryEntry{
					EntryID: e.Name,
					HasData: true,
					Data:    e.Data,
				}
			}
		}

		packets = append(packets, &cfgpacket.RegistryData{
			RegistryID: regName,
			Entries:    pktEntries,
		})
	}

	return packets
}
