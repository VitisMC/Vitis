package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SelectAdvancementTab tells the client to open a specific advancement tab.
type SelectAdvancementTab struct {
	HasTabID bool
	TabID    string
}

func NewSelectAdvancementTab() protocol.Packet { return &SelectAdvancementTab{} }

func (p *SelectAdvancementTab) ID() int32 {
	return int32(packetid.ClientboundSelectAdvancementTab)
}

func (p *SelectAdvancementTab) Decode(buf *protocol.Buffer) error {
	var err error
	if p.HasTabID, err = buf.ReadBool(); err != nil {
		return err
	}
	if p.HasTabID {
		p.TabID, err = buf.ReadString()
	}
	return err
}

func (p *SelectAdvancementTab) Encode(buf *protocol.Buffer) error {
	buf.WriteBool(p.HasTabID)
	if p.HasTabID {
		buf.WriteString(p.TabID)
	}
	return nil
}
