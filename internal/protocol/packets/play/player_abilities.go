package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

const (
	AbilityInvulnerable byte = 0x01
	AbilityFlying       byte = 0x02
	AbilityAllowFlying  byte = 0x04
	AbilityInstantBreak byte = 0x08
)

// PlayerAbilitiesClientbound sends player ability flags and speeds.
type PlayerAbilitiesClientbound struct {
	Flags       byte
	FlySpeed    float32
	FOVModifier float32
}

func NewPlayerAbilitiesClientbound() protocol.Packet { return &PlayerAbilitiesClientbound{} }

func (p *PlayerAbilitiesClientbound) ID() int32 { return int32(packetid.ClientboundAbilities) }

func (p *PlayerAbilitiesClientbound) Decode(buf *protocol.Buffer) error {
	var err error
	b, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.Flags = b
	if p.FlySpeed, err = buf.ReadFloat32(); err != nil {
		return err
	}
	p.FOVModifier, err = buf.ReadFloat32()
	return err
}

func (p *PlayerAbilitiesClientbound) Encode(buf *protocol.Buffer) error {
	buf.WriteByte(p.Flags)
	buf.WriteFloat32(p.FlySpeed)
	buf.WriteFloat32(p.FOVModifier)
	return nil
}

// PlayerAbilitiesServerbound is sent by the client when toggling flying.
type PlayerAbilitiesServerbound struct {
	Flags byte
}

func NewPlayerAbilitiesServerbound() protocol.Packet { return &PlayerAbilitiesServerbound{} }

func (p *PlayerAbilitiesServerbound) ID() int32 { return int32(packetid.ServerboundAbilities) }

func (p *PlayerAbilitiesServerbound) Decode(buf *protocol.Buffer) error {
	b, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.Flags = b
	return nil
}

func (p *PlayerAbilitiesServerbound) Encode(buf *protocol.Buffer) error {
	buf.WriteByte(p.Flags)
	return nil
}

// CreativeAbilities returns abilities flags for creative mode.
func CreativeAbilities() *PlayerAbilitiesClientbound {
	return &PlayerAbilitiesClientbound{
		Flags:       AbilityInvulnerable | AbilityAllowFlying | AbilityInstantBreak,
		FlySpeed:    0.05,
		FOVModifier: 0.1,
	}
}

// SurvivalAbilities returns abilities flags for survival mode.
func SurvivalAbilities() *PlayerAbilitiesClientbound {
	return &PlayerAbilitiesClientbound{
		Flags:       0,
		FlySpeed:    0.05,
		FOVModifier: 0.1,
	}
}
