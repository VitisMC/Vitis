package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

type WorldEvent struct {
	Event                 int32
	X, Y, Z               int32
	Data                  int32
	DisableRelativeVolume bool
}

func NewWorldEvent() protocol.Packet { return &WorldEvent{} }

func (p *WorldEvent) ID() int32 { return int32(packetid.ClientboundWorldEvent) }

func (p *WorldEvent) Decode(_ *protocol.Buffer) error { return nil }

func (p *WorldEvent) Encode(buf *protocol.Buffer) error {
	buf.WriteInt32(p.Event)
	pos := int64(((uint64(p.X) & 0x3FFFFFF) << 38) | ((uint64(p.Z) & 0x3FFFFFF) << 12) | (uint64(p.Y) & 0xFFF))
	buf.WriteInt64(pos)
	buf.WriteInt32(p.Data)
	buf.WriteBool(p.DisableRelativeVolume)
	return nil
}
