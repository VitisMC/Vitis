package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetTickingState tells the client the server's tick rate and whether it's frozen.
type SetTickingState struct {
	TickRate float32
	IsFrozen bool
}

func NewSetTickingState() protocol.Packet { return &SetTickingState{} }

func (p *SetTickingState) ID() int32 { return int32(packetid.ClientboundSetTickingState) }

func (p *SetTickingState) Decode(buf *protocol.Buffer) error {
	var err error
	if p.TickRate, err = buf.ReadFloat32(); err != nil {
		return err
	}
	b, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.IsFrozen = b != 0
	return nil
}

func (p *SetTickingState) Encode(buf *protocol.Buffer) error {
	buf.WriteFloat32(p.TickRate)
	if p.IsFrozen {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	return nil
}
