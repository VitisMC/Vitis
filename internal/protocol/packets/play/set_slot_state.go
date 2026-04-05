package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetSlotState is sent by the client to toggle a slot lock state (bundle, crafter).
type SetSlotState struct {
	SlotID   int32
	WindowID int32
	State    bool
}

func NewSetSlotState() protocol.Packet { return &SetSlotState{} }

func (p *SetSlotState) ID() int32 {
	return int32(packetid.ServerboundSetSlotState)
}

func (p *SetSlotState) Decode(buf *protocol.Buffer) error {
	var err error
	if p.SlotID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.WindowID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	p.State, err = buf.ReadBool()
	return err
}

func (p *SetSlotState) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.SlotID)
	buf.WriteVarInt(p.WindowID)
	buf.WriteBool(p.State)
	return nil
}
