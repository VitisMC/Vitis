package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

type EntityStatus struct {
	EntityID int32
	Status   byte
}

func NewEntityStatus() protocol.Packet { return &EntityStatus{} }

func (p *EntityStatus) ID() int32 { return int32(packetid.ClientboundEntityStatus) }

func (p *EntityStatus) Decode(_ *protocol.Buffer) error { return nil }

func (p *EntityStatus) Encode(buf *protocol.Buffer) error {
	buf.WriteInt32(p.EntityID)
	buf.WriteByte(p.Status)
	return nil
}
