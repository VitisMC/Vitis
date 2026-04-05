package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SteerBoat is sent by the client to indicate the state of each boat paddle.
type SteerBoat struct {
	LeftPaddle  bool
	RightPaddle bool
}

func NewSteerBoat() protocol.Packet { return &SteerBoat{} }

func (p *SteerBoat) ID() int32 {
	return int32(packetid.ServerboundSteerBoat)
}

func (p *SteerBoat) Decode(buf *protocol.Buffer) error {
	var err error
	if p.LeftPaddle, err = buf.ReadBool(); err != nil {
		return err
	}
	p.RightPaddle, err = buf.ReadBool()
	return err
}

func (p *SteerBoat) Encode(buf *protocol.Buffer) error {
	buf.WriteBool(p.LeftPaddle)
	buf.WriteBool(p.RightPaddle)
	return nil
}
