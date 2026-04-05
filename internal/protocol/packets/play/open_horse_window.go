package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// OpenHorseWindow tells the client to open the horse inventory window.
type OpenHorseWindow struct {
	WindowID  byte
	SlotCount int32
	EntityID  int32
}

func NewOpenHorseWindow() protocol.Packet { return &OpenHorseWindow{} }

func (p *OpenHorseWindow) ID() int32 {
	return int32(packetid.ClientboundOpenHorseWindow)
}

func (p *OpenHorseWindow) Decode(buf *protocol.Buffer) error {
	var err error
	if p.WindowID, err = buf.ReadByte(); err != nil {
		return err
	}
	if p.SlotCount, err = buf.ReadVarInt(); err != nil {
		return err
	}
	p.EntityID, err = buf.ReadInt32()
	return err
}

func (p *OpenHorseWindow) Encode(buf *protocol.Buffer) error {
	buf.WriteByte(p.WindowID)
	buf.WriteVarInt(p.SlotCount)
	buf.WriteInt32(p.EntityID)
	return nil
}
