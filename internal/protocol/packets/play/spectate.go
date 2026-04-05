package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// Spectate is sent when a spectator teleports to a target entity.
type Spectate struct {
	TargetUUID protocol.UUID
}

func NewSpectate() protocol.Packet { return &Spectate{} }

func (p *Spectate) ID() int32 {
	return int32(packetid.ServerboundSpectate)
}

func (p *Spectate) Decode(buf *protocol.Buffer) error {
	var err error
	p.TargetUUID, err = buf.ReadUUID()
	return err
}

func (p *Spectate) Encode(buf *protocol.Buffer) error {
	buf.WriteUUID(p.TargetUUID)
	return nil
}
