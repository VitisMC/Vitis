package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetBorderLerpSize interpolates the world border size over time.
type SetBorderLerpSize struct {
	OldDiameter float64
	NewDiameter float64
	Speed       int64
}

func NewSetBorderLerpSize() protocol.Packet { return &SetBorderLerpSize{} }

func (p *SetBorderLerpSize) ID() int32 {
	return int32(packetid.ClientboundWorldBorderLerpSize)
}

func (p *SetBorderLerpSize) Decode(buf *protocol.Buffer) error {
	var err error
	if p.OldDiameter, err = buf.ReadFloat64(); err != nil {
		return err
	}
	if p.NewDiameter, err = buf.ReadFloat64(); err != nil {
		return err
	}
	p.Speed, err = buf.ReadInt64()
	return err
}

func (p *SetBorderLerpSize) Encode(buf *protocol.Buffer) error {
	buf.WriteFloat64(p.OldDiameter)
	buf.WriteFloat64(p.NewDiameter)
	buf.WriteInt64(p.Speed)
	return nil
}
