package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// EditBook is sent when the client edits or signs a book.
type EditBook struct {
	Slot    int32
	Entries []string
	Title   string
	HasTitle bool
}

func NewEditBook() protocol.Packet { return &EditBook{} }

func (p *EditBook) ID() int32 {
	return int32(packetid.ServerboundEditBook)
}

func (p *EditBook) Decode(buf *protocol.Buffer) error {
	var err error
	if p.Slot, err = buf.ReadVarInt(); err != nil {
		return err
	}
	count, err := buf.ReadVarInt()
	if err != nil {
		return err
	}
	p.Entries = make([]string, count)
	for i := int32(0); i < count; i++ {
		if p.Entries[i], err = buf.ReadString(); err != nil {
			return err
		}
	}
	if p.HasTitle, err = buf.ReadBool(); err != nil {
		return err
	}
	if p.HasTitle {
		p.Title, err = buf.ReadString()
	}
	return err
}

func (p *EditBook) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.Slot)
	buf.WriteVarInt(int32(len(p.Entries)))
	for _, e := range p.Entries {
		buf.WriteString(e)
	}
	buf.WriteBool(p.HasTitle)
	if p.HasTitle {
		buf.WriteString(p.Title)
	}
	return nil
}
