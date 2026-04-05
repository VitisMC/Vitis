package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// StepTick advances the game by a specified number of ticks (debug).
type StepTick struct {
	Steps int32
}

func NewStepTick() protocol.Packet { return &StepTick{} }

func (p *StepTick) ID() int32 {
	return int32(packetid.ClientboundStepTick)
}

func (p *StepTick) Decode(buf *protocol.Buffer) error {
	var err error
	p.Steps, err = buf.ReadVarInt()
	return err
}

func (p *StepTick) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.Steps)
	return nil
}
