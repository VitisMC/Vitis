package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// PlayServerLink represents a single server link entry in play state.
type PlayServerLink struct {
	IsBuiltIn bool
	BuiltInID int32
	Label     string
	URL       string
}

// ClientboundServerLinks sends server links to the client during play.
type ClientboundServerLinks struct {
	Links []PlayServerLink
}

func NewClientboundServerLinks() protocol.Packet { return &ClientboundServerLinks{} }

func (p *ClientboundServerLinks) ID() int32 {
	return int32(packetid.ClientboundServerLinks)
}

func (p *ClientboundServerLinks) Decode(buf *protocol.Buffer) error {
	count, err := buf.ReadVarInt()
	if err != nil {
		return err
	}
	p.Links = make([]PlayServerLink, count)
	for i := int32(0); i < count; i++ {
		if p.Links[i].IsBuiltIn, err = buf.ReadBool(); err != nil {
			return err
		}
		if p.Links[i].IsBuiltIn {
			if p.Links[i].BuiltInID, err = buf.ReadVarInt(); err != nil {
				return err
			}
		} else {
			if p.Links[i].Label, err = buf.ReadString(); err != nil {
				return err
			}
		}
		if p.Links[i].URL, err = buf.ReadString(); err != nil {
			return err
		}
	}
	return nil
}

func (p *ClientboundServerLinks) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(int32(len(p.Links)))
	for _, l := range p.Links {
		buf.WriteBool(l.IsBuiltIn)
		if l.IsBuiltIn {
			buf.WriteVarInt(l.BuiltInID)
		} else {
			buf.WriteString(l.Label)
		}
		buf.WriteString(l.URL)
	}
	return nil
}
