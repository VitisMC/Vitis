package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// AdvancementTab is sent when the client switches tabs in the advancement screen.
type AdvancementTab struct {
	Action int32
	TabID  string
}

func NewAdvancementTab() protocol.Packet { return &AdvancementTab{} }

func (p *AdvancementTab) ID() int32 {
	return int32(packetid.ServerboundAdvancementTab)
}

func (p *AdvancementTab) Decode(buf *protocol.Buffer) error {
	var err error
	if p.Action, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.Action == 0 {
		p.TabID, err = buf.ReadString()
	}
	return err
}

func (p *AdvancementTab) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.Action)
	if p.Action == 0 {
		buf.WriteString(p.TabID)
	}
	return nil
}
