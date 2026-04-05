package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// StopSound tells the client to stop playing a specific sound.
type StopSound struct {
	Flags    byte
	Source   int32
	SoundID string
}

func NewStopSound() protocol.Packet { return &StopSound{} }

func (p *StopSound) ID() int32 {
	return int32(packetid.ClientboundStopSound)
}

func (p *StopSound) Decode(buf *protocol.Buffer) error {
	var err error
	if p.Flags, err = buf.ReadByte(); err != nil {
		return err
	}
	if p.Flags&0x01 != 0 {
		if p.Source, err = buf.ReadVarInt(); err != nil {
			return err
		}
	}
	if p.Flags&0x02 != 0 {
		p.SoundID, err = buf.ReadString()
	}
	return err
}

func (p *StopSound) Encode(buf *protocol.Buffer) error {
	buf.WriteByte(p.Flags)
	if p.Flags&0x01 != 0 {
		buf.WriteVarInt(p.Source)
	}
	if p.Flags&0x02 != 0 {
		buf.WriteString(p.SoundID)
	}
	return nil
}
