package registry

import (
	"sort"

	gentag "github.com/vitismc/vitis/internal/data/generated/tag"
	cfgpacket "github.com/vitismc/vitis/internal/protocol/packets/configuration"
)

// BuildUpdateTags constructs an UpdateTags packet from pre-computed generated tag data.
func (m *Manager) BuildUpdateTags() *cfgpacket.UpdateTags {
	tagData := gentag.Data()

	registries := make([]cfgpacket.RegistryTags, 0, len(tagData))
	regNames := make([]string, 0, len(tagData))
	for rn := range tagData {
		regNames = append(regNames, rn)
	}
	sort.Strings(regNames)

	for _, regName := range regNames {
		tags := tagData[regName]
		tagNames := make([]string, 0, len(tags))
		for tn := range tags {
			tagNames = append(tagNames, tn)
		}
		sort.Strings(tagNames)

		entries := make([]cfgpacket.TagEntry, len(tagNames))
		for i, tn := range tagNames {
			entries[i] = cfgpacket.TagEntry{Name: tn, Entries: tags[tn]}
		}
		registries = append(registries, cfgpacket.RegistryTags{
			Registry: regName,
			Tags:     entries,
		})
	}

	return &cfgpacket.UpdateTags{Registries: registries}
}
