package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SimulationDistance sets the simulation distance for the client.
type SimulationDistance struct {
	Distance int32
}

func NewSimulationDistance() protocol.Packet { return &SimulationDistance{} }

func (p *SimulationDistance) ID() int32 {
	return int32(packetid.ClientboundSimulationDistance)
}

func (p *SimulationDistance) Decode(buf *protocol.Buffer) error {
	var err error
	p.Distance, err = buf.ReadVarInt()
	return err
}

func (p *SimulationDistance) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.Distance)
	return nil
}
