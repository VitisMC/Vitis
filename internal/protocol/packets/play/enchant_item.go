package play

import (
	"fmt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// EnchantItem selects a button in the current menu, including stonecutter recipes.
type EnchantItem struct {
	WindowID    int32
	Enchantment int32
}

// NewEnchantItem creates a container button request.
func NewEnchantItem() protocol.Packet { return &EnchantItem{} }

// ID returns the container button packet identifier.
func (p *EnchantItem) ID() int32 {
	return int32(packetid.ServerboundEnchantItem)
}

// Decode reads the protocol 769 container and button VarInts.
func (p *EnchantItem) Decode(buf *protocol.Buffer) error {
	var err error
	p.WindowID, err = buf.ReadVarInt()
	if err != nil {
		return err
	}
	p.Enchantment, err = buf.ReadVarInt()
	if err != nil {
		return err
	}
	if p.WindowID < 0 || p.Enchantment < 0 || buf.Remaining() != 0 {
		return fmt.Errorf("invalid container button request")
	}
	return nil
}

// Encode writes the protocol 769 container and button VarInts.
func (p *EnchantItem) Encode(buf *protocol.Buffer) error {
	if p.WindowID < 0 || p.Enchantment < 0 {
		return fmt.Errorf("invalid container button request")
	}
	buf.WriteVarInt(p.WindowID)
	buf.WriteVarInt(p.Enchantment)
	return nil
}
