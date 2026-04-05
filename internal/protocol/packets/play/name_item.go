package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// NameItem is sent when the client names an item in an anvil.
type NameItem struct {
	ItemName string
}

func NewNameItem() protocol.Packet { return &NameItem{} }

func (p *NameItem) ID() int32 {
	return int32(packetid.ServerboundNameItem)
}

func (p *NameItem) Decode(buf *protocol.Buffer) error {
	var err error
	p.ItemName, err = buf.ReadString()
	return err
}

func (p *NameItem) Encode(buf *protocol.Buffer) error {
	buf.WriteString(p.ItemName)
	return nil
}
