package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// AttachEntity attaches an entity to another (e.g. lead/leash).
type AttachEntity struct {
	AttachedEntityID int32
	HoldingEntityID  int32
}

func NewAttachEntity() protocol.Packet { return &AttachEntity{} }

func (p *AttachEntity) ID() int32 {
	return int32(packetid.ClientboundAttachEntity)
}

func (p *AttachEntity) Decode(buf *protocol.Buffer) error {
	var err error
	if p.AttachedEntityID, err = buf.ReadInt32(); err != nil {
		return err
	}
	p.HoldingEntityID, err = buf.ReadInt32()
	return err
}

func (p *AttachEntity) Encode(buf *protocol.Buffer) error {
	buf.WriteInt32(p.AttachedEntityID)
	buf.WriteInt32(p.HoldingEntityID)
	return nil
}
