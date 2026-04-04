package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// PlayerInfoRemove removes players from the tab list.
type PlayerInfoRemove struct {
	UUIDs []protocol.UUID
}

func NewPlayerInfoRemove() protocol.Packet { return &PlayerInfoRemove{} }

func (p *PlayerInfoRemove) ID() int32 { return int32(packetid.ClientboundPlayerRemove) }

func (p *PlayerInfoRemove) Decode(buf *protocol.Buffer) error {
	count, err := buf.ReadVarInt()
	if err != nil {
		return err
	}
	p.UUIDs = make([]protocol.UUID, count)
	for i := int32(0); i < count; i++ {
		if p.UUIDs[i], err = buf.ReadUUID(); err != nil {
			return err
		}
	}
	return nil
}

func (p *PlayerInfoRemove) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(int32(len(p.UUIDs)))
	for _, u := range p.UUIDs {
		buf.WriteUUID(u)
	}
	return nil
}
