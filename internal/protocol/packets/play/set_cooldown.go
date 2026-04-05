package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetCooldown displays an item cooldown on the client.
type SetCooldown struct {
	ItemID        int32
	CooldownTicks int32
}

func NewSetCooldown() protocol.Packet { return &SetCooldown{} }

func (p *SetCooldown) ID() int32 {
	return int32(packetid.ClientboundSetCooldown)
}

func (p *SetCooldown) Decode(buf *protocol.Buffer) error {
	var err error
	if p.ItemID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	p.CooldownTicks, err = buf.ReadVarInt()
	return err
}

func (p *SetCooldown) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.ItemID)
	buf.WriteVarInt(p.CooldownTicks)
	return nil
}
