package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

const (
	maxBookPages      = 100
	maxBookPageChars  = 1024
	maxBookTitleChars = 32
)

// EditBook is sent when the client edits or signs a book.
type EditBook struct {
	Slot     int32
	Entries  []string
	Title    string
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
	if count < 0 || count > maxBookPages {
		return fmt.Errorf("decode edit_book: invalid page count %d", count)
	}
	p.Entries = make([]string, int(count))
	for i := int32(0); i < count; i++ {
		if p.Entries[i], err = buf.ReadStringLimit(maxBookPageChars); err != nil {
			return err
		}
	}
	if p.HasTitle, err = buf.ReadBool(); err != nil {
		return err
	}
	if p.HasTitle {
		p.Title, err = buf.ReadStringLimit(maxBookTitleChars)
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
