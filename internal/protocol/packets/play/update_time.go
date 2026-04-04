package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// UpdateTime sends the world age and time of day to the client.
// In 1.21.4 the packet has a trailing boolean: whether day time should tick.
type UpdateTime struct {
	WorldAge    int64
	TimeOfDay   int64
	TickDayTime bool
}

func NewUpdateTime() protocol.Packet { return &UpdateTime{} }

func (p *UpdateTime) ID() int32 { return int32(packetid.ClientboundUpdateTime) }

func (p *UpdateTime) Decode(buf *protocol.Buffer) error {
	var err error
	if p.WorldAge, err = buf.ReadInt64(); err != nil {
		return err
	}
	if p.TimeOfDay, err = buf.ReadInt64(); err != nil {
		return err
	}
	b, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.TickDayTime = b != 0
	return nil
}

func (p *UpdateTime) Encode(buf *protocol.Buffer) error {
	buf.WriteInt64(p.WorldAge)
	buf.WriteInt64(p.TimeOfDay)
	if p.TickDayTime {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	return nil
}
