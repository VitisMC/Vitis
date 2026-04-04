package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

const (
	ActionAddPlayer         int8 = 0x01
	ActionInitializeChat    int8 = 0x02
	ActionUpdateGameMode    int8 = 0x04
	ActionUpdateListed      int8 = 0x08
	ActionUpdateLatency     int8 = 0x10
	ActionUpdateDisplayName int8 = 0x20
	ActionUpdateListOrder   int8 = 0x40
)

// PlayerInfoEntry represents one player in the PlayerInfoUpdate packet.
type PlayerInfoEntry struct {
	UUID       protocol.UUID
	Name       string
	Properties []PlayerProperty
	GameMode   int32
	Listed     bool
	Ping       int32
}

// PlayerProperty is a player profile property (e.g. textures).
type PlayerProperty struct {
	Name      string
	Value     string
	IsSigned  bool
	Signature string
}

// PlayerInfoUpdate is sent to add/update players in the tab list.
type PlayerInfoUpdate struct {
	Actions int8
	Entries []PlayerInfoEntry
}

func NewPlayerInfoUpdate() protocol.Packet { return &PlayerInfoUpdate{} }

func (p *PlayerInfoUpdate) ID() int32 { return int32(packetid.ClientboundPlayerInfo) }

func (p *PlayerInfoUpdate) Decode(buf *protocol.Buffer) error {
	return nil
}

func (p *PlayerInfoUpdate) Encode(buf *protocol.Buffer) error {
	buf.WriteByte(byte(p.Actions))
	buf.WriteVarInt(int32(len(p.Entries)))

	for _, e := range p.Entries {
		buf.WriteUUID(e.UUID)

		if p.Actions&ActionAddPlayer != 0 {
			buf.WriteString(e.Name)
			buf.WriteVarInt(int32(len(e.Properties)))
			for _, prop := range e.Properties {
				buf.WriteString(prop.Name)
				buf.WriteString(prop.Value)
				if prop.IsSigned {
					buf.WriteByte(1)
					buf.WriteString(prop.Signature)
				} else {
					buf.WriteByte(0)
				}
			}
		}
		if p.Actions&ActionInitializeChat != 0 {
			buf.WriteByte(0)
		}
		if p.Actions&ActionUpdateGameMode != 0 {
			buf.WriteVarInt(e.GameMode)
		}
		if p.Actions&ActionUpdateListed != 0 {
			if e.Listed {
				buf.WriteByte(1)
			} else {
				buf.WriteByte(0)
			}
		}
		if p.Actions&ActionUpdateLatency != 0 {
			buf.WriteVarInt(e.Ping)
		}
		if p.Actions&ActionUpdateDisplayName != 0 {
			buf.WriteByte(0)
		}
		if p.Actions&ActionUpdateListOrder != 0 {
			buf.WriteVarInt(0)
		}
	}
	return nil
}
