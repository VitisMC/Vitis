package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SelectBundleItem is sent by the client to select an item within a bundle.
type SelectBundleItem struct {
	SlotID    int32
	SelectedIndex int32
}

func NewSelectBundleItem() protocol.Packet { return &SelectBundleItem{} }

func (p *SelectBundleItem) ID() int32 {
	return int32(packetid.ServerboundSelectBundleItem)
}

func (p *SelectBundleItem) Decode(buf *protocol.Buffer) error {
	var err error
	if p.SlotID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	p.SelectedIndex, err = buf.ReadVarInt()
	return err
}

func (p *SelectBundleItem) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.SlotID)
	buf.WriteVarInt(p.SelectedIndex)
	return nil
}
