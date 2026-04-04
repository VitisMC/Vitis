package configuration

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// RegistryEntry represents a single entry in a registry data packet.
type RegistryEntry struct {
	EntryID string
	HasData bool
	Data    []byte
}

// RegistryData is sent by the server to provide registry data to the client during configuration.
type RegistryData struct {
	RegistryID string
	Entries    []RegistryEntry
}

// NewRegistryData constructs an empty RegistryData packet.
func NewRegistryData() protocol.Packet {
	return &RegistryData{}
}

// ID returns the protocol packet id.
func (p *RegistryData) ID() int32 {
	return int32(packetid.ClientboundConfigRegistryData)
}

// Decode reads RegistryData fields from buffer.
func (p *RegistryData) Decode(buf *protocol.Buffer) error {
	regID, err := buf.ReadString()
	if err != nil {
		return fmt.Errorf("decode registry_data registry_id: %w", err)
	}
	p.RegistryID = regID

	count, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode registry_data entry count: %w", err)
	}
	p.Entries = make([]RegistryEntry, count)
	for i := int32(0); i < count; i++ {
		entryID, err := buf.ReadString()
		if err != nil {
			return fmt.Errorf("decode registry_data[%d] entry_id: %w", i, err)
		}
		hasData, err := buf.ReadBool()
		if err != nil {
			return fmt.Errorf("decode registry_data[%d] has_data: %w", i, err)
		}
		entry := RegistryEntry{EntryID: entryID, HasData: hasData}
		if hasData {
			remaining := buf.Remaining()
			data, err := buf.ReadBytes(remaining)
			if err != nil {
				return fmt.Errorf("decode registry_data[%d] data: %w", i, err)
			}
			entry.Data = data
		}
		p.Entries[i] = entry
	}
	return nil
}

// Encode writes RegistryData fields to buffer.
func (p *RegistryData) Encode(buf *protocol.Buffer) error {
	if err := buf.WriteString(p.RegistryID); err != nil {
		return fmt.Errorf("encode registry_data registry_id: %w", err)
	}
	buf.WriteVarInt(int32(len(p.Entries)))
	for i, entry := range p.Entries {
		if err := buf.WriteString(entry.EntryID); err != nil {
			return fmt.Errorf("encode registry_data[%d] entry_id: %w", i, err)
		}
		buf.WriteBool(entry.HasData)
		if entry.HasData {
			buf.WriteBytes(entry.Data)
		}
	}
	return nil
}
