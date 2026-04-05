package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetBeaconEffect is sent when the client selects effects on a beacon.
type SetBeaconEffect struct {
	HasPrimary   bool
	PrimaryEffect int32
	HasSecondary bool
	SecondaryEffect int32
}

func NewSetBeaconEffect() protocol.Packet { return &SetBeaconEffect{} }

func (p *SetBeaconEffect) ID() int32 {
	return int32(packetid.ServerboundSetBeaconEffect)
}

func (p *SetBeaconEffect) Decode(buf *protocol.Buffer) error {
	var err error
	if p.HasPrimary, err = buf.ReadBool(); err != nil {
		return err
	}
	if p.HasPrimary {
		if p.PrimaryEffect, err = buf.ReadVarInt(); err != nil {
			return err
		}
	}
	if p.HasSecondary, err = buf.ReadBool(); err != nil {
		return err
	}
	if p.HasSecondary {
		p.SecondaryEffect, err = buf.ReadVarInt()
	}
	return err
}

func (p *SetBeaconEffect) Encode(buf *protocol.Buffer) error {
	buf.WriteBool(p.HasPrimary)
	if p.HasPrimary {
		buf.WriteVarInt(p.PrimaryEffect)
	}
	buf.WriteBool(p.HasSecondary)
	if p.HasSecondary {
		buf.WriteVarInt(p.SecondaryEffect)
	}
	return nil
}
