package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// EnchantItem is sent when the client selects an enchantment from the enchantment table.
type EnchantItem struct {
	WindowID    int32
	Enchantment int32
}

func NewEnchantItem() protocol.Packet { return &EnchantItem{} }

func (p *EnchantItem) ID() int32 {
	return int32(packetid.ServerboundEnchantItem)
}

func (p *EnchantItem) Decode(buf *protocol.Buffer) error {
	var err error
	b, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.WindowID = int32(b)
	b2, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.Enchantment = int32(b2)
	return nil
}

func (p *EnchantItem) Encode(buf *protocol.Buffer) error {
	buf.WriteByte(byte(p.WindowID))
	buf.WriteByte(byte(p.Enchantment))
	return nil
}
