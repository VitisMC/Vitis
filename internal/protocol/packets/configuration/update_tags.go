package configuration

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// TagEntry represents a single tag within a registry.
type TagEntry struct {
	Name    string
	Entries []int32
}

// RegistryTags represents all tags for one registry.
type RegistryTags struct {
	Registry string
	Tags     []TagEntry
}

// UpdateTags is sent by the server to provide tag data during configuration.
type UpdateTags struct {
	Registries []RegistryTags
}

// NewUpdateTags constructs an empty UpdateTags packet.
func NewUpdateTags() protocol.Packet {
	return &UpdateTags{}
}

// ID returns the protocol packet id.
func (p *UpdateTags) ID() int32 {
	return int32(packetid.ClientboundConfigTags)
}

// Decode reads UpdateTags fields from buffer.
func (p *UpdateTags) Decode(buf *protocol.Buffer) error {
	regCount, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode update_tags registry_count: %w", err)
	}
	p.Registries = make([]RegistryTags, regCount)
	for i := int32(0); i < regCount; i++ {
		regID, err := buf.ReadString()
		if err != nil {
			return fmt.Errorf("decode update_tags[%d] registry: %w", i, err)
		}
		tagCount, err := buf.ReadVarInt()
		if err != nil {
			return fmt.Errorf("decode update_tags[%d] tag_count: %w", i, err)
		}
		tags := make([]TagEntry, tagCount)
		for j := int32(0); j < tagCount; j++ {
			name, err := buf.ReadString()
			if err != nil {
				return fmt.Errorf("decode update_tags[%d][%d] name: %w", i, j, err)
			}
			entryCount, err := buf.ReadVarInt()
			if err != nil {
				return fmt.Errorf("decode update_tags[%d][%d] entry_count: %w", i, j, err)
			}
			entries := make([]int32, entryCount)
			for k := int32(0); k < entryCount; k++ {
				v, err := buf.ReadVarInt()
				if err != nil {
					return fmt.Errorf("decode update_tags[%d][%d][%d]: %w", i, j, k, err)
				}
				entries[k] = v
			}
			tags[j] = TagEntry{Name: name, Entries: entries}
		}
		p.Registries[i] = RegistryTags{Registry: regID, Tags: tags}
	}
	return nil
}

// Encode writes UpdateTags fields to buffer.
func (p *UpdateTags) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(int32(len(p.Registries)))
	for _, reg := range p.Registries {
		if err := buf.WriteString(reg.Registry); err != nil {
			return fmt.Errorf("encode update_tags registry: %w", err)
		}
		buf.WriteVarInt(int32(len(reg.Tags)))
		for _, tag := range reg.Tags {
			if err := buf.WriteString(tag.Name); err != nil {
				return fmt.Errorf("encode update_tags tag name: %w", err)
			}
			buf.WriteVarInt(int32(len(tag.Entries)))
			for _, entry := range tag.Entries {
				buf.WriteVarInt(entry)
			}
		}
	}
	return nil
}
