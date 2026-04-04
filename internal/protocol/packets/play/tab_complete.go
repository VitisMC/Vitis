package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

type ServerboundTabComplete struct {
	TransactionID int32
	Text          string
}

func NewServerboundTabComplete() protocol.Packet { return &ServerboundTabComplete{} }

func (p *ServerboundTabComplete) ID() int32 {
	return int32(packetid.ServerboundTabComplete)
}

func (p *ServerboundTabComplete) Decode(buf *protocol.Buffer) error {
	var err error
	if p.TransactionID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	p.Text, err = buf.ReadString()
	return err
}

func (p *ServerboundTabComplete) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.TransactionID)
	buf.WriteString(p.Text)
	return nil
}

type TabCompleteMatch struct {
	Match   string
	Tooltip string
}

type ClientboundTabComplete struct {
	TransactionID int32
	Start         int32
	Length        int32
	Matches       []TabCompleteMatch
}

func NewClientboundTabComplete() protocol.Packet { return &ClientboundTabComplete{} }

func (p *ClientboundTabComplete) ID() int32 {
	return int32(packetid.ClientboundTabComplete)
}

func (p *ClientboundTabComplete) Decode(_ *protocol.Buffer) error { return nil }

func (p *ClientboundTabComplete) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.TransactionID)
	buf.WriteVarInt(p.Start)
	buf.WriteVarInt(p.Length)
	buf.WriteVarInt(int32(len(p.Matches)))
	for _, m := range p.Matches {
		buf.WriteString(m.Match)
		if m.Tooltip != "" {
			buf.WriteByte(1)
			writeNBTTextComponent(buf, m.Tooltip)
		} else {
			buf.WriteByte(0)
		}
	}
	return nil
}
